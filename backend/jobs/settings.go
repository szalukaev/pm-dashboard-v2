package jobs

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Settings of the history, kept in the settings table of the database.
const (
	keyRetentionDays  = "data_retention_days"
	keyCleanupEnabled = "cleanup_enabled"
	keyCleanupTime    = "cleanup_time"

	DefaultRetentionDays = 90
	MinRetentionDays     = 7
	MaxRetentionDays     = 365
	DefaultCleanupTime   = "02:00"
)

// ErrNoDatabase: the settings cannot be read or saved without the database.
var ErrNoDatabase = errors.New("database is not available")

// Settings reads and saves how long the history is kept and when it is
// cleaned.
type Settings struct {
	DB **sql.DB
}

func (s *Settings) get(key string, dest interface{}) bool {
	if s.DB == nil || *s.DB == nil {
		return false
	}
	var raw []byte
	if (*s.DB).QueryRow(`SELECT value FROM settings WHERE key = $1`, key).Scan(&raw) != nil {
		return false
	}
	return json.Unmarshal(raw, dest) == nil
}

func (s *Settings) set(key string, value interface{}) error {
	if s.DB == nil || *s.DB == nil {
		return ErrNoDatabase
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = (*s.DB).Exec(`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, key, data)
	return err
}

// RetentionDays: for how many days the snapshots are kept.
func (s *Settings) RetentionDays() int {
	days := 0
	if !s.get(keyRetentionDays, &days) || !ValidRetention(days) {
		return DefaultRetentionDays
	}
	return days
}

// ValidRetention reports whether the number of days may be set.
func ValidRetention(days int) bool {
	return days >= MinRetentionDays && days <= MaxRetentionDays
}

func (s *Settings) SetRetentionDays(days int) error {
	if !ValidRetention(days) {
		return errors.New("retention out of range")
	}
	return s.set(keyRetentionDays, days)
}

// CleanupEnabled: whether old snapshots are deleted by the schedule.
func (s *Settings) CleanupEnabled() bool {
	enabled := true
	s.get(keyCleanupEnabled, &enabled)
	return enabled
}

func (s *Settings) SetCleanupEnabled(enabled bool) error {
	return s.set(keyCleanupEnabled, enabled)
}

// CleanupTime: the time of day ("HH:MM", server time) of the cleanup.
func (s *Settings) CleanupTime() string {
	value := ""
	if !s.get(keyCleanupTime, &value) || !ValidTimeOfDay(value) {
		return DefaultCleanupTime
	}
	return value
}

func (s *Settings) SetCleanupTime(timeOfDay string) error {
	if !ValidTimeOfDay(timeOfDay) {
		return errors.New("time of day must be HH:MM")
	}
	return s.set(keyCleanupTime, timeOfDay)
}

// ValidTimeOfDay reports whether the value is a time of day as "HH:MM".
func ValidTimeOfDay(value string) bool {
	_, err := time.Parse("15:04", value)
	return err == nil
}
