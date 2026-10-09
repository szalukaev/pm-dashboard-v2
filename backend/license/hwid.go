package license

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

// DefaultHWIDPath is where the host's hardware identifiers are mounted into
// the container (read-only). The file is written on the host by
// scripts/collect-hwid.sh: the application never asks the hardware itself,
// which would need privileges a container should not have.
const DefaultHWIDPath = "/etc/pmdashboard/hwid-source"

// HWID is the hardware fingerprint of the host: two components, each a
// SHA-256 of an identifier. A component is "" when the host does not have a
// usable value for it.
type HWID struct {
	MachineIDHash   string // of /etc/machine-id
	BoardSerialHash string // of the baseboard serial number
}

const (
	hwidSeparator = ":"
	hwidMissing   = "-"
)

// String is the fingerprint as one string — what the activation screen
// shows and the vendor signs: "<machine id hash>:<board serial hash>",
// a missing component is "-".
func (h HWID) String() string {
	part := func(v string) string {
		if v == "" {
			return hwidMissing
		}
		return v
	}
	return part(h.MachineIDHash) + hwidSeparator + part(h.BoardSerialHash)
}

// Empty reports whether no component is known.
func (h HWID) Empty() bool {
	return h.MachineIDHash == "" && h.BoardSerialHash == ""
}

// ParseHWID reads the String form back.
func ParseHWID(s string) HWID {
	parts := strings.SplitN(strings.TrimSpace(s), hwidSeparator, 2)
	value := func(i int) string {
		if i >= len(parts) || parts[i] == hwidMissing {
			return ""
		}
		return strings.ToLower(strings.TrimSpace(parts[i]))
	}
	return HWID{MachineIDHash: value(0), BoardSerialHash: value(1)}
}

// Matches reports whether this hardware is the one a license was issued
// for: at least one of the two components is the same. Reinstalling the OS
// changes the machine id, replacing the board changes its serial — neither
// alone makes it another machine.
func (h HWID) Matches(licensed HWID) bool {
	same := func(a, b string) bool { return a != "" && a == b }
	return same(h.MachineIDHash, licensed.MachineIDHash) || same(h.BoardSerialHash, licensed.BoardSerialHash)
}

// ReadHWID reads the identifiers of the host from the file written by
// scripts/collect-hwid.sh ("machine_id=…" and "board_serial=…" lines) and
// hashes them.
func ReadHWID(path string) (HWID, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return HWID{}, err
	}
	return parseHWIDSource(string(data)), nil
}

func parseHWIDSource(content string) HWID {
	var hwid HWID
	for _, line := range strings.Split(content, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "machine_id":
			hwid.MachineIDHash = hashIdentifier(value)
		case "board_serial":
			if !isPlaceholderSerial(value) {
				hwid.BoardSerialHash = hashIdentifier(value)
			}
		}
	}
	return hwid
}

// isPlaceholderSerial detects what boards report instead of a real serial
// number. The collecting script filters these too; the check is repeated
// here so a stub can never become a license binding shared by many hosts.
func isPlaceholderSerial(serial string) bool {
	s := strings.ToLower(strings.TrimSpace(serial))
	if s == "" || strings.Trim(s, "0") == "" || strings.Trim(s, "x") == "" {
		return true
	}
	for _, stub := range []string{"to be filled", "o.e.m", "oem", "not specified", "not applicable", "default string", "none", "unknown", "n/a", "system serial"} {
		if strings.Contains(s, stub) {
			return true
		}
	}
	return false
}

func hashIdentifier(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
