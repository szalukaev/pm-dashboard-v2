package secrets

import (
	"strings"
	"testing"
)

type memoryStore map[string]string

func (s memoryStore) Get(key string) (string, error) { return s[key], nil }
func (s memoryStore) Set(key, value string) error    { s[key] = value; return nil }

func TestRoundTrip(t *testing.T) {
	store := memoryStore{}
	box, err := Open(store)
	if err != nil {
		t.Fatal(err)
	}

	const secret = "0123456789abcdef0123456789abcdef01234567"
	first, err := box.Encrypt(secret)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := box.Encrypt(secret)
	if first == second {
		t.Error("the same secret must encrypt differently every time")
	}
	if strings.Contains(first, secret) {
		t.Error("the encrypted text contains the secret")
	}

	// A new box over the same store (a restart) reads the old values
	reopened, err := Open(store)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := reopened.Decrypt(first); err != nil || got != secret {
		t.Errorf("Decrypt = %q, %v", got, err)
	}
}

func TestDecryptRejectsForeignAndChangedValues(t *testing.T) {
	box, _ := Open(memoryStore{})
	other, _ := Open(memoryStore{})

	encrypted, _ := box.Encrypt("secret")
	if _, err := other.Decrypt(encrypted); err == nil {
		t.Error("a value encrypted with another master key must not decrypt")
	}

	changed := []byte(encrypted)
	changed[len(changed)-3] ^= 1
	if _, err := box.Decrypt(string(changed)); err == nil {
		t.Error("a changed value must not decrypt")
	}
	for _, bad := range []string{"", "not base64 !", "QUJD"} {
		if _, err := box.Decrypt(bad); err == nil {
			t.Errorf("Decrypt(%q) must fail", bad)
		}
	}
}

func TestOpenKeepsADamagedKey(t *testing.T) {
	store := memoryStore{masterKeyName: "damaged"}
	if _, err := Open(store); err == nil {
		t.Fatal("a damaged master key must be an error")
	}
	if store[masterKeyName] != "damaged" {
		t.Error("a damaged master key must not be replaced")
	}
}
