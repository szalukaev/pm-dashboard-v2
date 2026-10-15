package license

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Состояние установки по отношению к её лицензии.
type State string

const (
	// NotActivated: лицензия ни разу не активировалась; работает только
	// экран активации.
	NotActivated State = "not_activated"
	// Active: лицензия действительна для этого железа.
	Active State = "active"
	// Grace: последняя проверка не прошла; всё работает, идёт отсчёт до
	// режима «только чтение».
	Grace State = "grace"
	// ReadOnly: льготный период истёк; данные можно читать, но не менять.
	ReadOnly State = "read_only"
)

// Почему проверка не прошла.
const (
	ReasonSignature       = "signature"        // сохранённый токен не проходит проверку подписи
	ReasonHWIDMismatch    = "hwid_mismatch"    // железо не то, на которое выдана лицензия
	ReasonHWIDUnavailable = "hwid_unavailable" // отпечаток хоста прочитать не удалось
)

// События, которые пишутся в журнал лицензии (таблица license_events).
const (
	EventActivated        = "activated"
	EventActivationFailed = "activation_failed"
	EventCheckFailed      = "check_failed"
	EventGraceStarted     = "grace_started"
	EventReadOnlyEntered  = "read_only_entered"
	EventRestored         = "restored"
)

// CheckInterval — как часто лицензия перепроверяется во время работы.
const CheckInterval = time.Hour

// Status — результат проверки лицензии.
type Status struct {
	State  State  `json:"state"`
	Reason string `json:"reason,omitempty"`

	Client   string   `json:"client,omitempty"`
	Edition  string   `json:"edition,omitempty"`
	Features []string `json:"features,omitempty"`

	GracePeriodDays int        `json:"grace_period_days,omitempty"`
	GraceEndsAt     *time.Time `json:"grace_ends_at,omitempty"`
	ActivatedAt     *time.Time `json:"activated_at,omitempty"`
	LastCheckOK     *time.Time `json:"last_check_ok,omitempty"`
	CheckedAt       time.Time  `json:"checked_at"`
}

// Ошибки Activate.
var (
	ErrHWIDUnavailable = errors.New("hardware fingerprint of the host is not available")
	ErrHWIDMismatch    = errors.New("the license was issued for other hardware")
	ErrNoDatabase      = errors.New("database is not available")
)

// Service — единственный владелец состояния лицензии.
type Service struct {
	db       **sql.DB
	hwidPath string
	now      func() time.Time

	mu     sync.RWMutex
	status Status
}

// New создаёт сервис. До первого Check состояние неизвестно (не
// активирована).
func New(db **sql.DB, hwidPath string) *Service {
	return &Service{
		db:       db,
		hwidPath: hwidPath,
		now:      time.Now,
		status:   Status{State: NotActivated},
	}
}

// Status возвращает результат последней проверки.
func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// IsReadOnly сообщает, запрещено ли менять данные.
func (s *Service) IsReadOnly() bool {
	return s.Status().State == ReadOnly
}

// Usable сообщает, можно ли вообще пользоваться продуктом (лицензия
// когда-либо активировалась).
func (s *Service) Usable() bool {
	return s.Status().State != NotActivated
}

// HWID возвращает отпечаток хоста для экрана активации.
func (s *Service) HWID() (HWID, error) {
	hwid, err := ReadHWID(s.hwidPath)
	if err != nil {
		return HWID{}, ErrHWIDUnavailable
	}
	if hwid.Empty() {
		return HWID{}, ErrHWIDUnavailable
	}
	return hwid, nil
}

// Run проверяет лицензию сразу и затем каждые CheckInterval, пока жив ctx.
func (s *Service) Run(ctx context.Context) {
	s.Check()
	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Check()
		}
	}
}

// stored — строка лицензии из базы.
type stored struct {
	token        string
	activatedAt  *time.Time
	lastCheckOK  *time.Time
	graceStarted *time.Time
}

// verdict — решение проверки до того, как что-либо записано.
type verdict struct {
	state   State
	reason  string
	payload *Payload
	// graceStarted — когда начался льготный период; nil — с лицензией всё в порядке.
	graceStarted *time.Time
}

// evaluate определяет состояние по сохранённой лицензии, железу хоста и
// текущему времени. Это вся логика проверки; побочных эффектов у неё нет.
func evaluate(lic *stored, hwid HWID, hwidErr error, now time.Time) verdict {
	if lic == nil {
		return verdict{state: NotActivated}
	}

	payload, err := Verify(lic.token)
	reason := ""
	switch {
	case err != nil:
		reason = ReasonSignature
	case hwidErr != nil:
		reason = ReasonHWIDUnavailable
	case !hwid.Matches(payload.HWID()):
		reason = ReasonHWIDMismatch
	}
	if reason == "" {
		return verdict{state: Active, payload: payload}
	}

	// Проверка не прошла: льготный период отсчитывается от первой неудачи.
	// Его длина — параметр подписанной лицензии; лицензия, которую не
	// удаётся прочитать, даёт длину по умолчанию.
	started := now
	if lic.graceStarted != nil {
		started = *lic.graceStarted
	}
	state := Grace
	if !now.Before(started.Add(time.Duration(payload.GraceDays()) * 24 * time.Hour)) {
		state = ReadOnly
	}
	return verdict{state: state, reason: reason, payload: payload, graceStarted: &started}
}

func (s *Service) load() (*stored, error) {
	if s.db == nil || *s.db == nil {
		return nil, ErrNoDatabase
	}
	var lic stored
	err := (*s.db).QueryRow(`SELECT license_blob, first_activated_at, last_check_ok_at, grace_started_at
		FROM licenses ORDER BY id DESC LIMIT 1`).Scan(&lic.token, &lic.activatedAt, &lic.lastCheckOK, &lic.graceStarted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &lic, nil
}

// Check выполняет полную проверку — подпись и железо, — записывает итог и
// возвращает новое состояние.
func (s *Service) Check() Status {
	now := s.now()
	lic, err := s.load()
	if err != nil {
		// Без базы нельзя ни решить, ни записать; остаётся последнее
		// известное состояние (сразу после запуска — не активирована).
		if !errors.Is(err, ErrNoDatabase) {
			slog.Warn("License check: cannot read the license", "error", err)
		}
		return s.Status()
	}

	hwid, hwidErr := s.HWID()
	v := evaluate(lic, hwid, hwidErr, now)
	previous := s.Status().State

	status := Status{State: v.state, Reason: v.reason, CheckedAt: now}
	if lic != nil {
		status.ActivatedAt = lic.activatedAt
		status.LastCheckOK = lic.lastCheckOK
	}
	if v.payload != nil {
		status.Client, status.Edition, status.Features = v.payload.Client, v.payload.Edition, v.payload.Features
		status.GracePeriodDays = v.payload.GraceDays()
	}

	db := *s.db
	switch v.state {
	case Active:
		if _, err := db.Exec(`UPDATE licenses SET last_check_ok_at = $1, grace_started_at = NULL WHERE license_blob = $2`, now, lic.token); err != nil {
			slog.Warn("Database write failed", "at", "Check", "error", err)
		}
		status.LastCheckOK = &now
		if previous == Grace || previous == ReadOnly || lic.graceStarted != nil {
			s.logEvent(EventRestored, "")
		}
	case Grace, ReadOnly:
		if lic.graceStarted == nil {
			if _, err := db.Exec(`UPDATE licenses SET grace_started_at = $1 WHERE license_blob = $2`, *v.graceStarted, lic.token); err != nil {
				slog.Warn("Database write failed", "at", "Check", "error", err)
			}
			s.logEvent(EventGraceStarted, v.reason)
		}
		ends := v.graceStarted.Add(time.Duration(v.payload.GraceDays()) * 24 * time.Hour)
		status.GraceEndsAt = &ends
		status.GracePeriodDays = v.payload.GraceDays()
		s.logEvent(EventCheckFailed, v.reason)
		if v.state == ReadOnly && previous != ReadOnly {
			s.logEvent(EventReadOnlyEntered, v.reason)
		}
	}

	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
	if v.state != previous {
		slog.Info("License state", "state", v.state, "reason", v.reason)
	}
	return status
}

// Activate сохраняет новый токен лицензии, проверив его подпись и то, что
// он выпущен для этого железа.
func (s *Service) Activate(token string) (Status, error) {
	if s.db == nil || *s.db == nil {
		return s.Status(), ErrNoDatabase
	}
	fail := func(err error, details string) (Status, error) {
		s.logEvent(EventActivationFailed, details)
		return s.Status(), err
	}

	payload, err := Verify(token)
	if err != nil {
		return fail(err, ReasonSignature)
	}
	hwid, err := s.HWID()
	if err != nil {
		return fail(ErrHWIDUnavailable, ReasonHWIDUnavailable)
	}
	if !hwid.Matches(payload.HWID()) {
		return fail(ErrHWIDMismatch, ReasonHWIDMismatch)
	}

	// Действующая лицензия одна: новая заменяет прежнюю.
	now := s.now()
	tx, err := (*s.db).Begin()
	if err != nil {
		return s.Status(), err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM licenses`); err != nil {
		return s.Status(), err
	}
	if _, err := tx.Exec(`INSERT INTO licenses (license_blob, hwid_hash, first_activated_at, last_check_ok_at)
		VALUES ($1, $2, $3, $3)`, token, hwid.String(), now); err != nil {
		return s.Status(), err
	}
	if err := tx.Commit(); err != nil {
		return s.Status(), err
	}
	s.logEvent(EventActivated, payload.Client)
	return s.Check(), nil
}

// Event — строка журнала лицензии.
type Event struct {
	At      time.Time `json:"at"`
	Event   string    `json:"event"`
	Details string    `json:"details"`
}

// Events возвращает последние строки журнала лицензии, сначала новые.
func (s *Service) Events(limit int) ([]Event, error) {
	if s.db == nil || *s.db == nil {
		return []Event{}, nil
	}
	rows, err := (*s.db).Query(`SELECT created_at, event, details FROM license_events ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.At, &e.Event, &e.Details); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// logEvent пишет в журнал лицензии. Он ведётся отдельно от аудита действий
// пользователей: по нему поддержка видит, что происходило с лицензией.
func (s *Service) logEvent(event, details string) {
	if s.db == nil || *s.db == nil {
		return
	}
	if _, err := (*s.db).Exec(`INSERT INTO license_events (event, details) VALUES ($1, $2)`, event, details); err != nil {
		slog.Warn("License journal: cannot write", "event", event, "error", err)
	}
}
