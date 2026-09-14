package middleware

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type LicenseChecker struct {
	DB *sql.DB

	mu            sync.RWMutex
	isReadOnly    bool
	lastCheck     time.Time
	checkInterval time.Duration
}

func NewLicenseChecker(db *sql.DB) *LicenseChecker {
	lc := &LicenseChecker{
		DB:            db,
		checkInterval: 1 * time.Hour,
	}
	lc.check()
	return lc
}

func (lc *LicenseChecker) check() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	var blob string
	var graceStarted *time.Time

	err := lc.DB.QueryRow(`SELECT license_blob, grace_started_at FROM licenses ORDER BY id DESC LIMIT 1`).Scan(&blob, &graceStarted)
	if err != nil {
		// No license — not read-only (let activation screen work)
		lc.isReadOnly = false
		lc.lastCheck = time.Now()
		return
	}

	// Read gracePeriodDays from the signed payload
	gracePeriodDays := readGracePeriodFromBlob(blob)
	if gracePeriodDays <= 0 {
		gracePeriodDays = 14
	}

	if graceStarted != nil {
		graceEnd := graceStarted.Add(time.Duration(gracePeriodDays) * 24 * time.Hour)
		lc.isReadOnly = time.Now().After(graceEnd)
	} else {
		lc.isReadOnly = false
	}

	lc.lastCheck = time.Now()
}

func (lc *LicenseChecker) IsReadOnly() bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	// Periodic re-check
	if time.Since(lc.lastCheck) > lc.checkInterval {
		go lc.check()
	}
	return lc.isReadOnly
}

// ReadOnlyMiddleware blocks POST/PUT/PATCH/DELETE when license is in read-only mode
func (lc *LicenseChecker) ReadOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow license endpoints always
		if strings.HasPrefix(r.URL.Path, "/api/license") {
			next.ServeHTTP(w, r)
			return
		}

		// Allow GET/OPTIONS/HEAD always
		if r.Method == "GET" || r.Method == "OPTIONS" || r.Method == "HEAD" {
			next.ServeHTTP(w, r)
			return
		}

		// Block writes in read-only mode
		if lc.IsReadOnly() {
			http.Error(w, `{"error":"READ_ONLY_MODE"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// readGracePeriodFromBlob decodes the license blob and reads grace_period_days
func readGracePeriodFromBlob(blob string) int {
	parts := strings.SplitN(blob, ".", 2)
	if len(parts) != 2 {
		return 14
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 14
	}
	var payload struct {
		GracePeriodDays int `json:"grace_period_days"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return 14
	}
	if payload.GracePeriodDays <= 0 {
		return 14
	}
	return payload.GracePeriodDays
}
