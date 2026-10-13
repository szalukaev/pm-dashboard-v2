package jobs

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"pm-dashboard/mail"
	"pm-dashboard/notify"

	"github.com/lib/pq"
)

// MailKey — имя рассылки среди воркеров.
const MailKey = "mail"

const (
	// mailTick — как часто рассылка проверяет, не пора ли что-то отправить.
	mailTick = 5 * time.Minute
	// alertMailWindow — уведомления старше в письмо не попадают: если почту
	// настроили позже, накопившееся за недели разом не уйдёт.
	alertMailWindow = 24 * time.Hour
	// alertMailLimit — сколько уведомлений в одном письме.
	alertMailLimit = 100
)

// Mailer отправляет почту: письма с уведомлениями по правилам пользователей
// и регулярные сводки.
type Mailer struct {
	DB       **sql.DB
	Settings *Settings
	Store    Store
	Mail     *mail.Store
	// Send отправляет одно письмо; по умолчанию mail.Send. Подменяется в тестах.
	Send func(cfg mail.Config, to, subject, html string) error
	// Reports отправляет регулярные сводки, которым пришло время, и
	// возвращает число отправленных; может быть nil.
	Reports func(cfg mail.Config, now time.Time) (int, error)
	// Notify сообщает открытым страницам о смене состояния; может быть nil.
	Notify func(event string, data interface{})

	mu      sync.Mutex
	running bool
}

// Run — цикл рассылки; заканчивается вместе с ctx.
func (m *Mailer) Run(ctx context.Context) {
	ticker := time.NewTicker(mailTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if m.DB == nil || *m.DB == nil || !m.Settings.MailEnabled() {
			continue
		}
		m.RunNow(false)
	}
}

// Running сообщает, идёт ли рассылка прямо сейчас.
func (m *Mailer) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// Status возвращает состояние последнего запуска.
func (m *Mailer) Status() Status {
	status, ok := m.Store.Load(MailKey)
	if !ok {
		status = Status{State: StateIdle}
	}
	// Запуск, прерванный перезапуском сервера, оставил после себя «идёт»
	if status.State == StateRunning && !m.Running() {
		status.State = StateIdle
	}
	return status
}

// RunNow отправляет всё, чему пришло время, и возвращает число отправленных
// писем. force — запуск администратором: письма с уведомлениями уходят, не
// дожидаясь часа с прошлого письма.
func (m *Mailer) RunNow(force bool) (int64, error) {
	if m.DB == nil || *m.DB == nil {
		return 0, ErrNoDatabase
	}
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return 0, ErrBusy
	}
	m.running = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()

	cfg, err := m.Mail.Load()
	if err != nil {
		return 0, err
	}
	// Почта не настроена или выключена: отправлять нечем, и это не сбой
	if !cfg.Ready() {
		if force {
			return 0, mail.ErrNotConfigured
		}
		return 0, nil
	}

	started := time.Now()
	last := m.Status()
	// Проверка, после которой отправлять было нечего, не затирает сведения о
	// последней настоящей рассылке
	sent, err := m.alerts(cfg, started, force)
	if m.Reports != nil {
		reports, reportsErr := m.Reports(cfg, started)
		sent += reports
		if err == nil {
			err = reportsErr
		}
	}
	if sent == 0 && err == nil && !force && last.LastStart != nil {
		return 0, nil
	}

	status := Status{State: StateIdle, LastStart: &started, DurationMs: time.Since(started).Milliseconds(), Processed: int64(sent)}
	if err != nil {
		status.State, status.Error = StateError, err.Error()
		slog.Error("Mailing failed", "sent", sent, "error", err)
	} else if sent > 0 {
		slog.Info("Mail sent", "letters", sent)
	}
	m.Store.Save(MailKey, status)
	if m.Notify != nil {
		m.Notify(EventWorkersChanged, nil)
	}
	return int64(sent), err
}

// alerts отправляет пользователям письма с их неотправленными уведомлениями:
// одно письмо на пользователя, не чаще интервала, заданного администратором в
// расписании рассылки. Возвращает число писем и первую ошибку отправки.
func (m *Mailer) alerts(cfg mail.Config, now time.Time, force bool) (int, error) {
	since := now.Add(-alertMailWindow)
	quiet := now.Add(-time.Duration(m.Settings.MailAlertInterval()) * time.Minute)
	if force {
		// Никакое письмо не моложе этого момента: час ждать не нужно
		quiet = now.Add(time.Hour)
	}
	rows, err := (*m.DB).Query(`SELECT u.id, u.email, COALESCE(us.language, 'ru')
		FROM users u LEFT JOIN user_settings us ON us.user_id = u.id
		WHERE u.email <> '' AND NOT COALESCE(u.is_blocked, false)
		AND EXISTS (SELECT 1 FROM notifications n WHERE n.user_id = u.id AND n.to_email
			AND n.emailed_at IS NULL AND n.created_at > $1)
		AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.user_id = u.id AND n.emailed_at > $2)`, since, quiet)
	if err != nil {
		return 0, err
	}
	type target struct {
		userID      int
		email, lang string
	}
	var targets []target
	for rows.Next() {
		var t target
		if rows.Scan(&t.userID, &t.email, &t.lang) == nil {
			targets = append(targets, t)
		}
	}
	rows.Close()

	send := m.Send
	if send == nil {
		send = mail.Send
	}
	sent := 0
	var firstErr error
	for _, t := range targets {
		list, ids, err := m.pending(t.userID, since)
		if err != nil || len(list) == 0 {
			continue
		}
		subject, body := notify.Digest(list, t.lang, cfg.TaskURL)
		if err := send(cfg, t.email, subject, body); err != nil {
			slog.Warn("Could not send the notifications by mail", "user", t.userID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		(*m.DB).Exec("UPDATE notifications SET emailed_at = $1 WHERE id = ANY($2)", now, pq.Array(ids))
		sent++
	}
	return sent, firstErr
}

// pending возвращает неотправленные уведомления пользователя для письма,
// старые сверху.
func (m *Mailer) pending(userID int, since time.Time) ([]notify.Notification, []int64, error) {
	rows, err := (*m.DB).Query(`SELECT id, rule, params, entity_type, entity_id, created_at
		FROM notifications WHERE user_id = $1 AND to_email AND emailed_at IS NULL AND created_at > $2
		ORDER BY created_at, id LIMIT $3`, userID, since, alertMailLimit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var list []notify.Notification
	var ids []int64
	for rows.Next() {
		var n notify.Notification
		var params []byte
		if rows.Scan(&n.ID, &n.Rule, &params, &n.EntityType, &n.EntityID, &n.CreatedAt) == nil {
			n.Params = params
			list = append(list, n)
			ids = append(ids, n.ID)
		}
	}
	return list, ids, rows.Err()
}
