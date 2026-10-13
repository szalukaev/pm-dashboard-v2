package middleware

import (
	"database/sql"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDebugBodyMasksSecrets(t *testing.T) {
	raw := []byte(`{"username":"ivan","password":"qwerty","nested":{"api_key":"abc","smtp_password":"p","source_token":"t","note":"ok"}}`)
	got := debugBody(raw, "application/json")
	for _, secret := range []string{"qwerty", `"abc"`, `"p"`, `"t"`} {
		if strings.Contains(got, secret) {
			t.Errorf("secret %s is left in %s", secret, got)
		}
	}
	for _, kept := range []string{"ivan", `"note":"ok"`} {
		if !strings.Contains(got, kept) {
			t.Errorf("%s is lost in %s", kept, got)
		}
	}
}

func TestDebugBodyKeepsOnlyJSON(t *testing.T) {
	// Не JSON — в журнал уходит только пометка: в тексте секрет не найти по имени поля
	got := debugBody([]byte("password=qwerty&user=ivan"), "application/x-www-form-urlencoded")
	if strings.Contains(got, "qwerty") || !strings.Contains(got, "not saved") {
		t.Errorf("form body saved as %q", got)
	}
	// Слишком большое тело не сохраняется
	big := []byte(`{"a":"` + strings.Repeat("x", maxDebugBody) + `"}`)
	if got := debugBody(big, "application/json"); !strings.Contains(got, "not saved") {
		t.Errorf("big body saved, %d bytes", len(got))
	}
	if got := debugBody(nil, "application/json"); got != "" {
		t.Errorf("empty body saved as %q", got)
	}
}

func TestMaskQuery(t *testing.T) {
	got := maskQuery(url.Values{"token": {"abc"}, "limit": {"25"}})
	if strings.Contains(got, "abc") || !strings.Contains(got, "limit=25") {
		t.Errorf("maskQuery = %q", got)
	}
}

func TestDebugActiveUntilExpires(t *testing.T) {
	var db *sql.DB
	d := &DebugLog{DB: &db, loaded: time.Now(), until: map[int]time.Time{
		1: time.Now().Add(time.Hour),
		2: time.Now().Add(-time.Minute),
	}}
	if !d.active(1) {
		t.Error("user 1: debug must be on")
	}
	// Срок истёк — режим выключен без чьих-либо действий
	if d.active(2) {
		t.Error("user 2: debug must be off after the period")
	}
	if d.active(3) {
		t.Error("user 3: debug was never turned on")
	}
}
