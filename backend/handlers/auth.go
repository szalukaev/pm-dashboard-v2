package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"pm-dashboard/db"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB **sql.DB
	Sessions db.SessionStore
	Audit    *middleware.AuditMiddleware
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// dummyPasswordHash is a precomputed bcrypt hash used when the username does
// not exist, so the response time does not reveal whether the account is real
// (user-enumeration via timing).
var dummyPasswordHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

// Rate limiter: 5 failed attempts per 15 minutes per username,
// 20 failed attempts per 15 minutes per client IP.
type loginAttempt struct {
	Count       int
	FirstFail   time.Time
	LockedUntil time.Time
}

const (
	loginMaxPerUser = 5
	loginMaxPerIP   = 20
	loginWindow     = 15 * time.Minute
	loginLockTime   = 15 * time.Minute
)

var (
	loginAttempts  = make(map[string]*loginAttempt)
	loginIPAttempts = make(map[string]*loginAttempt)
	loginMu        sync.Mutex
)

// attemptLocked reports whether the entry is currently locked or its window expired.
// Caller must hold loginMu. Returns locked, expired.
func attemptLocked(a *loginAttempt) (bool, bool) {
	if a == nil {
		return false, false
	}
	if !a.LockedUntil.IsZero() {
		if time.Now().Before(a.LockedUntil) {
			return true, false
		}
		// lock expired
		return false, true
	}
	if time.Since(a.FirstFail) > loginWindow {
		return false, true
	}
	return false, false
}

func isLocked(username string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginAttempts[username]
	if !ok {
		return false
	}
	locked, expired := attemptLocked(a)
	if expired {
		delete(loginAttempts, username)
		return false
	}
	return locked
}

// isIPLocked blocks a client IP after too many failures from many usernames.
func isIPLocked(ip string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginIPAttempts[ip]
	if !ok {
		return false
	}
	locked, expired := attemptLocked(a)
	if expired {
		delete(loginIPAttempts, ip)
		return false
	}
	return locked
}

func recordFailure(username string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginAttempts[username]
	if !ok {
		loginAttempts[username] = &loginAttempt{Count: 1, FirstFail: time.Now()}
		return
	}
	if _, expired := attemptLocked(a); expired {
		loginAttempts[username] = &loginAttempt{Count: 1, FirstFail: time.Now()}
		return
	}
	a.Count++
	if a.Count >= loginMaxPerUser {
		a.LockedUntil = time.Now().Add(loginLockTime)
	}
}

// recordIPFailure counts a failed login against the client IP.
func recordIPFailure(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginIPAttempts[ip]
	if !ok {
		loginIPAttempts[ip] = &loginAttempt{Count: 1, FirstFail: time.Now()}
		return
	}
	if _, expired := attemptLocked(a); expired {
		loginIPAttempts[ip] = &loginAttempt{Count: 1, FirstFail: time.Now()}
		return
	}
	a.Count++
	if a.Count >= loginMaxPerIP {
		a.LockedUntil = time.Now().Add(loginLockTime)
	}
}

func clearAttempts(username string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginAttempts, username)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	if req.Username == "" || req.Password == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Check rate limit (username + client IP)
	clientIP := middleware.ClientIP(r)
	if isLocked(req.Username) || isIPLocked(clientIP) {
		utils.Error(w, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS")
		return
	}

	var id int
	var passwordHash, role, displayName, avatar string
	var lastLogin *string
	var forcePasswordChange bool
	err := (*h.DB).QueryRow("SELECT id, password_hash, role, COALESCE(display_name,''), COALESCE(avatar,''), COALESCE(force_password_change, false), last_login FROM users WHERE username = $1", req.Username).
		Scan(&id, &passwordHash, &role, &displayName, &avatar, &forcePasswordChange, &lastLogin)
	if err != nil {
		// Unknown user: still run bcrypt against a dummy hash so the timing
		// matches a real failed password check (blocks account enumeration).
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
		recordFailure(req.Username)
		recordIPFailure(clientIP)
		if h.Audit != nil {
			h.Audit.LogLogin(0, req.Username, false, r)
		}
		utils.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		recordFailure(req.Username)
		recordIPFailure(clientIP)
		if h.Audit != nil {
			h.Audit.LogLogin(0, req.Username, false, r)
		}
		utils.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS")
		return
	}

	// Successful login — clear attempts
	clearAttempts(req.Username)
	(*h.DB).Exec("UPDATE users SET last_login = NOW() WHERE id = $1", id)
	// Re-read last_login so response has fresh value
	(*h.DB).QueryRow("SELECT last_login FROM users WHERE id = $1", id).Scan(&lastLogin)
	if h.Audit != nil {
		h.Audit.LogLogin(id, req.Username, true, r)
	}

	sessionID := uuid.New().String()
	expiresAt := 7 * 24 * time.Hour
	if err := h.Sessions.Set(r.Context(), sessionID, id, expiresAt); err != nil {
		utils.Error(w, http.StatusInternalServerError, "SESSION_CREATE_FAILED")
		return
	}

	// SECURE_COOKIES=true в продакшене за HTTPS (кука не уйдёт по http).
	secureCookies := os.Getenv("SECURE_COOKIES") == "true"
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookies,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(expiresAt),
	})

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"id":                    id,
			"username":              req.Username,
			"display_name":          displayName,
			"role":                  role,
			"avatar":                avatar,
			"last_login":            lastLogin,
			"force_password_change": forcePasswordChange,
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	cookie, err := r.Cookie("session_id")
	if err == nil {
		h.Sessions.Delete(r.Context(), cookie.Value)
	}
	if h.Audit != nil {
		h.Audit.LogLogout(middleware.GetUserID(r), r)
	}
	secureCookies := os.Getenv("SECURE_COOKIES") == "true"
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookies,
		MaxAge:   -1,
	})
	utils.Success(w)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	userID := middleware.GetUserID(r)
	if userID == 0 {
		utils.Error(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var username, role, displayName, avatar string
	var lastLogin *string
	var forcePasswordChange bool
	err := (*h.DB).QueryRow("SELECT username, role, COALESCE(display_name,''), COALESCE(avatar,''), COALESCE(force_password_change, false), last_login FROM users WHERE id = $1", userID).Scan(&username, &role, &displayName, &avatar, &forcePasswordChange, &lastLogin)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"id":                    userID,
		"username":              username,
		"display_name":          displayName,
		"role":                  role,
		"avatar":                avatar,
		"last_login":            lastLogin,
		"force_password_change": forcePasswordChange,
	})
}

func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	userID := middleware.GetUserID(r)
	if userID == 0 {
		utils.Error(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var body struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	displayName := strings.TrimSpace(body.DisplayName)
	if displayName == "" {
		utils.Error(w, http.StatusBadRequest, "DISPLAY_NAME_REQUIRED")
		return
	}
	if len(displayName) > 255 {
		utils.Error(w, http.StatusBadRequest, "DISPLAY_NAME_TOO_LONG")
		return
	}

	(*h.DB).Exec("UPDATE users SET display_name = $1 WHERE id = $2", displayName, userID)

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"display_name": displayName,
	})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
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
	err := (*h.DB).QueryRow("SELECT password_hash FROM users WHERE id = $1", userID).Scan(&passwordHash)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}

	// If current_password is provided, verify it (skip if force_password_change)
	var forcePasswordChange bool
	(*h.DB).QueryRow("SELECT COALESCE(force_password_change, false) FROM users WHERE id = $1", userID).Scan(&forcePasswordChange)

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

	(*h.DB).Exec("UPDATE users SET password_hash = $1, force_password_change = false WHERE id = $2", string(newHash), userID)
	utils.Success(w)
}

func (h *AuthHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	userID := middleware.GetUserID(r)
	if userID == 0 {
		utils.Error(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	mime := http.DetectContentType(buf[:n])
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" && mime != "image/gif" {
		utils.Error(w, http.StatusBadRequest, "INVALID_IMAGE")
		return
	}

	data := buf[:n]
	tmp := make([]byte, 4096)
	for {
		n, err := file.Read(tmp)
		if n > 0 {
			data = append(data, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	if len(data) > 2<<20 {
		utils.Error(w, http.StatusBadRequest, "FILE_TOO_LARGE")
		return
	}

	avatar := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	(*h.DB).Exec("UPDATE users SET avatar = $1 WHERE id = $2", avatar, userID)
	utils.JSON(w, http.StatusOK, map[string]interface{}{"avatar": avatar})
}

func (h *AuthHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	userID := middleware.GetUserID(r)
	if userID == 0 {
		utils.Error(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	(*h.DB).Exec("UPDATE users SET avatar = '' WHERE id = $1", userID)
	utils.Success(w)
}
