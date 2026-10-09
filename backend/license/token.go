// Package license checks the offline license of the installation: the
// signature of the license token, its binding to the hardware of the host,
// and the state that follows — active, grace period or read-only.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

// vendorPublicKeys are the keys a license may be signed with. They are
// constants of the binary on purpose: read from a config, a key could be
// replaced by the administrator of the installation with their own.
// A list, so that a key can be rotated without voiding issued licenses.
var vendorPublicKeys = []string{
	"988abc868c27381c1785ddbc3925486afc662fc434f30dc355b8fb38bd3410f3",
}

// Payload is the signed content of a license token.
type Payload struct {
	Client string `json:"client"`
	// HWIDHash is the hardware fingerprint as the vendor tool signs it: the
	// string shown on the activation screen (see HWID.String).
	HWIDHash string `json:"hwid_hash"`
	// HWIDComponents is the same fingerprint given by components.
	HWIDComponents *struct {
		MachineIDHash   string `json:"machine_id_hash"`
		BoardSerialHash string `json:"board_serial_hash"`
	} `json:"hwid_components"`
	Edition         string   `json:"edition"`
	Features        []string `json:"features"`
	GracePeriodDays int      `json:"grace_period_days"`
	IssuedAt        string   `json:"issued_at"`
}

// DefaultGraceDays is used when a license does not say otherwise.
const DefaultGraceDays = 14

// GraceDays returns the length of the grace period the license gives.
func (p *Payload) GraceDays() int {
	if p == nil || p.GracePeriodDays <= 0 {
		return DefaultGraceDays
	}
	return p.GracePeriodDays
}

// HWID returns the hardware the license was issued for.
func (p *Payload) HWID() HWID {
	if p.HWIDComponents != nil {
		return HWID{MachineIDHash: p.HWIDComponents.MachineIDHash, BoardSerialHash: p.HWIDComponents.BoardSerialHash}
	}
	return ParseHWID(p.HWIDHash)
}

var (
	ErrMalformed = errors.New("malformed license token")
	ErrSignature = errors.New("license signature is not valid")
)

var tokenEncoding = base64.RawURLEncoding

// Verify checks the signature of a token — base64url(payload) "."
// base64url(signature) — and returns its payload. The signature is checked
// over the very bytes that were signed; the JSON is never re-encoded.
func Verify(token string) (*Payload, error) {
	return verifyWith(token, vendorPublicKeys)
}

func verifyWith(token string, publicKeys []string) (*Payload, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return nil, ErrMalformed
	}
	body, err := tokenEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrMalformed
	}
	signature, err := tokenEncoding.DecodeString(parts[1])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return nil, ErrMalformed
	}

	signed := false
	for _, keyHex := range publicKeys {
		key, err := hex.DecodeString(keyHex)
		if err == nil && len(key) == ed25519.PublicKeySize && ed25519.Verify(key, body, signature) {
			signed = true
			break
		}
	}
	if !signed {
		return nil, ErrSignature
	}

	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, ErrMalformed
	}
	return &payload, nil
}
