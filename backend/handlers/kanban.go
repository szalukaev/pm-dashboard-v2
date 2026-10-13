package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"pm-dashboard/access"
	"pm-dashboard/datasource"
	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/lib/pq"
)

type KanbanHandler struct {
	DB     **sql.DB
	Source *manager.Manager
	// Events сообщает об изменениях открытым страницам (WebSocket); может быть nil.
	Events func(event string, data interface{})
	// Keys даёт личные ключи API, чтобы писать от имени пользователя.
	Keys *UserKeys
}

type KanbanCard struct {
	ExternalID     int      `json:"external_id"`
	Subject        string   `json:"subject"`
	ProjectName    string   `json:"project_name"`
	StatusName     string   `json:"status_name"`
	StatusID       int      `json:"status_id"`
	PriorityName   string   `json:"priority_name"`
	PriorityID     int      `json:"priority_id"`
	AssignedToName string   `json:"assigned_to_name"`
	AssignedToID   *int     `json:"assigned_to_id"`
	CategoryName   string   `json:"category_name"`
	DueDate        *string  `json:"due_date"`
	EstimatedHours *float64 `json:"estimated_hours"`
	IsOverdue      bool     `json:"is_overdue"`
}

type KanbanColumn struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Icon  string       `json:"icon,omitempty"`
	Tasks []KanbanCard `json:"tasks"`
}

type KanbanBoard struct {
	Mode    string         `json:"mode"` // "users" или "statuses"
	Columns []KanbanColumn `json:"columns"`
	Total   int            `json:"total"`
}

func (h *KanbanHandler) GetBoard(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode != "statuses" {
		mode = "users"
	}

	var columns []KanbanColumn
	var err error
	scope := middleware.GetScope(r)
	// Вид доски, закрытый администратором, не отдаётся и по прямому запросу
	if !scope.CanKanban(mode) {
		slog.Warn("Kanban mode outside of user rights", "user", middleware.GetUserID(r), "mode", mode)
		utils.Error(w, http.StatusForbidden, "KANBAN_MODE_FORBIDDEN")
		return
	}
	if mode == "statuses" {
		warnForeignProject(r, scope, r.URL.Query().Get("project_id"))
		columns, err = h.statusBoard(scope, r.URL.Query().Get("project_id"))
	} else {
		columns, err = h.userBoard(scope)
	}
	if err != nil {
		slog.Error("Failed to build the kanban board", "mode", mode, "error", err)
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}

	board := KanbanBoard{Mode: mode, Columns: columns}
	for _, col := range columns {
		board.Total += len(col.Tasks)
	}
	utils.JSON(w, http.StatusOK, board)
}

// statusBoard: колонка на каждый статус источника данных с задачами
// выбранного проекта и всех его подпроектов. Нет проекта — нет колонок,
// клиент просит его выбрать.
func (h *KanbanHandler) statusBoard(scope *access.Scope, projectParam string) ([]KanbanColumn, error) {
	userID := scope.UserID
	projectID, err := strconv.Atoi(projectParam)
	if err != nil {
		return []KanbanColumn{}, nil
	}
	projects, err := datasource.ExpandProjects(*h.DB, []int{projectID})
	if err != nil {
		return nil, err
	}
	// Проект выбран явно, поэтому собственный выбор проектов пользователя
	// не действует — действуют только права.
	args := []interface{}{pq.Array(projects)}
	cards, err := h.loadCards("project_id = ANY($1) AND "+scope.AllowedIssueCond("", &args), args...)
	if err != nil {
		return nil, err
	}

	rows, err := (*h.DB).Query("SELECT external_id, name FROM statuses WHERE data_source = 'redmine' ORDER BY external_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []KanbanColumn{}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		columns = append(columns, KanbanColumn{ID: strconv.Itoa(id), Name: name})
	}

	columns = fillColumns(columns, cards, func(c KanbanCard) string { return strconv.Itoa(c.StatusID) })
	return orderColumns(columns, h.columnOrder(userID, "kanban_column_order_statuses")), nil
}

// userBoard: колонка на каждого сотрудника команды пользователя и «без
// исполнителя» (пустая скрыта); только задачи в открытых статусах.
func (h *KanbanHandler) userBoard(scope *access.Scope) ([]KanbanColumn, error) {
	userID := scope.UserID

	// Команда — те, за кем пользователь следит в пределах прав; у самого
	// пользователя тоже есть колонка, если он видит только свои задачи.
	var team []int
	if !scope.AllTeam {
		team = append(team, scope.Team...)
		if scope.OwnMember != nil && !containsInt(team, *scope.OwnMember) {
			team = append(team, *scope.OwnMember)
		}
	}

	args := []interface{}{}
	where := statusIn("status_id", GroupOpen) + " AND " + scope.IssueCond("", &args)
	cards, err := h.loadCards(where, args...)
	if err != nil {
		return nil, err
	}

	// Если команда задана, колонки — её сотрудники, включая тех, у кого
	// задач нет; если нет — все, у кого задачи есть.
	columns := []KanbanColumn{}
	if !scope.AllTeam {
		rows, err := (*h.DB).Query("SELECT external_id, name FROM members WHERE data_source = 'redmine' AND external_id = ANY($1) ORDER BY name",
			pq.Array(team))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			if err := rows.Scan(&id, &name); err != nil {
				return nil, err
			}
			columns = append(columns, KanbanColumn{ID: strconv.Itoa(id), Name: name})
		}
	} else {
		seen := map[int]bool{}
		for _, c := range cards {
			if c.AssignedToID != nil && !seen[*c.AssignedToID] {
				seen[*c.AssignedToID] = true
				columns = append(columns, KanbanColumn{ID: strconv.Itoa(*c.AssignedToID), Name: c.AssignedToName})
			}
		}
		sort.Slice(columns, func(i, j int) bool { return columns[i].Name < columns[j].Name })
	}
	// Заголовок этой колонки задаёт клиент (он переводится).
	columns = append(columns, KanbanColumn{ID: unassignedColumn})

	columns = fillColumns(columns, cards, func(c KanbanCard) string {
		if c.AssignedToID == nil {
			return unassignedColumn
		}
		return strconv.Itoa(*c.AssignedToID)
	})
	if last := len(columns) - 1; len(columns[last].Tasks) == 0 {
		columns = columns[:last]
	}
	return orderColumns(columns, h.columnOrder(userID, "kanban_column_order_users")), nil
}

// loadCards читает задачи для доски, сначала самые важные.
func (h *KanbanHandler) loadCards(where string, args ...interface{}) ([]KanbanCard, error) {
	rows, err := (*h.DB).Query(`SELECT external_id, subject, project_name, status_name, status_id,
		priority_name, priority_id, assigned_to_name, assigned_to_id,
		category_name, due_date, estimated_hours,
		COALESCE(`+statusNotIn("status_id", GroupClosed)+`, true)
		FROM issues WHERE data_source = 'redmine' AND `+where+`
		ORDER BY `+priorityRank("priority_id")+` DESC NULLS LAST, external_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Now().Format("2006-01-02")
	var cards []KanbanCard
	for rows.Next() {
		var c KanbanCard
		var notClosed bool
		if err := rows.Scan(&c.ExternalID, &c.Subject, &c.ProjectName, &c.StatusName, &c.StatusID,
			&c.PriorityName, &c.PriorityID, &c.AssignedToName, &c.AssignedToID,
			&c.CategoryName, &c.DueDate, &c.EstimatedHours, &notClosed); err != nil {
			return nil, err
		}
		// Просрочена: срок прошёл, а задача не закрыта
		c.IsOverdue = notClosed && c.DueDate != nil && (*c.DueDate)[:min(10, len(*c.DueDate))] < today
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// columnOrder возвращает порядок колонок, сохранённый пользователем для вида доски.
func (h *KanbanHandler) columnOrder(userID int, column string) []string {
	var data []byte
	(*h.DB).QueryRow("SELECT "+column+" FROM user_settings WHERE user_id = $1", userID).Scan(&data)
	var order []string
	if len(data) > 0 {
		json.Unmarshal(data, &order)
	}
	return order
}

// fillColumns кладёт каждую карточку в колонку с идентификатором, который
// возвращает key(card); карточки без колонки отбрасываются. Tasks в
// результате никогда не равен nil.
func fillColumns(columns []KanbanColumn, cards []KanbanCard, key func(KanbanCard) string) []KanbanColumn {
	index := make(map[string]int, len(columns))
	for i := range columns {
		columns[i].Tasks = []KanbanCard{}
		index[columns[i].ID] = i
	}
	for _, c := range cards {
		if i, ok := index[key(c)]; ok {
			columns[i].Tasks = append(columns[i].Tasks, c)
		}
	}
	return columns
}

// orderColumns ставит колонки, перечисленные в order (идентификаторы),
// первыми и в этом порядке; остальные сохраняют взаимное расположение.
func orderColumns(columns []KanbanColumn, order []string) []KanbanColumn {
	position := make(map[string]int, len(order))
	for i, id := range order {
		if _, dup := position[id]; !dup {
			position[id] = i
		}
	}
	sort.SliceStable(columns, func(i, j int) bool {
		pi, okI := position[columns[i].ID]
		pj, okJ := position[columns[j].ID]
		if okI != okJ {
			return okI
		}
		return okI && pi < pj
	})
	return columns
}

func (h *KanbanHandler) MoveCard(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	var body struct {
		IssueID  int    `json:"issue_id"`
		TargetID string `json:"target_id"` // status_id или имя сотрудника
		Mode     string `json:"mode"`      // "users" или "statuses"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Задача должна входить в права пользователя.
	scope := middleware.GetScope(r)
	// Перенос карточки на доске, вид которой закрыт администратором, не принимается
	if (body.Mode == access.KanbanUsers || body.Mode == access.KanbanStatuses) && !scope.CanKanban(body.Mode) {
		utils.Error(w, http.StatusForbidden, "KANBAN_MODE_FORBIDDEN")
		return
	}
	if !requireIssue(w, r, *h.DB, body.IssueID) {
		return
	}

	// localSet: новые значения для локальной копии; redmineFields: поля для
	// источника данных (пустая строка очищает поле в Redmine).
	var localSet string
	var localArgs []interface{}
	redmineFields := map[string]interface{}{}

	switch body.Mode {
	case "statuses":
		var statusID int
		var statusName string
		if err := (*h.DB).QueryRow("SELECT external_id, name FROM statuses WHERE external_id::text = $1 AND data_source = 'redmine'",
			body.TargetID).Scan(&statusID, &statusName); err != nil {
			utils.Error(w, http.StatusBadRequest, "STATUS_NOT_FOUND")
			return
		}
		localSet = "status_name = $1, status_id = $2"
		localArgs = []interface{}{statusName, statusID}
		redmineFields["status_id"] = statusID

	case "users":
		if body.TargetID == unassignedColumn {
			localSet = "assigned_to_name = $1, assigned_to_id = $2"
			localArgs = []interface{}{"", nil}
			redmineFields["assigned_to_id"] = ""
			break
		}
		var memberID int
		var memberName string
		if err := (*h.DB).QueryRow("SELECT external_id, name FROM members WHERE external_id::text = $1 AND data_source = 'redmine'",
			body.TargetID).Scan(&memberID, &memberName); err != nil {
			utils.Error(w, http.StatusBadRequest, "MEMBER_NOT_FOUND")
			return
		}
		// Назначить задачу можно только сотруднику, которого пользователь вправе видеть.
		if !scope.AllowsMember(memberID) {
			utils.Error(w, http.StatusForbidden, "FORBIDDEN")
			return
		}
		localSet = "assigned_to_name = $1, assigned_to_id = $2"
		localArgs = []interface{}{memberName, memberID}
		redmineFields["assigned_to_id"] = memberID

	default:
		utils.Error(w, http.StatusBadRequest, "INVALID_MODE")
		return
	}

	// Главный — источник данных: сначала запись туда, а локальная копия
	// меняется только при успехе, иначе следующая синхронизация молча
	// откатила бы перенос.
	// Перенос делается от имени самого пользователя, если он ввёл личный
	// ключ API.
	var client *redmine.Client
	if h.Keys != nil {
		client = h.Keys.ClientFor(scope.UserID)
	} else if h.Source != nil {
		client = h.Source.Client()
	}
	if client == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATA_SOURCE_NOT_CONFIGURED")
		return
	}
	if err := client.UpdateIssue(body.IssueID, redmineFields); err != nil {
		slog.Warn("Failed to move issue in Redmine", "issue", body.IssueID, "error", err)
		utils.JSON(w, http.StatusBadGateway, map[string]string{
			"error":   "REDMINE_UPDATE_FAILED",
			"message": err.Error(),
		})
		return
	}
	// Redmine принимает запрос, но молча пропускает изменение, которое
	// запрещает его workflow: прежде чем верить переносу, задача читается заново.
	if state, err := client.GetIssueState(body.IssueID); err != nil {
		slog.Warn("Could not verify the move in Redmine", "issue", body.IssueID, "error", err)
	} else if !moveApplied(state, redmineFields) {
		utils.Error(w, http.StatusBadGateway, "REDMINE_CHANGE_REJECTED")
		return
	}

	localArgs = append(localArgs, body.IssueID)
	if _, err := (*h.DB).Exec("UPDATE issues SET "+localSet+", synced_at = NOW() WHERE external_id = $3 AND data_source = 'redmine'",
		localArgs...); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}

	notifyIssuesUpdated(h.Events, body.IssueID)
	issueEdited(r, body.IssueID)
	utils.Success(w)
}

// unassignedColumn — идентификатор колонки «без исполнителя» на доске по сотрудникам.
const unassignedColumn = "unassigned"

// moveApplied сообщает, имеет ли задача в Redmine значения, которых
// требовал перенос (fields — то, что ушло в UpdateIssue).
func moveApplied(state *redmine.IssueState, fields map[string]interface{}) bool {
	if want, ok := fields["status_id"]; ok && state.StatusID != want {
		return false
	}
	if want, ok := fields["assigned_to_id"]; ok {
		if want == "" {
			return state.AssignedToID == nil
		}
		return state.AssignedToID != nil && *state.AssignedToID == want
	}
	return true
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (h *KanbanHandler) SaveColumnOrder(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	userID := middleware.GetUserID(r)

	var body struct {
		Mode  string   `json:"mode"`
		Order []string `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	orderJSON, _ := json.Marshal(body.Order)

	if _, err := (*h.DB).Exec(`INSERT INTO user_settings (user_id, updated_at) VALUES ($1, NOW())
		ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}

	switch body.Mode {
	case "statuses":
		if _, err := (*h.DB).Exec("UPDATE user_settings SET kanban_column_order_statuses = $1 WHERE user_id = $2", orderJSON, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	case "users":
		if _, err := (*h.DB).Exec("UPDATE user_settings SET kanban_column_order_users = $1 WHERE user_id = $2", orderJSON, userID); err != nil {
			utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
			return
		}
	}

	utils.Success(w)
}
