package license

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// State of the installation with respect to its license.
type State string

const (
	// NotActivated: no license was ever activated; only the activation
	// screen works.
	NotActivated State = "not_activated"
	// Active: the license is valid for this hardware.
	Active State = "active"
	// Grace: the last check failed; everything works, the countdown to
	// read-only is running.
	Grace State = "grace"
	// ReadOnly: the grace period is over; data can be read but not changed.
	ReadOnly State = "read_only"
)

// Why a check failed.
const (
	ReasonSignature       = "signature"        // the stored token does not verify
	ReasonHWIDMismatch    = "hwid_mismatch"    // the hardware is not the licensed one
	ReasonHWIDUnavailable = "hwid_unavailable" // the fingerprint of the host cannot be read
)

// Events written to the license journal (table license_events).
const (
	EventActivated        = "activated"
	EventActivationFailed = "activation_failed"
	EventCheckFailed      = "check_failed"
	EventGraceStarted     = "grace_started"
	EventReadOnlyEntered  = "read_only_entered"
	EventRestored         = "restored"
)

// CheckInterval is how often the license is re-checked at runtime.
const CheckInterval = time.Hour

// Status is the result of a license check.
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

// Errors of Activate.
var (
	ErrHWIDUnavailable = errors.New("hardware fingerprint of the host is not available")
	ErrHWIDMismatch    = errors.New("the license was issued for other hardware")
	ErrNoDatabase      = errors.New("database is not available")
)

// Service is the single owner of the license state.
type Service struct {
	db       **sql.DB
	hwidPath string
	now      func() time.Time

	mu     sync.RWMutex
	status Status
}

// New creates the service. The state is unknown (not activated) until the
// first Check.
func New(db **sql.DB, hwidPath string) *Service {
	return &Service{
		db:       db,
		hwidPath: hwidPath,
		now:      time.Now,
		status:   Status{State: NotActivated},
	}
}

// Status returns the result of the last check.
func (s *Service) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// IsReadOnly reports whether changing data is blocked.
func (s *Service) IsReadOnly() bool {
	return s.Status().State == ReadOnly
}

// Usable reports whether the product may be used at all (a license was
// activated at some point).
func (s *Service) Usable() bool {
	return s.Status().State != NotActivated
}

// HWID returns the fingerprint of the host, for the activation screen.
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

// Run checks the license at once and then every CheckInterval until ctx is done.
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

// stored is the license row of the database.
type stored struct {
	token        string
	activatedAt  *time.Time
	lastCheckOK  *time.Time
	graceStarted *time.Time
}

// verdict is what a check decides, before anything is written.
type verdict struct {
	state   State
	reason  string
	payload *Payload
	// graceStarted is when the grace period began, nil when the license is fine.
	graceStarted *time.Time
}

// evaluate decides the state from the stored license, the hardware of the
// host and the current time. It is the whole logic of a check and has no
// side effects.
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

	// The check failed: the grace period runs from the first failure. Its
	// length is a parameter of the signed license; a license that cannot be
	// read gives the default one.
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

// Check performs the full check — signature and hardware — records the
// outcome and returns the new status.
func (s *Service) Check() Status {
	now := s.now()
	lic, err := s.load()
	if err != nil {
		// Without a database nothing can be decided or recorded; keep the
		// last known status (not activated right after start).
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
		db.Exec(`UPDATE licenses SET last_check_ok_at = $1, grace_started_at = NULL WHERE license_blob = $2`, now, lic.token)
		status.LastCheckOK = &now
		if previous == Grace || previous == ReadOnly || lic.graceStarted != nil {
			s.logEvent(EventRestored, "")
		}
	case Grace, ReadOnly:
		if lic.graceStarted == nil {
			db.Exec(`UPDATE licenses SET grace_started_at = $1 WHERE license_blob = $2`, *v.graceStarted, lic.token)
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

// Activate stores a new license token after checking its signature and that
// it was issued for this hardware.
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

	// One active license: the new one replaces the old one.
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

// Event is a row of the license journal.
type Event struct {
	At      time.Time `json:"at"`
	Event   string    `json:"event"`
	Details string    `json:"details"`
}

// Events returns the latest rows of the license journal, newest first.
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

// logEvent writes to the license journal. It is kept apart from the audit
// of user actions: support reads it to see what happened to a license.
func (s *Service) logEvent(event, details string) {
	if s.db == nil || *s.db == nil {
		return
	}
	if _, err := (*s.db).Exec(`INSERT INTO license_events (event, details) VALUES ($1, $2)`, event, details); err != nil {
		slog.Warn("License journal: cannot write", "event", event, "error", err)
	}
}
