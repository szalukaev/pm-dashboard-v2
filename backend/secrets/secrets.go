// Package secrets encrypts secrets of third-party services that have to be
// stored — personal API keys of the data source.
//
// They are kept in PostgreSQL encrypted with AES-256-GCM. The master key is
// generated on the first start and lives in the SQLite config (/data), apart
// from the database: a database dump alone does not reveal the secrets.
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

// Store is the part of the SQLite config store the box needs.
type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// Box encrypts and decrypts with the master key.
type Box struct {
	aead cipher.AEAD
}

// Open loads the master key from the store, generating it on the first use.
func Open(store Store) (*Box, error) {
	encoded, _ := store.Get(masterKeyName)
	key, err := hex.DecodeString(encoded)
	if err != nil || len(key) != keySize {
		if encoded != "" {
			// Never replace an existing key silently: everything encrypted
			// with it would become unreadable.
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

// Encrypt returns the secret encrypted, as text safe for a database column.
// Every call gives a different result for the same secret.
func (b *Box) Encrypt(plain string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt returns the secret; it fails if the text was not produced by
// Encrypt with the same master key or was changed since.
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
