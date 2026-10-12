package mail

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestValidAddress(t *testing.T) {
	for _, address := range []string{"user@example.com", "first.last+tag@sub.example.org"} {
		if !ValidAddress(address) {
			t.Errorf("адрес %q должен приниматься", address)
		}
	}
	invalid := []string{
		"", "user", "user@", "@example.com",
		"Имя <user@example.com>",          // только сам адрес, без имени
		"a@example.com, b@example.com",    // один адрес, не список
		"user@example.com\r\nBcc: x@y.zz", // перенос строки — попытка подставить заголовок
		"user @example.com",
		strings.Repeat("a", 250) + "@example.com",
	}
	for _, address := range invalid {
		if ValidAddress(address) {
			t.Errorf("адрес %q не должен приниматься", address)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	good := Config{Enabled: true, Host: "smtp.example.com", Port: 587, Security: SecurityStartTLS, From: "pm@example.com"}
	if code := good.Validate(); code != "" {
		t.Fatalf("годные настройки отклонены: %s", code)
	}
	if !good.Ready() {
		t.Error("с годными настройками можно отправлять")
	}

	cases := []struct {
		name   string
		change func(*Config)
		code   string
	}{
		{"нет сервера", func(c *Config) { c.Host = "" }, "INVALID_SMTP_HOST"},
		{"перенос строки в сервере", func(c *Config) { c.Host = "smtp.example.com\r\nx" }, "INVALID_SMTP_HOST"},
		{"порт 0", func(c *Config) { c.Port = 0 }, "INVALID_SMTP_PORT"},
		{"порт больше 65535", func(c *Config) { c.Port = 70000 }, "INVALID_SMTP_PORT"},
		{"неизвестное шифрование", func(c *Config) { c.Security = "ssl3" }, "INVALID_SMTP_SECURITY"},
		{"адрес отправителя не адрес", func(c *Config) { c.From = "pm" }, "INVALID_SMTP_FROM"},
		{"перенос строки в подписи", func(c *Config) { c.FromName = "PM\r\nBcc: x@y.zz" }, "INVALID_SMTP_FROM"},
	}
	for _, c := range cases {
		cfg := good
		c.change(&cfg)
		if got := cfg.Validate(); got != c.code {
			t.Errorf("%s: получено %q, ожидалось %q", c.name, got, c.code)
		}
		if cfg.Ready() {
			t.Errorf("%s: с такими настройками отправлять нельзя", c.name)
		}
	}

	off := good
	off.Enabled = false
	if off.Ready() {
		t.Error("выключенная отправка не готова, даже если настройки годные")
	}
}

func TestMessage(t *testing.T) {
	cfg := Config{From: "pm@example.com", FromName: "PM Dashboard"}
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	html := "<p>" + strings.Repeat("Сводка по задачам. ", 20) + "</p>"
	message := string(Message(cfg, "user@example.com", "Сводка за неделю", html, at))

	head, body, found := strings.Cut(message, "\r\n\r\n")
	if !found {
		t.Fatal("заголовки должны отделяться от текста пустой строкой")
	}
	for _, want := range []string{
		`From: "PM Dashboard" <pm@example.com>`,
		"To: user@example.com",
		"Content-Type: text/html; charset=utf-8",
		"Content-Transfer-Encoding: base64",
		"MIME-Version: 1.0",
	} {
		if !strings.Contains(head, want) {
			t.Errorf("в заголовках нет %q:\n%s", want, head)
		}
	}
	// Русская тема не идёт в заголовок как есть
	if strings.Contains(head, "Сводка") || !strings.Contains(head, "Subject: =?utf-8?q?") {
		t.Errorf("тема должна быть закодирована:\n%s", head)
	}

	// Текст — base64 строками не длиннее 76 знаков, раскодируется обратно
	var encoded strings.Builder
	for _, line := range strings.Split(strings.TrimSpace(body), "\r\n") {
		if len(line) > 76 {
			t.Errorf("строка длиннее 76 знаков: %d", len(line))
		}
		encoded.WriteString(line)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded.String())
	if err != nil || string(decoded) != html {
		t.Errorf("текст письма не раскодировался обратно: %v", err)
	}
}

func TestSendNeedsConfig(t *testing.T) {
	if err := Send(Config{}, "user@example.com", "тема", "<p>текст</p>"); err != ErrNotConfigured {
		t.Errorf("без настроек: %v", err)
	}
	cfg := Config{Enabled: true, Host: "smtp.example.com", Port: 587, Security: SecurityStartTLS, From: "pm@example.com"}
	if err := Send(cfg, "не адрес", "тема", "<p>текст</p>"); err == nil || err == ErrNotConfigured {
		t.Errorf("негодный адрес получателя: %v", err)
	}
}
