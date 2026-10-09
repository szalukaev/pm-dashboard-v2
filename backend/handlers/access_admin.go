package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"pm-dashboard/access"
	"pm-dashboard/datasource/manager"
	"pm-dashboard/db"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// AccessHandler is the administrator's side of access control: users, their
// rights, groups and role templates. All routes are admin-only.
type AccessHandler struct {
	DB     **sql.DB
	Source *manager.Manager
	// Sessions ends the sessions of a user whose password was reset.
	Sessions db.SessionStore
}

func (h *AccessHandler) resolver() *access.Resolver {
	return &access.Resolver{DB: h.DB}
}

// rightsChanged is called after any change of rights: the set of projects
// to sync may have changed with them.
func (h *AccessHandler) rightsChanged() {
	if h.Source != nil {
		h.Source.TriggerSync()
	}
}

func pathID(r *http.Request, name string) (int, bool) {
	id, err := strconv.Atoi(mux.Vars(r)[name])
	return id, err == nil && id > 0
}

// decodePermissions reads a permission set from the request body.
func decodePermissions(r *http.Request) (access.Permissions, bool) {
	var p access.Permissions
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		return p, false
	}
	return p.Normalized(), true
}

// ─── Users ───

type adminUser struct {
	ID             int      `json:"id"`
	Username       string   `json:"username"`
	DisplayName    string   `json:"display_name"`
	Role           string   `json:"role"`
	CreatedAt      string   `json:"created_at"`
	LastLogin      *string  `json:"last_login"`
	IsBlocked      bool     `json:"is_blocked"`
	MemberID       *int     `json:"member_id"`
	MemberName     string   `json:"member_name"`
	RoleTemplateID *int     `json:"role_template_id"`
	TemplateName   string   `json:"template_name"`
	// Custom: rights are set individually and override the template.
	Custom bool     `json:"custom"`
	Groups []string `json:"groups"`
}

func (h *AccessHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := (*h.DB).Query(`SELECT u.id, u.username, COALESCE(u.display_name, ''), u.role, u.created_at, u.last_login,
			u.is_blocked, u.member_id, COALESCE(m.name, ''), u.role_template_id, COALESCE(t.name, ''),
			EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id),
			COALESCE((SELECT array_agg(g.name ORDER BY g.name) FROM group_members gm
				JOIN groups g ON g.id = gm.group_id WHERE gm.user_id = u.id), '{}')
		FROM users u
		LEFT JOIN members m ON m.external_id = u.member_id AND m.data_source = 'redmine'
		LEFT JOIN role_templates t ON t.id = u.role_template_id
		ORDER BY u.id`)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	users := []adminUser{}
	for rows.Next() {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.CreatedAt, &u.LastLogin,
			&u.IsBlocked, &u.MemberID, &u.MemberName, &u.RoleTemplateID, &u.TemplateName,
			&u.Custom, pq.Array(&u.Groups)); err != nil {
			utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
			return
		}
		users = append(users, u)
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"users": users})
}

func (h *AccessHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username       string `json:"username"`
		Password       string `json:"password"`
		DisplayName    string `json:"display_name"`
		Role           string `json:"role"`
		RoleTemplateID *int   `json:"role_template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if len(body.Username) < 3 || len(body.Password) < 6 {
		utils.Error(w, http.StatusBadRequest, "INVALID_INPUT")
		return
	}
	if body.Role == "" {
		body.Role = "user"
	}
	if !isValidRole(body.Role) {
		utils.Error(w, http.StatusBadRequest, "INVALID_ROLE")
		return
	}
	// A template that grants administration decides the role.
	if body.RoleTemplateID != nil {
		var grantsAdmin bool
		if err := (*h.DB).QueryRow("SELECT grants_admin FROM role_templates WHERE id = $1", *body.RoleTemplateID).Scan(&grantsAdmin); err != nil {
			utils.Error(w, http.StatusBadRequest, "TEMPLATE_NOT_FOUND")
			return
		}
		if grantsAdmin {
			body.Role = "admin"
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "HASH_FAILED")
		return
	}
	displayName := strings.TrimSpace(body.DisplayName)
	if displayName == "" {
		displayName = body.Username
	}
	// The password is set by the administrator, so it is temporary. Who the
	// user is in the data source they tell themselves on the first login
	// (a personal API key, see source_link.go).
	_, err = (*h.DB).Exec(`INSERT INTO users (username, password_hash, role, display_name, role_template_id, force_password_change)
		VALUES ($1, $2, $3, $4, $5, true)`,
		body.Username, string(hash), body.Role, displayName, body.RoleTemplateID)
	if err != nil {
		utils.Error(w, http.StatusConflict, "USER_EXISTS")
		return
	}
	h.rightsChanged()
	utils.Success(w)
}

// UpdateUser changes the fields present in the body: role, display_name,
// is_blocked, role_template_id (may be null).
func (h *AccessHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	self := userID == middleware.GetUserID(r)

	var exists bool
	(*h.DB).QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if !exists {
		utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}

	sets := []string{}
	args := []interface{}{}
	set := func(column string, value interface{}) {
		args = append(args, value)
		sets = append(sets, column+" = $"+strconv.Itoa(len(args)))
	}
	nullableInt := func(raw json.RawMessage) (*int, bool) {
		var v *int
		return v, json.Unmarshal(raw, &v) == nil
	}

	if raw, ok := body["role"]; ok {
		var role string
		if json.Unmarshal(raw, &role) != nil || !isValidRole(role) {
			utils.Error(w, http.StatusBadRequest, "INVALID_ROLE")
			return
		}
		// An administrator cannot demote themselves — protects against losing the last admin.
		if self && role != "admin" {
			utils.Error(w, http.StatusBadRequest, "CANNOT_DEMOTE_SELF")
			return
		}
		set("role", role)
	}
	if raw, ok := body["display_name"]; ok {
		var name string
		if json.Unmarshal(raw, &name) != nil {
			utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		set("display_name", strings.TrimSpace(name))
	}
	if raw, ok := body["is_blocked"]; ok {
		var blocked bool
		if json.Unmarshal(raw, &blocked) != nil {
			utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		if self && blocked {
			utils.Error(w, http.StatusBadRequest, "CANNOT_BLOCK_SELF")
			return
		}
		set("is_blocked", blocked)
	}
	// member_id is not here on purpose: the link to the account in the data
	// source is set only by the user themselves, with a personal API key.
	if raw, ok := body["role_template_id"]; ok {
		templateID, valid := nullableInt(raw)
		if !valid {
			utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		if templateID != nil {
			var grantsAdmin bool
			if err := (*h.DB).QueryRow("SELECT grants_admin FROM role_templates WHERE id = $1", *templateID).Scan(&grantsAdmin); err != nil {
				utils.Error(w, http.StatusBadRequest, "TEMPLATE_NOT_FOUND")
				return
			}
			if grantsAdmin {
				set("role", "admin")
			}
		}
		set("role_template_id", templateID)
		// Choosing a template applies its rights: an individual override,
		// if there was one, is dropped.
		if _, err := (*h.DB).Exec("DELETE FROM user_permissions WHERE user_id = $1", userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	if len(sets) > 0 {
		args = append(args, userID)
		if _, err := (*h.DB).Exec("UPDATE users SET "+strings.Join(sets, ", ")+" WHERE id = $"+strconv.Itoa(len(args)), args...); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	h.rightsChanged()
	utils.Success(w)
}

func (h *AccessHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	if userID == middleware.GetUserID(r) {
		utils.Error(w, http.StatusBadRequest, "CANNOT_DELETE_SELF")
		return
	}
	res, err := (*h.DB).Exec("DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}
	h.rightsChanged()
	utils.Success(w)
}

// temporaryPassword returns a random password that is easy to read aloud
// and type: no look-alike characters.
func temporaryPassword() (string, error) {
	const alphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const length = 12
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

// ResetPassword gives the user a random temporary password and returns it
// once: the administrator passes it on outside of the system, and the user
// must replace it on the next login.
func (h *AccessHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	password, err := temporaryPassword()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "PASSWORD_GENERATION_FAILED")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "HASH_FAILED")
		return
	}
	res, err := (*h.DB).Exec("UPDATE users SET password_hash = $1, force_password_change = true WHERE id = $2", string(hash), userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}
	// The old password no longer works, and neither must the sessions
	// opened with it: the user is logged out on every device.
	if h.Sessions != nil {
		if _, err := h.Sessions.DeleteUser(r.Context(), userID); err != nil {
			slog.Warn("Could not end the sessions after a password reset", "user", userID, "error", err)
		}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"password": password})
}

// ─── Rights of a user ───

// GetUserPermissions returns the rights of a user by source — individual,
// role template, groups — and what they add up to.
func (h *AccessHandler) GetUserPermissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	grants, err := h.resolver().Grants(userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, grants)
}

// SetUserPermissions sets the rights of a user individually.
func (h *AccessHandler) SetUserPermissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	perms, ok := decodePermissions(r)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if err := access.Save(*h.DB, "user_permissions", "user_id", userID, perms); err != nil {
		utils.Error(w, http.StatusBadRequest, "USER_NOT_FOUND")
		return
	}
	h.rightsChanged()
	utils.Success(w)
}

// ClearUserPermissions drops the individual rights: the role template (or
// the default, own tasks only) applies again.
func (h *AccessHandler) ClearUserPermissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	if _, err := (*h.DB).Exec("DELETE FROM user_permissions WHERE user_id = $1", userID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	h.rightsChanged()
	utils.Success(w)
}

// ─── Cloning ───

var errNotFound = errors.New("not found")

// sourcePermissions returns the rights to copy: of a user — everything they
// have in sum, of a group or a template — its own set.
func (h *AccessHandler) sourcePermissions(kind string, id int) (access.Permissions, error) {
	switch kind {
	case "user":
		grants, err := h.resolver().Grants(id)
		if err != nil {
			return access.Permissions{}, err
		}
		return grants.Effective, nil
	case "group":
		return access.Scan((*h.DB).QueryRow("SELECT "+access.Columns+" FROM group_permissions WHERE group_id = $1", id).Scan)
	case "template":
		return access.Scan((*h.DB).QueryRow("SELECT "+access.Columns+" FROM role_templates WHERE id = $1", id).Scan)
	}
	return access.Permissions{}, errNotFound
}

// ClonePermissions copies rights from a user or a group to another user or
// group. Tabs and widgets are copied only when asked to.
func (h *AccessHandler) ClonePermissions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FromType       string `json:"from_type"` // user | group
		FromID         int    `json:"from_id"`
		ToType         string `json:"to_type"` // user | group
		ToID           int    `json:"to_id"`
		IncludeTabs    bool   `json:"include_tabs"`
		IncludeWidgets bool   `json:"include_widgets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if (body.FromType != "user" && body.FromType != "group") || (body.ToType != "user" && body.ToType != "group") ||
		(body.FromType == body.ToType && body.FromID == body.ToID) {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	perms, err := h.sourcePermissions(body.FromType, body.FromID)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "SOURCE_NOT_FOUND")
		return
	}

	table, key := "user_permissions", "user_id"
	if body.ToType == "group" {
		table, key = "group_permissions", "group_id"
	}
	// What is not copied stays as the target has it.
	if !body.IncludeTabs || !body.IncludeWidgets {
		current, err := access.Scan((*h.DB).QueryRow("SELECT "+access.Columns+" FROM "+table+" WHERE "+key+" = $1", body.ToID).Scan)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
			return
		}
		if !body.IncludeTabs {
			perms.VisibleTabs = current.VisibleTabs
		}
		if !body.IncludeWidgets {
			perms.Widgets = current.Widgets
		}
	}
	if err := access.Save(*h.DB, table, key, body.ToID, perms); err != nil {
		utils.Error(w, http.StatusNotFound, "TARGET_NOT_FOUND")
		return
	}
	h.rightsChanged()
	utils.Success(w)
}

// ─── Groups ───

type adminGroup struct {
	ID          int                `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	MemberIDs   []int64            `json:"member_ids"`
	Permissions access.Permissions `json:"permissions"`
}

func (h *AccessHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	rows, err := (*h.DB).Query(`SELECT g.id, g.name, g.description,
			COALESCE((SELECT array_agg(gm.user_id ORDER BY gm.user_id) FROM group_members gm WHERE gm.group_id = g.id), '{}'),
			` + prefixedPermissionColumns("p") + `
		FROM groups g JOIN group_permissions p ON p.group_id = g.id ORDER BY g.name`)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	groups := []adminGroup{}
	for rows.Next() {
		var g adminGroup
		perms, err := access.Scan(rows.Scan, &g.ID, &g.Name, &g.Description, pq.Array(&g.MemberIDs))
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
			return
		}
		g.Permissions = perms
		groups = append(groups, g)
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"groups": groups})
}

// prefixedPermissionColumns returns access.Columns with a table alias.
func prefixedPermissionColumns(alias string) string {
	parts := strings.Split(access.Columns, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + p
	}
	return strings.Join(parts, ", ")
}

func (h *AccessHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_INPUT")
		return
	}
	tx, err := (*h.DB).Begin()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	defer tx.Rollback()

	var id int
	if err := tx.QueryRow("INSERT INTO groups (name, description) VALUES ($1, $2) RETURNING id",
		strings.TrimSpace(body.Name), strings.TrimSpace(body.Description)).Scan(&id); err != nil {
		utils.Error(w, http.StatusConflict, "GROUP_EXISTS")
		return
	}
	// A group always has a permission row; a new group grants nothing and
	// hides nothing by itself.
	if _, err := tx.Exec("INSERT INTO group_permissions (group_id) VALUES ($1)", id); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "id": id})
}

// UpdateGroup changes what is present in the body: name, description,
// member_ids (the full list), permissions.
func (h *AccessHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body struct {
		Name        *string             `json:"name"`
		Description *string             `json:"description"`
		MemberIDs   *[]int              `json:"member_ids"`
		Permissions *access.Permissions `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	var exists bool
	(*h.DB).QueryRow("SELECT EXISTS(SELECT 1 FROM groups WHERE id = $1)", groupID).Scan(&exists)
	if !exists {
		utils.Error(w, http.StatusNotFound, "GROUP_NOT_FOUND")
		return
	}

	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			utils.Error(w, http.StatusBadRequest, "INVALID_INPUT")
			return
		}
		if _, err := (*h.DB).Exec("UPDATE groups SET name = $1 WHERE id = $2", name, groupID); err != nil {
			utils.Error(w, http.StatusConflict, "GROUP_EXISTS")
			return
		}
	}
	if body.Description != nil {
		if _, err := (*h.DB).Exec("UPDATE groups SET description = $1 WHERE id = $2", strings.TrimSpace(*body.Description), groupID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if body.MemberIDs != nil {
		tx, err := (*h.DB).Begin()
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
		defer tx.Rollback()
		if _, err := tx.Exec("DELETE FROM group_members WHERE group_id = $1", groupID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
		// Unknown user ids are skipped by the join.
		if _, err := tx.Exec(`INSERT INTO group_members (group_id, user_id)
			SELECT $1, id FROM users WHERE id = ANY($2)`, groupID, pq.Array(*body.MemberIDs)); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
		if err := tx.Commit(); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if body.Permissions != nil {
		if err := access.Save(*h.DB, "group_permissions", "group_id", groupID, body.Permissions.Normalized()); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	h.rightsChanged()
	utils.Success(w)
}

func (h *AccessHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	res, err := (*h.DB).Exec("DELETE FROM groups WHERE id = $1", groupID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "GROUP_NOT_FOUND")
		return
	}
	h.rightsChanged()
	utils.Success(w)
}

// ─── Role templates ───

type adminTemplate struct {
	ID          int                `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	IsSystem    bool               `json:"is_system"`
	GrantsAdmin bool               `json:"grants_admin"`
	UserCount   int                `json:"user_count"`
	Permissions access.Permissions `json:"permissions"`
}

func (h *AccessHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := (*h.DB).Query(`SELECT t.id, t.name, t.description, t.is_system, t.grants_admin,
			(SELECT COUNT(*) FROM users u WHERE u.role_template_id = t.id),
			` + prefixedPermissionColumns("t") + `
		FROM role_templates t ORDER BY t.is_system DESC, t.id`)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	templates := []adminTemplate{}
	for rows.Next() {
		var t adminTemplate
		perms, err := access.Scan(rows.Scan, &t.ID, &t.Name, &t.Description, &t.IsSystem, &t.GrantsAdmin, &t.UserCount)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
			return
		}
		t.Permissions = perms
		templates = append(templates, t)
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"templates": templates})
}

// CreateTemplate creates a template with the given rights, or with the
// rights copied from a user (from_user_id) or another template
// (from_template_id).
func (h *AccessHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name           string              `json:"name"`
		Description    string              `json:"description"`
		Permissions    *access.Permissions `json:"permissions"`
		FromUserID     *int                `json:"from_user_id"`
		FromTemplateID *int                `json:"from_template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_INPUT")
		return
	}

	var perms access.Permissions
	var err error
	switch {
	case body.FromUserID != nil:
		perms, err = h.sourcePermissions("user", *body.FromUserID)
	case body.FromTemplateID != nil:
		perms, err = h.sourcePermissions("template", *body.FromTemplateID)
	case body.Permissions != nil:
		perms = body.Permissions.Normalized()
	}
	if err != nil {
		utils.Error(w, http.StatusNotFound, "SOURCE_NOT_FOUND")
		return
	}

	var id int
	if err := (*h.DB).QueryRow("INSERT INTO role_templates (name, description) VALUES ($1, $2) RETURNING id",
		strings.TrimSpace(body.Name), strings.TrimSpace(body.Description)).Scan(&id); err != nil {
		utils.Error(w, http.StatusConflict, "TEMPLATE_EXISTS")
		return
	}
	if err := access.SaveTemplate(*h.DB, id, perms); err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "id": id})
}

// UpdateTemplate changes name, description and rights of a template. New
// rights apply at once to every user of the template whose rights are not
// set individually.
func (h *AccessHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	var body struct {
		Name        *string             `json:"name"`
		Description *string             `json:"description"`
		Permissions *access.Permissions `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	var exists bool
	(*h.DB).QueryRow("SELECT EXISTS(SELECT 1 FROM role_templates WHERE id = $1)", templateID).Scan(&exists)
	if !exists {
		utils.Error(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND")
		return
	}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			utils.Error(w, http.StatusBadRequest, "INVALID_INPUT")
			return
		}
		if _, err := (*h.DB).Exec("UPDATE role_templates SET name = $1 WHERE id = $2", name, templateID); err != nil {
			utils.Error(w, http.StatusConflict, "TEMPLATE_EXISTS")
			return
		}
	}
	if body.Description != nil {
		if _, err := (*h.DB).Exec("UPDATE role_templates SET description = $1 WHERE id = $2", strings.TrimSpace(*body.Description), templateID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	if body.Permissions != nil {
		if err := access.SaveTemplate(*h.DB, templateID, body.Permissions.Normalized()); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}
	h.rightsChanged()
	utils.Success(w)
}

// DeleteTemplate removes a template. Its users keep their rights: the
// template's set becomes their individual one ("custom").
func (h *AccessHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, ok := pathID(r, "id")
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	tx, err := (*h.DB).Begin()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO user_permissions (user_id, `+access.Columns+`)
		SELECT u.id, `+prefixedPermissionColumns("t")+`
		FROM users u JOIN role_templates t ON t.id = u.role_template_id
		WHERE t.id = $1
		ON CONFLICT (user_id) DO NOTHING`, templateID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	res, err := tx.Exec("DELETE FROM role_templates WHERE id = $1", templateID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND")
		return
	}
	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}
	utils.Success(w)
}
