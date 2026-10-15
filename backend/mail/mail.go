// Пакет mail отправляет письма через SMTP-сервер, заданный администратором.
// Настройки лежат в базе, пароль — зашифрованным.
package mail

import (
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	netmail "net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Как защищено соединение с сервером.
const (
	SecurityNone     = "none"     // без шифрования
	SecurityStartTLS = "starttls" // шифрование включается после подключения (обычно порт 587)
	SecurityTLS      = "tls"      // шифрование с самого начала (обычно порт 465)
)

// settingsKey — под этим ключом настройки лежат в таблице settings.
const settingsKey = "smtp"

// sendTimeout — сколько ждать сервер, прежде чем признать отправку неудачной.
const sendTimeout = 30 * time.Second

var (
	// ErrNotConfigured: сервер не задан или отправка выключена.
	ErrNotConfigured = errors.New("smtp is not configured")
	// ErrNoDatabase: без базы настройки не прочитать.
	ErrNoDatabase = errors.New("database is not available")
)

// Config — настройки отправки почты.
type Config struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Security string `json:"security"`
	Username string `json:"username"`
	// From — адрес отправителя; FromName — подпись рядом с ним.
	From     string `json:"from"`
	FromName string `json:"from_name"`
	// BaseURL — адрес, по которому пользователи открывают дашборд
	// (http://host:port). С ним номер и название задачи в письме становятся
	// ссылкой на её карточку; без него остаются текстом.
	BaseURL string `json:"base_url"`
	// Password хранится в базе зашифрованным (PasswordEnc) и наружу не
	// отдаётся.
	Password    string `json:"-"`
	PasswordEnc string `json:"password_enc,omitempty"`
}

// Cipher шифрует и расшифровывает пароль (secrets.Box).
type Cipher interface {
	Encrypt(plain string) (string, error)
	Decrypt(encrypted string) (string, error)
}

// Store читает и сохраняет настройки почты.
type Store struct {
	DB  **sql.DB
	Box Cipher
}

// Load возвращает настройки с расшифрованным паролем. Если их ещё не
// задавали, возвращает пустые настройки без ошибки.
func (s *Store) Load() (Config, error) {
	cfg := Config{Port: 587, Security: SecurityStartTLS}
	if s.DB == nil || *s.DB == nil {
		return cfg, ErrNoDatabase
	}
	var raw []byte
	err := (*s.DB).QueryRow(`SELECT value FROM settings WHERE key = $1`, settingsKey).Scan(&raw)
	if err == sql.ErrNoRows {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, err
	}
	if cfg.PasswordEnc != "" && s.Box != nil {
		if plain, err := s.Box.Decrypt(cfg.PasswordEnc); err == nil {
			cfg.Password = plain
		}
	}
	return cfg, nil
}

// Save сохраняет настройки. password — новый пароль; nil оставляет прежний.
func (s *Store) Save(cfg Config, password *string) error {
	if s.DB == nil || *s.DB == nil {
		return ErrNoDatabase
	}
	if password == nil {
		current, err := s.Load()
		if err != nil {
			return err
		}
		cfg.PasswordEnc = current.PasswordEnc
	} else if *password == "" {
		cfg.PasswordEnc = ""
	} else {
		if s.Box == nil {
			return errors.New("secret store is not available")
		}
		encrypted, err := s.Box.Encrypt(*password)
		if err != nil {
			return err
		}
		cfg.PasswordEnc = encrypted
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = (*s.DB).Exec(`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, settingsKey, data)
	return err
}

// Validate проверяет настройки, которые собираются сохранить, и возвращает
// код ошибки для интерфейса или пустую строку.
func (c Config) Validate() string {
	switch {
	case strings.TrimSpace(c.Host) == "" || strings.ContainsAny(c.Host, " \r\n"):
		return "INVALID_SMTP_HOST"
	case c.Port < 1 || c.Port > 65535:
		return "INVALID_SMTP_PORT"
	case c.Security != SecurityNone && c.Security != SecurityStartTLS && c.Security != SecurityTLS:
		return "INVALID_SMTP_SECURITY"
	case !ValidAddress(c.From):
		return "INVALID_SMTP_FROM"
	case strings.ContainsAny(c.FromName, "\r\n") || len(c.FromName) > 200:
		return "INVALID_SMTP_FROM"
	case c.BaseURL != "" && !validBaseURL(c.BaseURL):
		return "INVALID_BASE_URL"
	}
	return ""
}

// validBaseURL сообщает, что строка — адрес сайта: http или https, с именем
// сервера, без параметров и пробелов.
func validBaseURL(address string) bool {
	if len(address) > 255 || strings.ContainsAny(address, " \r\n\"'<>") {
		return false
	}
	u, err := url.Parse(address)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.RawQuery == "" && u.Fragment == ""
}

// TaskURL возвращает ссылку на карточку задачи в дашборде или пустую строку,
// если адрес дашборда не задан.
func (c Config) TaskURL(issueID int) string {
	if c.BaseURL == "" || !validBaseURL(c.BaseURL) || issueID <= 0 {
		return ""
	}
	return strings.TrimRight(c.BaseURL, "/") + "/tasks?task=" + strconv.Itoa(issueID)
}

// Ready сообщает, можно ли отправлять письма с такими настройками.
func (c Config) Ready() bool {
	return c.Enabled && c.Validate() == ""
}

// ValidAddress сообщает, что строка — один адрес электронной почты, без
// имени и переносов строк.
func ValidAddress(address string) bool {
	if address == "" || len(address) > 255 || strings.ContainsAny(address, "\r\n<>, ") {
		return false
	}
	parsed, err := netmail.ParseAddress(address)
	return err == nil && parsed.Address == address && strings.Contains(address, "@")
}

// Message собирает письмо в формате, который принимает SMTP-сервер: тема и
// подпись отправителя кодируются, текст письма — HTML в UTF-8.
func Message(cfg Config, to, subject, html string, now time.Time) []byte {
	from := (&netmail.Address{Name: cfg.FromName, Address: cfg.From}).String()
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + now.Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=utf-8",
		"Content-Transfer-Encoding: base64",
	}
	body := base64.StdEncoding.EncodeToString([]byte(html))
	// Строки длиннее 76 знаков почтовые серверы не обязаны принимать
	var lines []string
	for len(body) > 76 {
		lines = append(lines, body[:76])
		body = body[76:]
	}
	lines = append(lines, body)
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.Join(lines, "\r\n") + "\r\n")
}

// Send отправляет письмо на один адрес.
func Send(cfg Config, to, subject, html string) error {
	if !cfg.Ready() {
		return ErrNotConfigured
	}
	if !ValidAddress(to) {
		return fmt.Errorf("invalid recipient address")
	}

	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := &net.Dialer{Timeout: sendTimeout}
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}

	var conn net.Conn
	var err error
	if cfg.Security == SecurityTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	} else {
		conn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(sendTimeout))

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("greeting: %w", err)
	}
	defer client.Close()

	if cfg.Security == SecurityStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("the server does not offer STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if cfg.Username != "" {
		// Пароль по незашифрованному соединению стандартная библиотека
		// отправлять откажется — и правильно
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := writer.Write(Message(cfg, to, subject, html, time.Now())); err != nil {
		return fmt.Errorf("body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("body: %w", err)
	}
	return client.Quit()
}
