// Package push отправляет push-уведомления в браузеры и установленное
// веб-приложение (Web Push, ключи VAPID). Push приходит о том же, о чём
// колокольчик: его вызывает движок уведомлений.
package push

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// settingsKey — где в таблице settings лежат ключи VAPID.
const settingsKey = "webpush_vapid"

// fallbackSubject — контакт отправителя, если адрес дашборда не задан.
const fallbackSubject = "mailto:push@pm-dashboard.local"

// Keys — пара ключей VAPID: открытый отдаётся браузеру при подписке,
// закрытым подписываются отправки.
type Keys struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

// Subscription — подписка одного браузера или устройства.
type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Message — то, что получит service worker.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

// Service хранит подписки и рассылает по ним push.
type Service struct {
	DB **sql.DB
	// Subject возвращает контакт отправителя: адрес дашборда (https://…)
	// или mailto. nil или пустая строка — служебный адрес.
	Subject func() string
	// Send отправляет одно сообщение; в тестах подменяется.
	Send func(payload []byte, sub *webpush.Subscription, opts *webpush.Options) (*http.Response, error)

	mu   sync.Mutex
	keys *Keys
}

// New создаёт сервис с настоящей отправкой.
func New(db **sql.DB, subject func() string) *Service {
	return &Service{DB: db, Subject: subject, Send: webpush.SendNotification}
}

// PublicKey возвращает открытый ключ VAPID, при первом обращении создавая
// пару ключей и сохраняя её в базе.
func (s *Service) PublicKey() (string, error) {
	keys, err := s.loadKeys()
	if err != nil {
		return "", err
	}
	return keys.Public, nil
}

func (s *Service) loadKeys() (*Keys, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keys != nil {
		return s.keys, nil
	}
	db := *s.DB
	var raw []byte
	err := db.QueryRow(`SELECT value FROM settings WHERE key = $1`, settingsKey).Scan(&raw)
	if err == sql.ErrNoRows {
		private, public, genErr := webpush.GenerateVAPIDKeys()
		if genErr != nil {
			return nil, genErr
		}
		data, _ := json.Marshal(Keys{Public: public, Private: private})
		// Два сервера могли создать ключи одновременно — остаётся первая пара
		if _, err = db.Exec(`INSERT INTO settings (key, value) VALUES ($1, $2::jsonb) ON CONFLICT (key) DO NOTHING`, settingsKey, string(data)); err != nil {
			return nil, err
		}
		err = db.QueryRow(`SELECT value FROM settings WHERE key = $1`, settingsKey).Scan(&raw)
	}
	if err != nil {
		return nil, err
	}
	var keys Keys
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, err
	}
	s.keys = &keys
	return s.keys, nil
}

// Subscribe сохраняет подписку пользователя; та же подписка другого
// пользователя (сменили учётную запись на устройстве) переходит к этому.
func (s *Service) Subscribe(userID int, sub Subscription, userAgent string) error {
	if len(userAgent) > 255 {
		userAgent = userAgent[:255]
	}
	_, err := (*s.DB).Exec(`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE SET user_id = $1, p256dh = $3, auth = $4, user_agent = $5`,
		userID, sub.Endpoint, sub.Keys.P256dh, sub.Keys.Auth, userAgent)
	return err
}

// Unsubscribe удаляет подписку пользователя.
func (s *Service) Unsubscribe(userID int, endpoint string) error {
	_, err := (*s.DB).Exec(`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`, userID, endpoint)
	return err
}

// Notify рассылает сообщение на все устройства пользователя. Подписки,
// которые сервис push больше не знает (404, 410), удаляются.
func (s *Service) Notify(userID int, msg Message) {
	if *s.DB == nil {
		return
	}
	keys, err := s.loadKeys()
	if err != nil {
		slog.Error("Push: no VAPID keys", "error", err)
		return
	}
	rows, err := (*s.DB).Query(`SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID)
	if err != nil {
		slog.Error("Push: failed to read subscriptions", "user", userID, "error", err)
		return
	}
	var subs []webpush.Subscription
	for rows.Next() {
		var sub webpush.Subscription
		if rows.Scan(&sub.Endpoint, &sub.Keys.P256dh, &sub.Keys.Auth) == nil {
			subs = append(subs, sub)
		}
	}
	rows.Close()
	if len(subs) == 0 {
		return
	}

	payload, _ := json.Marshal(msg)
	subject := fallbackSubject
	if s.Subject != nil {
		if v := s.Subject(); v != "" {
			subject = v
		}
	}
	opts := &webpush.Options{
		Subscriber:      subject,
		VAPIDPublicKey:  keys.Public,
		VAPIDPrivateKey: keys.Private,
		TTL:             int((24 * time.Hour).Seconds()),
		Urgency:         webpush.UrgencyHigh,
	}
	for i := range subs {
		resp, err := s.Send(payload, &subs[i], opts)
		if err != nil {
			slog.Warn("Push: send failed", "user", userID, "error", err)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
			// Подписка отозвана (приложение удалено, разрешение снято)
			(*s.DB).Exec(`DELETE FROM push_subscriptions WHERE endpoint = $1`, subs[i].Endpoint)
		case resp.StatusCode >= 400:
			slog.Warn("Push: rejected", "user", userID, "status", resp.StatusCode)
		}
	}
}
