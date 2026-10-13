package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/mail"
	"pm-dashboard/middleware"
	"pm-dashboard/secrets"
	"pm-dashboard/utils"
)

// Пользователь дашборда привязывается к своей учётной записи в источнике
// данных личным ключом API: ключ определяет там своего владельца. Задать
// или снять привязку может только сам пользователь и только свою —
// маршрута для администратора нет.
//
// Так человека определяет коннектор Redmine; другой коннектор может делать
// это иначе, поэтому клиенту сообщается вид источника, и он показывает
// подходящую форму.
//
// Ключ проверяется в источнике (чей он?) и хранится зашифрованным, чтобы
// изменения пользователя уходили в источник от его имени. Клиенту ключ
// обратно не отдаётся: показываются только последние символы — по ним
// видно, какой ключ используется.

// UserKeys читает личные ключи API пользователей.
type UserKeys struct {
	DB     **sql.DB
	Source *manager.Manager
	Box    *secrets.Box
}

// ClientFor возвращает клиента источника данных для изменения данных от
// имени пользователя: с личным ключом, если он есть, иначе системного.
// nil — источник не настроен.
func (k *UserKeys) ClientFor(userID int) *redmine.Client {
	if k == nil || k.Source == nil {
		return nil
	}
	if personal := k.personalClient(userID); personal != nil {
		return personal
	}
	return k.Source.Client()
}

// personalClient возвращает клиента, действующего с ключом самого
// пользователя; nil — ключ не введён (или источник не настроен).
func (k *UserKeys) personalClient(userID int) *redmine.Client {
	if k == nil || k.Source == nil || k.Box == nil || *k.DB == nil || k.Source.Client() == nil {
		return nil
	}
	var encrypted string
	if err := (*k.DB).QueryRow("SELECT source_token_enc FROM users WHERE id = $1", userID).Scan(&encrypted); err != nil || encrypted == "" {
		return nil
	}
	token, err := k.Box.Decrypt(encrypted)
	if err != nil {
		slog.Warn("Stored personal API key cannot be read", "user", userID, "error", err)
		return nil
	}
	return k.Source.ClientWithKey(token)
}

// RefreshProjects перечитывает проекты, в которых пользователь состоит в
// источнике данных («права как в Redmine», см. access.Permissions.FromSource).
func (k *UserKeys) RefreshProjects(userID int) {
	client := k.personalClient(userID)
	if client == nil {
		return
	}
	k.fillEmail(userID, client)
	projects, err := client.GetCurrentProjects()
	switch {
	case errors.Is(err, redmine.ErrUnauthorized):
		// Ключ отозван в источнике: вместе с ним уходят и права, которые он давал.
		slog.Warn("Personal API key is no longer accepted by the data source", "user", userID)
		projects = []int{}
	case err != nil:
		// Источник недоступен: остаётся то, что прочитано в прошлый раз.
		slog.Warn("Could not read the user's projects from the data source", "user", userID, "error", err)
		return
	}
	data, _ := json.Marshal(projects)
	if _, err := (*k.DB).Exec("UPDATE users SET source_projects = $1, source_projects_at = NOW() WHERE id = $2", data, userID); err != nil {
		slog.Warn("Could not save the user's projects from the data source", "user", userID, "error", err)
		return
	}
	// Сотрудники читаются по каждому проекту, и у пользователя со многими
	// проектами это занимает минуты: в фоне, по одному пользователю, чтобы
	// синхронизация, перед которой это запускается, не ждала.
	go func() {
		membersRefresh.Lock()
		defer membersRefresh.Unlock()
		k.refreshMembers(userID, client, projects)
	}()
}

// membersRefresh не даёт работать нескольким refreshMembers одновременно.
var membersRefresh sync.Mutex

// sourceMembersMaxAge — сколько времени можно доверять сотрудникам,
// прочитанным из источника. Они читаются по каждому проекту, и повторять
// это перед каждой синхронизацией слишком тяжело.
const sourceMembersMaxAge = time.Hour

// refreshMembers перечитывает, кого источник данных показывает
// пользователю: участников его проектов, запрошенных его же ключом. При
// «правах как в источнике» никто другой пользователю здесь не виден.
func (k *UserKeys) refreshMembers(userID int, client *redmine.Client, projects []int) {
	var fresh bool
	(*k.DB).QueryRow(`SELECT source_members IS NOT NULL AND source_members_at > NOW() - $1 * INTERVAL '1 second'
		FROM users WHERE id = $2`, int(sourceMembersMaxAge.Seconds()), userID).Scan(&fresh)
	if fresh {
		return
	}

	members := []int{}
	seen := map[int]bool{}
	// Себя пользователь видит всегда
	var own *int
	(*k.DB).QueryRow("SELECT member_id FROM users WHERE id = $1", userID).Scan(&own)
	if own != nil {
		seen[*own] = true
		members = append(members, *own)
	}
	for _, projectID := range projects {
		list, err := client.GetProjectMembers(projectID)
		switch {
		case errors.Is(err, redmine.ErrUnauthorized):
			return
		case err != nil:
			// Участников этого проекта источник пользователю не показывает:
			// проект никого не добавляет.
			slog.Debug("The user's key cannot read the members of a project", "user", userID, "project", projectID, "error", err)
			continue
		}
		for _, m := range list {
			if m.ExternalID != 0 && !seen[m.ExternalID] {
				seen[m.ExternalID] = true
				members = append(members, m.ExternalID)
			}
		}
	}
	data, _ := json.Marshal(members)
	if _, err := (*k.DB).Exec("UPDATE users SET source_members = $1, source_members_at = NOW() WHERE id = $2", data, userID); err != nil {
		slog.Warn("Could not save the members the data source shows to the user", "user", userID, "error", err)
	}
}

// emailTried — пользователи, чей адрес уже спрашивали у источника с момента
// запуска: если адреса там нет, незачем спрашивать перед каждой синхронизацией.
var emailTried sync.Map

// fillEmail берёт адрес электронной почты пользователя из источника, если в
// дашборде он не задан. Адрес, который пользователь указал сам, не трогает.
func (k *UserKeys) fillEmail(userID int, client *redmine.Client) {
	if _, tried := emailTried.LoadOrStore(userID, true); tried {
		return
	}
	var email string
	if (*k.DB).QueryRow("SELECT email FROM users WHERE id = $1", userID).Scan(&email) != nil || email != "" {
		return
	}
	address, err := client.GetCurrentUserMail()
	if err != nil {
		// Источник не ответил: спросим в следующий раз
		emailTried.Delete(userID)
		return
	}
	if mail.ValidAddress(address) {
		(*k.DB).Exec("UPDATE users SET email = $1 WHERE id = $2 AND email = ''", address, userID)
	}
}

// RefreshAllProjects выполняет RefreshProjects для каждого активного
// пользователя с ключом. Запускается перед каждой синхронизацией, поэтому
// смена участия в источнике доходит до дашборда за один интервал
// синхронизации.
func (k *UserKeys) RefreshAllProjects() {
	if k == nil || *k.DB == nil {
		return
	}
	rows, err := (*k.DB).Query("SELECT id FROM users WHERE source_token_enc <> '' AND NOT is_blocked")
	if err != nil {
		return
	}
	var ids []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		k.RefreshProjects(id)
	}
}

// sourceInfo описывает привязку для /auth/me и ответа на вход.
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
	// Личный ключ запрашивает только коннектор Redmine.
	info["needs_token"] = info["type"] == "redmine" && memberID == nil
	return info
}

// tokenHint оставляет последние символы ключа — их хватает, чтобы его узнать.
func tokenHint(token string) string {
	const keep = 4
	if len(token) <= keep {
		return ""
	}
	return token[len(token)-keep:]
}

// SetSourceToken привязывает текущего пользователя к владельцу заданного
// личного ключа API источника данных.
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

	// Одна учётная запись источника — один пользователь дашборда.
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
	// Сотрудник может быть ещё не синхронизирован (его нет в отслеживаемых
	// проектах): берётся имя, которое только что дал источник.
	(*h.DB).Exec(`INSERT INTO members (external_id, name, login, data_source, synced_at)
		VALUES ($1, $2, $3, 'redmine', NOW())
		ON CONFLICT (external_id, data_source) DO NOTHING`, member.ExternalID, member.Name, member.Login)

	// Проекты пользователя в источнике — для «прав как в Redmine»; затем
	// синхронизация забирает задачи проектов, ставших видимыми.
	if h.Keys != nil {
		// Новая связка — адрес почты спрашивается у источника заново
		emailTried.Delete(userID)
		h.Keys.RefreshProjects(userID)
	}
	if h.Source != nil {
		h.Source.TriggerSync()
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "source": h.sourceInfo(userID)})
}

// ClearSourceToken снимает привязку текущего пользователя.
func (h *AuthHandler) ClearSourceToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if _, err := (*h.DB).Exec(`UPDATE users SET member_id = NULL, source_token_enc = '', source_token_hint = '',
		source_projects = '[]', source_projects_at = NULL,
		source_members = NULL, source_members_at = NULL WHERE id = $1`, userID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "source": h.sourceInfo(userID)})
}
