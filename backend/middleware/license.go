package middleware

import (
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"time"
)

type LicenseChecker struct {
	DB *sql.DB

	mu             sync.RWMutex
	isReadOnly     bool
	lastCheck      time.Time
	checkInterval  time.Duration
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

	var graceStarted *time.Time
	var gracePeriodDays int = 14

	err := lc.DB.QueryRow(`SELECT grace_started_at FROM licenses ORDER BY id DESC LIMIT 1`).Scan(&graceStarted)
	if err != nil {
		// No license — not read-only (let activation screen work)
		lc.isReadOnly = false
		lc.lastCheck = time.Now()
		return
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
