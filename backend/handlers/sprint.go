package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"pm-dashboard/access"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
)

type SprintHandler struct {
	DB **sql.DB
}

// Статусы спринта. «Просрочен» — не статус, а открытый спринт, срок которого
// прошёл.
const (
	SprintOpen   = "open"
	SprintClosed = "closed"
)

type Sprint struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	ProjectName      *string `json:"project_name"`
	Status           string  `json:"status"`
	StartDate        *string `json:"start_date"`
	DueDate          *string `json:"due_date"`
	Description      *string `json:"description"`
	CategoryName     *string `json:"category_name"`
	AutoFillCategory bool    `json:"auto_fill_category"`
	// IsShared: общий спринт — виден команде создателя и забирает задачи из
	// бэклога; иначе личный, виден только создателю.
	IsShared bool `json:"is_shared"`
	// IsOwn: спринт создан самим пользователем; OwnerName — кем создан.
	IsOwn     bool   `json:"is_own"`
	OwnerName string `json:"owner_name"`
	// IsOverdue: спринт открыт, а дата окончания прошла.
	IsOverdue bool `json:"is_overdue"`
	// ClosedAt — когда закрыт; ClosedLate — закрыт позже даты окончания.
	ClosedAt     *string      `json:"closed_at"`
	ClosedLate   bool         `json:"closed_late"`
	Tasks        []SprintTask `json:"tasks"`
	TaskCount    int          `json:"task_count"`
	OpenCount    int          `json:"open_count"`
	TestingCount int          `json:"testing_count"`
	ClosedCount  int          `json:"closed_count"`
	Progress     float64      `json:"progress"`

	ownerID   int
	ownerTeam []int // выбранная команда создателя, пусто — вся команда
}

type SprintTask struct {
	ExternalID     int      `json:"external_id"`
	Subject        string   `json:"subject"`
	ProjectName    string   `json:"project_name"`
	StatusName     string   `json:"status_name"`
	PriorityName   string   `json:"priority_name"`
	PriorityID     int      `json:"priority_id"`
	AssignedToName string   `json:"assigned_to_name"`
	DueDate        *string  `json:"due_date"`
	EstimatedHours *float64 `json:"estimated_hours"`
	IsOverdue      bool     `json:"is_overdue"`
	// StatusGroup — группа статуса (open, testing, closed), см. status_groups.go
	StatusGroup string `json:"status_group"`

	statusGroup string
}

// visibleSprints возвращает спринты, видимые пользователю, без задач.
// sprintID > 0 оставляет один спринт.
func (h *SprintHandler) visibleSprints(userID, sprintID int) ([]Sprint, error) {
	var member *int
	if err := (*h.DB).QueryRow("SELECT member_id FROM users WHERE id = $1", userID).Scan(&member); err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	query := `SELECT s.id, s.name, s.project_name, s.status, s.start_date, s.due_date,
		s.description, s.category_name, s.auto_fill_category, s.is_shared, s.closed_at,
		COALESCE(s.user_id, 0), COALESCE(NULLIF(u.display_name, ''), u.username, ''), os.selected_team
		FROM sprints s
		LEFT JOIN users u ON u.id = s.user_id
		LEFT JOIN user_settings os ON os.user_id = s.user_id
		WHERE (s.user_id = $1 OR s.is_shared)`
	args := []interface{}{userID}
	if sprintID > 0 {
		query += " AND s.id = $2"
		args = append(args, sprintID)
	}

	rows, err := (*h.DB).Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Now().Format("2006-01-02")
	sprints := []Sprint{}
	for rows.Next() {
		var s Sprint
		var closedAt *time.Time
		var team []byte
		if rows.Scan(&s.ID, &s.Name, &s.ProjectName, &s.Status, &s.StartDate, &s.DueDate,
			&s.Description, &s.CategoryName, &s.AutoFillCategory, &s.IsShared, &closedAt,
			&s.ownerID, &s.OwnerName, &team) != nil {
			continue
		}
		_ = json.Unmarshal(team, &s.ownerTeam)
		// Кому виден спринт, решает access.SprintVisibleTo
		if !access.SprintVisibleTo(userID, member, s.ownerID, s.IsShared, s.ownerTeam) {
			continue
		}
		s.IsOwn = s.ownerID == userID
		s.setDates(closedAt, today)
		sprints = append(sprints, s)
	}
	return sprints, nil
}

// setDates считает признаки, зависящие от дат: просрочен ли открытый спринт
// и закрыт ли закрытый позже срока. today — сегодняшняя дата, ГГГГ-ММ-ДД.
func (s *Sprint) setDates(closedAt *time.Time, today string) {
	due := dateOnly(s.DueDate)
	s.IsOverdue = s.Status != SprintClosed && due != "" && due < today
	if closedAt != nil {
		closed := closedAt.In(time.Local)
		text := closed.Format(time.RFC3339)
		s.ClosedAt = &text
		s.ClosedLate = s.Status == SprintClosed && due != "" && closed.Format("2006-01-02") > due
	}
}

// dateOnly возвращает дату без времени (ГГГГ-ММ-ДД) или пустую строку.
func dateOnly(date *string) string {
	if date == nil {
		return ""
	}
	if len(*date) > 10 {
		return (*date)[:10]
	}
	return *date
}

// sortSprints ставит спринты в порядок экрана: просроченные, затем остальные
// открытые — и те и другие по дате окончания, без даты в конце; закрытые
// последними, недавно закрытые выше.
func sortSprints(sprints []Sprint) {
	rank := func(s Sprint) int {
		switch {
		case s.Status == SprintClosed:
			return 2
		case s.IsOverdue:
			return 0
		}
		return 1
	}
	sort.SliceStable(sprints, func(i, j int) bool {
		a, b := sprints[i], sprints[j]
		if rank(a) != rank(b) {
			return rank(a) < rank(b)
		}
		if a.Status == SprintClosed {
			if ca, cb := derefString(a.ClosedAt), derefString(b.ClosedAt); ca != cb {
				return ca > cb
			}
			return a.ID > b.ID
		}
		da, db := dateOnly(a.DueDate), dateOnly(b.DueDate)
		if da != db {
			if da == "" || db == "" {
				return db == ""
			}
			return da < db
		}
		return a.ID < b.ID
	})
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// findSprint возвращает спринт, если он виден пользователю; иначе отвечает
// 404 — чужой спринт снаружи неотличим от несуществующего.
func (h *SprintHandler) findSprint(w http.ResponseWriter, r *http.Request) (*Sprint, bool) {
	sprintID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return nil, false
	}
	sprints, err := h.visibleSprints(middleware.GetUserID(r), sprintID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return nil, false
	}
	if len(sprints) == 0 {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return nil, false
	}
	return &sprints[0], true
}

func (h *SprintHandler) ListSprints(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	sprints, err := h.visibleSprints(middleware.GetUserID(r), 0)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	scope := middleware.GetScope(r)
	for i := range sprints {
		sprints[i].loadTasks(h, scope)
	}
	sortSprints(sprints)

	utils.JSON(w, http.StatusOK, map[string]interface{}{"sprints": sprints})
}

func (h *SprintHandler) GetSprint(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	s, ok := h.findSprint(w, r)
	if !ok {
		return
	}
	s.loadTasks(h, middleware.GetScope(r))
	utils.JSON(w, http.StatusOK, s)
}

func (h *SprintHandler) CreateSprint(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	var body struct {
		Name             string  `json:"name"`
		ProjectName      *string `json:"project_name"`
		StartDate        *string `json:"start_date"`
		DueDate          *string `json:"due_date"`
		Description      *string `json:"description"`
		CategoryName     *string `json:"category_name"`
		AutoFillCategory bool    `json:"auto_fill_category"`
		IsShared         bool    `json:"is_shared"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		utils.Error(w, http.StatusBadRequest, "NAME_REQUIRED")
		return
	}
	body.StartDate = emptyToNil(body.StartDate)
	if body.StartDate == nil {
		utils.Error(w, http.StatusBadRequest, "START_DATE_REQUIRED")
		return
	}
	body.DueDate = emptyToNil(body.DueDate)
	if !sprintDatesInOrder(body.StartDate, body.DueDate) {
		utils.Error(w, http.StatusBadRequest, "DUE_BEFORE_START")
		return
	}
	body.ProjectName = emptyToNil(body.ProjectName)
	body.CategoryName = emptyToNil(body.CategoryName)
	body.Description = emptyToNil(body.Description)

	var sprintID int
	err := (*h.DB).QueryRow(`INSERT INTO sprints (user_id, name, project_name, start_date, due_date,
		description, category_name, auto_fill_category, is_shared)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		userID, body.Name, body.ProjectName, body.StartDate, body.DueDate,
		body.Description, body.CategoryName, body.AutoFillCategory, body.IsShared).Scan(&sprintID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "CREATE_FAILED")
		return
	}

	// Авто-заполнение, если заданы проект и категория
	if body.AutoFillCategory && body.ProjectName != nil && body.CategoryName != nil {
		h.autoFill(userID, sprintID, body.IsShared, *body.ProjectName, *body.CategoryName, middleware.GetScope(r))
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"id": sprintID, "success": true})
}

func (h *SprintHandler) UpdateSprint(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	current, ok := h.findSprint(w, r)
	if !ok {
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	allowed := map[string]bool{
		"name": true, "project_name": true, "status": true,
		"start_date": true, "due_date": true, "description": true,
		"category_name": true, "auto_fill_category": true, "is_shared": true,
	}

	// Смена статуса закрывает спринт или открывает его заново; всё остальное —
	// редактирование. Это разные права, и запросу могут понадобиться оба.
	scope := middleware.GetScope(r)
	for _, action := range sprintUpdateActions(body, allowed, current.Status) {
		if !scope.CanSprint(action) {
			slog.Warn("Sprint action outside of user rights", "user", userID, "action", action, "sprint", current.ID)
			utils.Error(w, http.StatusForbidden, "SPRINT_ACTION_FORBIDDEN")
			return
		}
	}

	if status, sent := body["status"]; sent && !validSprintStatus(status) {
		utils.Error(w, http.StatusBadRequest, "INVALID_STATUS")
		return
	}
	if shared, sent := body["is_shared"]; sent {
		value, isBool := shared.(bool)
		if !isBool {
			utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
			return
		}
		// Сделать общий спринт личным значит забрать его у остальных:
		// вид меняет только создатель.
		if value != current.IsShared && !current.IsOwn {
			utils.Error(w, http.StatusForbidden, "SPRINT_KIND_OWNER_ONLY")
			return
		}
	}

	// Форма присылает пустую строку для незаполненного поля: название и дата
	// начала обязательны, остальное сохраняется как NULL.
	for k, v := range body {
		text, isText := v.(string)
		blank := v == nil || (isText && strings.TrimSpace(text) == "")
		switch {
		case !blank || !allowed[k] || k == "is_shared" || k == "auto_fill_category":
		case k == "name":
			utils.Error(w, http.StatusBadRequest, "NAME_REQUIRED")
			return
		case k == "start_date":
			utils.Error(w, http.StatusBadRequest, "START_DATE_REQUIRED")
			return
		default:
			body[k] = nil
		}
	}

	if !sprintDatesInOrder(sprintUpdateDates(body, current)) {
		utils.Error(w, http.StatusBadRequest, "DUE_BEFORE_START")
		return
	}

	sets := []string{}
	args := []interface{}{}
	for k, v := range body {
		if allowed[k] {
			args = append(args, v)
			sets = append(sets, k+" = $"+strconv.Itoa(len(args)))
		}
	}
	if len(sets) == 0 {
		utils.Error(w, http.StatusBadRequest, "NO_FIELDS")
		return
	}
	// Дата закрытия идёт за статусом: ставится при закрытии, снимается при
	// повторном открытии
	if status, sent := body["status"]; sent && status != current.Status {
		if status == SprintClosed {
			sets = append(sets, "closed_at = NOW()")
		} else {
			sets = append(sets, "closed_at = NULL")
		}
	}

	args = append(args, current.ID)
	query := "UPDATE sprints SET " + utils.JoinStrings(sets, ",") + " WHERE id = $" + strconv.Itoa(len(args))
	if _, err := (*h.DB).Exec(query, args...); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}

	utils.Success(w)
}

// validSprintStatus сообщает, можно ли сохранить такое значение статуса.
// sprintDatesInOrder сообщает, что срок спринта не раньше его начала. Без
// одной из дат сравнивать не с чем.
func sprintDatesInOrder(start, due *string) bool {
	s, d := dateOnly(start), dateOnly(due)
	return s == "" || d == "" || d >= s
}

// sprintUpdateDates возвращает даты спринта, какими они станут после правки:
// форма может прислать только одну из них, вторая остаётся прежней.
func sprintUpdateDates(body map[string]interface{}, current *Sprint) (start, due *string) {
	start, due = current.StartDate, current.DueDate
	if v, sent := body["start_date"]; sent {
		start = nil
		if text, isText := v.(string); isText {
			start = &text
		}
	}
	if v, sent := body["due_date"]; sent {
		due = nil
		if text, isText := v.(string); isText {
			due = &text
		}
	}
	return start, due
}

func validSprintStatus(v interface{}) bool {
	status, isText := v.(string)
	return isText && (status == SprintOpen || status == SprintClosed)
}

// sprintUpdateActions возвращает права, нужные для изменения спринта: «close»,
// когда меняется статус, «edit» — когда прислано любое другое известное поле.
// Форма редактирования присылает статус вместе со всем остальным, поэтому
// статус, равный текущему, изменением не считается.
func sprintUpdateActions(body map[string]interface{}, known map[string]bool, currentStatus string) []string {
	var edit, status bool
	for field, value := range body {
		switch {
		case field == "status":
			status = value != currentStatus
		case known[field]:
			edit = true
		}
	}
	var actions []string
	if edit {
		actions = append(actions, access.SprintEdit)
	}
	if status {
		actions = append(actions, access.SprintClose)
	}
	return actions
}

func (h *SprintHandler) DeleteSprint(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	s, ok := h.findSprint(w, r)
	if !ok {
		return
	}
	if _, err := (*h.DB).Exec("DELETE FROM sprints WHERE id = $1", s.ID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "DELETE_FAILED")
		return
	}

	utils.Success(w)
}

// openSprintIDs возвращает незакрытые спринты одного вида, видимые
// пользователю: общие — все видимые, личные — только свои. Среди них задача
// может лежать лишь в одном.
func (h *SprintHandler) openSprintIDs(userID int, shared bool) ([]int, error) {
	sprints, err := h.visibleSprints(userID, 0)
	if err != nil {
		return nil, err
	}
	ids := []int{}
	for _, s := range sprints {
		if s.Status != SprintClosed && s.IsShared == shared {
			ids = append(ids, s.ID)
		}
	}
	return ids, nil
}

// AssignTask кладёт задачу в спринт. Задача лежит только в одном незакрытом
// спринте своего вида, поэтому из остальных таких спринтов она убирается;
// from_sprint_id — спринт, из которого задачу перетащили: из него она
// убирается, какого бы вида он ни был.
func (h *SprintHandler) AssignTask(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)
	target, ok := h.findSprint(w, r)
	if !ok {
		return
	}
	if target.Status == SprintClosed {
		utils.Error(w, http.StatusConflict, "SPRINT_CLOSED")
		return
	}

	var body struct {
		IssueExternalID int `json:"issue_external_id"`
		FromSprintID    int `json:"from_sprint_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	if !requireIssue(w, r, *h.DB, body.IssueExternalID) {
		return
	}

	others, err := h.openSprintIDs(userID, target.IsShared)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "ASSIGN_FAILED")
		return
	}
	if body.FromSprintID > 0 && body.FromSprintID != target.ID {
		// Спринт-источник должен быть виден пользователю
		if from, err := h.visibleSprints(userID, body.FromSprintID); err == nil && len(from) == 1 {
			others = append(others, body.FromSprintID)
		}
	}

	tx, err := (*h.DB).Begin()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "ASSIGN_FAILED")
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM sprint_issues WHERE issue_external_id = $1 AND sprint_id <> $2
		AND sprint_id = ANY($3)`, body.IssueExternalID, target.ID, pq.Array(others)); err != nil {
		utils.Error(w, http.StatusInternalServerError, "ASSIGN_FAILED")
		return
	}
	if _, err := tx.Exec(`INSERT INTO sprint_issues (sprint_id, issue_external_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, target.ID, body.IssueExternalID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "ASSIGN_FAILED")
		return
	}
	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "ASSIGN_FAILED")
		return
	}

	utils.Success(w)
}

func (h *SprintHandler) UnassignTask(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	issueID, err := strconv.Atoi(mux.Vars(r)["issueId"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	s, ok := h.findSprint(w, r)
	if !ok {
		return
	}

	res, err := (*h.DB).Exec("DELETE FROM sprint_issues WHERE sprint_id = $1 AND issue_external_id = $2", s.ID, issueID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "UNASSIGN_FAILED")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		utils.Error(w, http.StatusNotFound, "SPRINT_NOT_FOUND")
		return
	}

	utils.Success(w)
}

// GetBacklog возвращает задачи проектов пользователя (в пределах прав), не
// закрытые и не в тестировании, которых нет ни в одном незакрытом общем
// спринте, видимом пользователю. Личные спринты задач из бэклога не забирают.
func (h *SprintHandler) GetBacklog(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	shared, err := h.openSprintIDs(middleware.GetUserID(r), true)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}

	args := []interface{}{}
	where := []string{
		statusNotIn("i.status_id", GroupClosed, GroupTesting),
		middleware.GetScope(r).ProjectIssueCond("i.", &args),
	}

	args = append(args, pq.Array(shared))
	sharedParam := "$" + strconv.Itoa(len(args))
	query := `SELECT i.external_id, i.subject, COALESCE(i.project_name, ''), i.status_name,
		i.priority_name, i.priority_id, i.assigned_to_name, i.due_date, i.estimated_hours
		FROM issues i
		WHERE ` + utils.JoinStrings(where, " AND ") + `
		AND i.external_id NOT IN (SELECT issue_external_id FROM sprint_issues WHERE sprint_id = ANY(` + sharedParam + `))
		ORDER BY i.project_name, ` + priorityRank("i.priority_id") + ` DESC NULLS LAST`

	rows, err := (*h.DB).Query(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	// Группировка по проекту; задачи без проекта — группа с пустым названием,
	// подпись ей даёт интерфейс
	today := time.Now().Format("2006-01-02")
	groups := make(map[string][]SprintTask)
	for rows.Next() {
		var t SprintTask
		if rows.Scan(&t.ExternalID, &t.Subject, &t.ProjectName, &t.StatusName,
			&t.PriorityName, &t.PriorityID, &t.AssignedToName, &t.DueDate, &t.EstimatedHours) != nil {
			continue
		}
		t.StatusGroup = GroupOpen
		t.IsOverdue = dateOnly(t.DueDate) != "" && dateOnly(t.DueDate) < today
		groups[t.ProjectName] = append(groups[t.ProjectName], t)
	}

	type BacklogGroup struct {
		ProjectName string       `json:"project_name"`
		Tasks       []SprintTask `json:"tasks"`
		TaskCount   int          `json:"task_count"`
	}
	result := []BacklogGroup{}
	for name, tasks := range groups {
		result = append(result, BacklogGroup{ProjectName: name, Tasks: tasks, TaskCount: len(tasks)})
	}
	// У map нет порядка: без сортировки проекты перемешивались бы при каждом
	// обновлении. Задачи без проекта — в конце.
	sort.Slice(result, func(i, j int) bool {
		a, b := strings.ToLower(result[i].ProjectName), strings.ToLower(result[j].ProjectName)
		if (a == "") != (b == "") {
			return b == ""
		}
		return a < b
	})

	utils.JSON(w, http.StatusOK, map[string]interface{}{"backlog": result})
}

func (h *SprintHandler) RefreshSprint(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	s, ok := h.findSprint(w, r)
	if !ok {
		return
	}
	if s.Status != SprintClosed && s.AutoFillCategory && s.ProjectName != nil && s.CategoryName != nil {
		h.autoFill(middleware.GetUserID(r), s.ID, s.IsShared, *s.ProjectName, *s.CategoryName, middleware.GetScope(r))
	}

	utils.Success(w)
}

// emptyToNil превращает пустое значение в NULL: пустая дата — не дата, а
// пустой проект или категория значат «не задано».
func emptyToNil(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}

// loadTasks читает задачи спринта, которые пользователь по-прежнему вправе
// видеть (задача, положенная в спринт, остаётся в нём, но права могли сузить),
// и считает по ним счётчики и готовность.
func (s *Sprint) loadTasks(h *SprintHandler, scope *access.Scope) {
	s.Tasks = h.getSprintTasks(s.ID, scope)
	s.TaskCount = len(s.Tasks)
	s.calculateProgress()
}

func (h *SprintHandler) getSprintTasks(sprintID int, scope *access.Scope) []SprintTask {
	args := []interface{}{sprintID}
	visible := scope.AllowedIssueCond("i.", &args)
	rows, err := (*h.DB).Query(`SELECT i.external_id, i.subject, COALESCE(i.project_name, ''), i.status_name,
		i.priority_name, i.priority_id, i.assigned_to_name, i.due_date, i.estimated_hours,
		COALESCE(st.group_name, 'open')
		FROM sprint_issues si JOIN issues i ON si.issue_external_id = i.external_id
		LEFT JOIN statuses st ON st.external_id = i.status_id AND st.data_source = 'redmine'
		WHERE si.sprint_id = $1 AND `+visible+`
		ORDER BY `+priorityRank("i.priority_id")+` DESC NULLS LAST, i.external_id`, args...)
	if err != nil {
		return []SprintTask{}
	}
	defer rows.Close()

	today := time.Now().Format("2006-01-02")
	tasks := []SprintTask{}
	for rows.Next() {
		var t SprintTask
		if rows.Scan(&t.ExternalID, &t.Subject, &t.ProjectName, &t.StatusName,
			&t.PriorityName, &t.PriorityID, &t.AssignedToName, &t.DueDate, &t.EstimatedHours,
			&t.statusGroup) == nil {
			t.StatusGroup = t.statusGroup
			due := dateOnly(t.DueDate)
			t.IsOverdue = t.statusGroup != GroupClosed && due != "" && due < today
			tasks = append(tasks, t)
		}
	}
	sortSprintTasks(tasks)
	return tasks
}

// sortSprintTasks ставит задачи спринта в порядок экрана: открытые, в
// тестировании, закрытые. Порядок внутри каждой части (по приоритету)
// сохраняется.
func sortSprintTasks(tasks []SprintTask) {
	rank := func(group string) int {
		switch group {
		case GroupClosed:
			return 2
		case GroupTesting:
			return 1
		}
		return 0
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return rank(tasks[i].statusGroup) < rank(tasks[j].statusGroup)
	})
}

// autoFill добавляет в спринт задачи проекта и категории, которые
// пользователь вправе видеть: не закрытые, не в тестировании и не лежащие в
// другом незакрытом спринте того же вида.
func (h *SprintHandler) autoFill(userID, sprintID int, shared bool, projectName, categoryName string, scope *access.Scope) {
	taken, err := h.openSprintIDs(userID, shared)
	if err != nil {
		return
	}
	taken = append(taken, sprintID)

	args := []interface{}{projectName, categoryName}
	visible := scope.AllowedIssueCond("", &args)
	args = append(args, pq.Array(taken))
	takenParam := "$" + strconv.Itoa(len(args))
	rows, err := (*h.DB).Query(`SELECT external_id FROM issues
		WHERE project_name = $1 AND category_name = $2
		AND `+statusNotIn("status_id", GroupClosed, GroupTesting)+`
		AND external_id NOT IN (SELECT issue_external_id FROM sprint_issues WHERE sprint_id = ANY(`+takenParam+`))
		AND `+visible, args...)
	if err != nil {
		return
	}
	var issues []int
	for rows.Next() {
		var issueID int
		if rows.Scan(&issueID) == nil {
			issues = append(issues, issueID)
		}
	}
	_ = rows.Close()

	for _, issueID := range issues {
		if _, err := (*h.DB).Exec(`INSERT INTO sprint_issues (sprint_id, issue_external_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			sprintID, issueID); err != nil {
			slog.Warn("Database write failed", "at", "autoFill", "error", err)
		}
	}
}

// calculateProgress считает задачи по группам статусов и готовность спринта:
// закрытая задача — целая доля, задача в тестировании — половина.
func (s *Sprint) calculateProgress() {
	s.OpenCount, s.TestingCount, s.ClosedCount = 0, 0, 0
	for _, t := range s.Tasks {
		switch t.statusGroup {
		case GroupClosed:
			s.ClosedCount++
		case GroupTesting:
			s.TestingCount++
		default:
			s.OpenCount++
		}
	}

	s.Progress = 0
	if total := len(s.Tasks); total > 0 {
		s.Progress = (float64(s.ClosedCount) + float64(s.TestingCount)*0.5) / float64(total) * 100
	}
}
