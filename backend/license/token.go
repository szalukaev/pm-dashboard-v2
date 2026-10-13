// Package license проверяет офлайн-лицензию установки: подпись токена
// лицензии, его привязку к железу хоста и вытекающее из этого состояние —
// активна, льготный период или только чтение.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

// vendorPublicKeys — ключи, которыми может быть подписана лицензия. Они
// намеренно зашиты в бинарник: ключ из конфигурации администратор
// установки мог бы заменить своим. Это список, чтобы ключ можно было
// сменить, не аннулируя выпущенные лицензии.
var vendorPublicKeys = []string{
	"988abc868c27381c1785ddbc3925486afc662fc434f30dc355b8fb38bd3410f3",
}

// Payload — подписанное содержимое токена лицензии.
type Payload struct {
	Client string `json:"client"`
	// HWIDHash — аппаратный отпечаток в том виде, в каком его подписывает
	// утилита поставщика: строка с экрана активации (см. HWID.String).
	HWIDHash string `json:"hwid_hash"`
	// HWIDComponents — тот же отпечаток по компонентам.
	HWIDComponents *struct {
		MachineIDHash   string `json:"machine_id_hash"`
		BoardSerialHash string `json:"board_serial_hash"`
	} `json:"hwid_components"`
	Edition         string   `json:"edition"`
	Features        []string `json:"features"`
	GracePeriodDays int      `json:"grace_period_days"`
	IssuedAt        string   `json:"issued_at"`
}

// DefaultGraceDays действует, если в лицензии не указано иное.
const DefaultGraceDays = 14

// GraceDays возвращает длину льготного периода, которую даёт лицензия.
func (p *Payload) GraceDays() int {
	if p == nil || p.GracePeriodDays <= 0 {
		return DefaultGraceDays
	}
	return p.GracePeriodDays
}

// HWID возвращает железо, для которого выпущена лицензия.
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

// Verify проверяет подпись токена — base64url(содержимое) "."
// base64url(подпись) — и возвращает его содержимое. Подпись проверяется по
// тем самым байтам, которые были подписаны; JSON заново не кодируется.
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
