package license

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

// DefaultHWIDPath — куда в контейнер монтируются аппаратные идентификаторы
// хоста (только для чтения). Файл пишет scripts/collect-hwid.sh, который
// docker-compose запускает до старта backend: само приложение железо не
// опрашивает — для этого понадобились бы привилегии, которых у его
// контейнера быть не должно.
const DefaultHWIDPath = "/etc/pmdashboard/hwid-source"

// HWID — аппаратный отпечаток хоста: два компонента, каждый — SHA-256
// идентификатора. Компонент равен "", если у хоста нет для него пригодного
// значения.
type HWID struct {
	MachineIDHash   string // от /etc/machine-id
	BoardSerialHash string // от серийного номера материнской платы
}

const (
	hwidSeparator = ":"
	hwidMissing   = "-"
)

// String — отпечаток одной строкой: то, что показывает экран активации и
// подписывает поставщик: "<хэш machine id>:<хэш серийного номера платы>";
// отсутствующий компонент — "-".
func (h HWID) String() string {
	part := func(v string) string {
		if v == "" {
			return hwidMissing
		}
		return v
	}
	return part(h.MachineIDHash) + hwidSeparator + part(h.BoardSerialHash)
}

// Empty сообщает, что не известен ни один компонент.
func (h HWID) Empty() bool {
	return h.MachineIDHash == "" && h.BoardSerialHash == ""
}

// ParseHWID разбирает строку, которую даёт String.
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

// Matches сообщает, то ли это железо, для которого выпущена лицензия:
// совпадает хотя бы один из двух компонентов. Переустановка ОС меняет
// machine id, замена платы — её серийный номер; ни то ни другое по
// отдельности не делает машину другой.
func (h HWID) Matches(licensed HWID) bool {
	same := func(a, b string) bool { return a != "" && a == b }
	return same(h.MachineIDHash, licensed.MachineIDHash) || same(h.BoardSerialHash, licensed.BoardSerialHash)
}

// ReadHWID читает идентификаторы хоста из файла, записанного
// scripts/collect-hwid.sh (строки "machine_id=…" и "board_serial=…"), и
// хэширует их.
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

// isPlaceholderSerial распознаёт то, что платы сообщают вместо настоящего
// серийного номера. Скрипт сбора тоже их отсеивает; проверка повторена
// здесь, чтобы заглушка не стала привязкой лицензии, общей для многих хостов.
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
