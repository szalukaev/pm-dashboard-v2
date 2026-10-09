package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/middleware"
	"pm-dashboard/secrets"
	"pm-dashboard/utils"
)

// A dashboard user is tied to their account in the data source by a
// personal API key: the key identifies its owner there. Only the user can
// set or remove the link, and only their own — there is no admin route.
//
// This is how the Redmine connector identifies a person; another connector
// may do it differently, so the client is told the kind of the source and
// shows the matching form.
//
// The key is checked in the source (who does it belong to?) and stored
// encrypted, so that changes the user makes go to the source under their
// own name. It is never sent back to the client: only its last characters
// are shown, to recognise which key is in use.

// UserKeys reads the personal API keys of users.
type UserKeys struct {
	DB     **sql.DB
	Source *manager.Manager
	Box    *secrets.Box
}

// ClientFor returns the client of the data source to change data with on
// behalf of the user: with their personal key when they have one, otherwise
// the system client. nil when the source is not configured.
func (k *UserKeys) ClientFor(userID int) *redmine.Client {
	if k == nil || k.Source == nil {
		return nil
	}
	system := k.Source.Client()
	if system == nil || k.Box == nil {
		return system
	}
	var encrypted string
	if err := (*k.DB).QueryRow("SELECT source_token_enc FROM users WHERE id = $1", userID).Scan(&encrypted); err != nil || encrypted == "" {
		return system
	}
	token, err := k.Box.Decrypt(encrypted)
	if err != nil {
		slog.Warn("Stored personal API key cannot be read, using the system key", "user", userID, "error", err)
		return system
	}
	if personal := k.Source.ClientWithKey(token); personal != nil {
		return personal
	}
	return system
}

// sourceInfo describes the link for /auth/me and the login answer.
func (h *AuthHandler) sourceInfo(userID int) map[string]interface{} {
	info := map[string]interface{}{
		"type":        "",
		"linked":      false,
		"member_id":   nil,
		"member_name": "",
		"token_hint":  "",
		"needs_token": false,
	}
	if h.Source != nil {
		info["type"] = h.Source.SourceType()
	}

	var memberID *int
	var memberName, hint string
	err := (*h.DB).QueryRow(`SELECT u.member_id, COALESCE(m.name, ''), u.source_token_hint
		FROM users u LEFT JOIN members m ON m.external_id = u.member_id AND m.data_source = 'redmine'
		WHERE u.id = $1`, userID).Scan(&memberID, &memberName, &hint)
	if err != nil {
		return info
	}
	if memberID != nil {
		info["linked"] = true
		info["member_id"] = *memberID
		info["member_name"] = memberName
		info["token_hint"] = hint
	}
	// Only the Redmine connector asks for a personal key.
	info["needs_token"] = info["type"] == "redmine" && memberID == nil
	return info
}

// tokenHint keeps the last characters of a key, enough to recognise it.
func tokenHint(token string) string {
	const keep = 4
	if len(token) <= keep {
		return ""
	}
	return token[len(token)-keep:]
}

// SetSourceToken ties the current user to the owner of the given personal
// API key of the data source.
func (h *AuthHandler) SetSourceToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	token := strings.TrimSpace(body.Token)
	if token == "" {
		utils.Error(w, http.StatusBadRequest, "TOKEN_REQUIRED")
		return
	}
	if h.Source == nil || h.Source.SourceType() != "redmine" {
		utils.Error(w, http.StatusBadRequest, "DATA_SOURCE_NOT_CONFIGURED")
		return
	}

	member, err := h.Source.ClientWithKey(token).GetCurrentUser()
	if errors.Is(err, redmine.ErrUnauthorized) {
		utils.Error(w, http.StatusBadRequest, "TOKEN_INVALID")
		return
	}
	if err != nil {
		slog.Warn("Could not check a personal API key in the data source", "user", userID, "error", err)
		utils.Error(w, http.StatusBadGateway, "DATA_SOURCE_UNAVAILABLE")
		return
	}

	// One account of the source — one dashboard user.
	var taken bool
	(*h.DB).QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE member_id = $1 AND id <> $2)", member.ExternalID, userID).Scan(&taken)
	if taken {
		slog.Warn("Personal API key of an account already linked to another user", "user", userID, "member", member.ExternalID)
		utils.Error(w, http.StatusConflict, "TOKEN_ALREADY_LINKED")
		return
	}

	if h.Secrets == nil {
		utils.Error(w, http.StatusInternalServerError, "SECRET_STORE_UNAVAILABLE")
		return
	}
	encrypted, err := h.Secrets.Encrypt(token)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "SECRET_STORE_UNAVAILABLE")
		return
	}
	if _, err := (*h.DB).Exec("UPDATE users SET member_id = $1, source_token_enc = $2, source_token_hint = $3 WHERE id = $4",
		member.ExternalID, encrypted, tokenHint(token), userID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	// The member may not be synced yet (not in a tracked project): keep the
	// name the source has just given.
	(*h.DB).Exec(`INSERT INTO members (external_id, name, login, data_source, synced_at)
		VALUES ($1, $2, $3, 'redmine', NOW())
		ON CONFLICT (external_id, data_source) DO NOTHING`, member.ExternalID, member.Name, member.Login)

	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "source": h.sourceInfo(userID)})
}

// ClearSourceToken removes the link of the current user.
func (h *AuthHandler) ClearSourceToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if _, err := (*h.DB).Exec("UPDATE users SET member_id = NULL, source_token_enc = '', source_token_hint = '' WHERE id = $1", userID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "source": h.sourceInfo(userID)})
}
