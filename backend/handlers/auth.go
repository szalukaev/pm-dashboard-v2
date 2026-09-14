package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"pm-dashboard/db"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB       *sql.DB
	Sessions db.SessionStore
	Audit    *middleware.AuditMiddleware
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Rate limiter: 5 failed attempts per 15 minutes per username
type loginAttempt struct {
	Count     int
	FirstFail time.Time
	LockedUntil time.Time
}

var (
	loginAttempts = make(map[string]*loginAttempt)
	loginMu       sync.Mutex
)

func isLocked(username string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginAttempts[username]
	if !ok {
		return false
	}
	if !a.LockedUntil.IsZero() && time.Now().Before(a.LockedUntil) {
		return true
	}
	// Reset if lock expired
	if !a.LockedUntil.IsZero() && time.Now().After(a.LockedUntil) {
		delete(loginAttempts, username)
		return false
	}
	// Reset if window expired
	if time.Now().Sub(a.FirstFail) > 15*time.Minute {
		delete(loginAttempts, username)
		return false
	}
	return false
}

func recordFailure(username string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginAttempts[username]
	if !ok {
		loginAttempts[username] = &loginAttempt{Count: 1, FirstFail: time.Now()}
		return
	}
	// Reset if window expired
	if time.Now().Sub(a.FirstFail) > 15*time.Minute {
		loginAttempts[username] = &loginAttempt{Count: 1, FirstFail: time.Now()}
		return
	}
	a.Count++
	if a.Count >= 5 {
		a.LockedUntil = time.Now().Add(15 * time.Minute)
	}
}

func clearAttempts(username string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginAttempts, username)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	if req.Username == "" || req.Password == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Check rate limit
	if isLocked(req.Username) {
		utils.Error(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS")
		return
	}

	var id int
	var passwordHash, role string
	var forcePasswordChange bool
	err := h.DB.QueryRow("SELECT id, password_hash, role, COALESCE(force_password_change, false) FROM users WHERE username = $1", req.Username).
		Scan(&id, &passwordHash, &role, &forcePasswordChange)
	if err != nil {
		recordFailure(req.Username)
		if h.Audit != nil {
			h.Audit.LogLogin(0, req.Username, false, r)
		}
		utils.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		recordFailure(req.Username)
		if h.Audit != nil {
			h.Audit.LogLogin(0, req.Username, false, r)
		}
		utils.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS")
		return
	}

	// Successful login — clear attempts
	clearAttempts(req.Username)
	if h.Audit != nil {
		h.Audit.LogLogin(id, req.Username, true, r)
	}

	sessionID := uuid.New().String()
	expiresAt := 7 * 24 * time.Hour
	if err := h.Sessions.Set(r.Context(), sessionID, id, expiresAt); err != nil {
		utils.Error(w, http.StatusInternalServerError, "SESSION_CREATE_FAILED")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(expiresAt),
	})

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"id":                    id,
			"username":              req.Username,
			"role":                  role,
			"force_password_change": forcePasswordChange,
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		h.Sessions.Delete(r.Context(), cookie.Value)
	}
	if h.Audit != nil {
		h.Audit.LogLogout(middleware.GetUserID(r), r)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	utils.Success(w)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == 0 {
		utils.Error(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var username, role string
	var forcePasswordChange bool
	err := h.DB.QueryRow("SELECT username, role, COALESCE(force_password_change, false) FROM users WHERE id = $1", userID).Scan(&username, &role, &forcePasswordChange)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"id":                    userID,
		"username":              username,
		"role":                  role,
		"force_password_change": forcePasswordChange,
	})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == 0 {
		utils.Error(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if len(body.NewPassword) < 6 {
		utils.Error(w, http.StatusBadRequest, "PASSWORD_TOO_SHORT")
		return
	}

	var passwordHash string
	err := h.DB.QueryRow("SELECT password_hash FROM users WHERE id = $1", userID).Scan(&passwordHash)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}

	// If current_password is provided, verify it (skip if force_password_change)
	var forcePasswordChange bool
	h.DB.QueryRow("SELECT COALESCE(force_password_change, false) FROM users WHERE id = $1", userID).Scan(&forcePasswordChange)

	if !forcePasswordChange {
		if body.CurrentPassword == "" {
			utils.Error(w, http.StatusBadRequest, "CURRENT_PASSWORD_REQUIRED")
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(body.CurrentPassword)); err != nil {
			utils.Error(w, http.StatusUnauthorized, "INVALID_CURRENT_PASSWORD")
			return
		}
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "HASH_FAILED")
		return
	}

	h.DB.Exec("UPDATE users SET password_hash = $1, force_password_change = false WHERE id = $2", string(newHash), userID)
	utils.Success(w)
}
