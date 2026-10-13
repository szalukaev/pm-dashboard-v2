// Package secrets шифрует секреты сторонних сервисов, которые приходится
// хранить, — личные ключи API источника данных.
//
// Они лежат в PostgreSQL зашифрованными AES-256-GCM. Мастер-ключ создаётся
// при первом запуске и хранится в конфигурации SQLite (/data), отдельно от
// базы: один только дамп базы секретов не раскрывает.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	masterKeyName = "secrets_master_key"
	keySize       = 32
)

// Store — та часть хранилища конфигурации SQLite, которая нужна Box.
type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// Box шифрует и расшифровывает мастер-ключом.
type Box struct {
	aead cipher.AEAD
}

// Open загружает мастер-ключ из хранилища, а при первом обращении создаёт его.
func Open(store Store) (*Box, error) {
	encoded, _ := store.Get(masterKeyName)
	key, err := hex.DecodeString(encoded)
	if err != nil || len(key) != keySize {
		if encoded != "" {
			// Существующий ключ молча не заменяется: всё зашифрованное им
			// перестало бы читаться.
			return nil, errors.New("master key in the config store is damaged")
		}
		key = make([]byte, keySize)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate master key: %w", err)
		}
		if err := store.Set(masterKeyName, hex.EncodeToString(key)); err != nil {
			return nil, fmt.Errorf("save master key: %w", err)
		}
	}
	return newBox(key)
}

func newBox(key []byte) (*Box, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Encrypt возвращает зашифрованный секрет текстом, пригодным для столбца
// базы. Для одного и того же секрета каждый вызов даёт другой результат.
func (b *Box) Encrypt(plain string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt возвращает секрет; ошибка — если текст получен не от Encrypt с тем
// же мастер-ключом или с тех пор изменён.
func (b *Box) Decrypt(encrypted string) (string, error) {
	sealed, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	size := b.aead.NonceSize()
	if len(sealed) < size {
		return "", errors.New("encrypted value is too short")
	}
	plain, err := b.aead.Open(nil, sealed[:size], sealed[size:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
