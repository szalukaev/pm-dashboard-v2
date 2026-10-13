package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"pm-dashboard/datasource/manager"
	"pm-dashboard/datasource/redmine"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/gorilla/mux"
)

type TaskHandler struct {
	DB     **sql.DB
	Source *manager.Manager
	// Events reports changes to open pages (WebSocket); may be nil.
	Events func(event string, data interface{})
	// Keys gives the personal API keys to write under the user's name.
	Keys *UserKeys
}

type TaskResponse struct {
	ID             int      `json:"id"`
	ExternalID     int      `json:"external_id"`
	ProjectID      int      `json:"project_id"`
	ProjectName    string   `json:"project_name"`
	Subject        string   `json:"subject"`
	Description    string   `json:"description,omitempty"`
	StatusName     string   `json:"status_name"`
	StatusID       int      `json:"status_id"`
	PriorityName   string   `json:"priority_name"`
	PriorityID     int      `json:"priority_id"`
	AssignedToName string   `json:"assigned_to_name"`
	AssignedToID   *int     `json:"assigned_to_id"`
	CategoryName   string   `json:"category_name"`
	StartDate      *string  `json:"start_date"`
	DueDate        *string  `json:"due_date"`
	EstimatedHours *float64 `json:"estimated_hours"`
	SpentHours     float64  `json:"spent_hours"`
	DoneRatio      int      `json:"done_ratio"`
	TrackerName    string   `json:"tracker_name"`
	AuthorName     string   `json:"author_name"`
	BugFixHours    float64  `json:"bug_fix_hours"`
}

type TaskGroup struct {
	Name          string         `json:"name"`
	TaskCount     int            `json:"task_count"`
	EstimateTotal float64        `json:"estimate_total"`
	FactTotal     float64        `json:"fact_total"`
	Tasks         []TaskResponse `json:"tasks"`
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	scope := middleware.GetScope(r)

	// Параметры запроса
	taskType := r.URL.Query().Get("type") // "open", "testing", "closed", "all"
	projectID := r.URL.Query().Get("project_id")
	search := r.URL.Query().Get("search")
	groupBy := r.URL.Query().Get("group_by") // "project", "assignee"
	sortBy := r.URL.Query().Get("sort_by")
	sortDir := r.URL.Query().Get("sort_dir")
	category := r.URL.Query().Get("category")

	// Только то, что пользователь вправе видеть: права от администратора,
	// суженные проектами и командой, выбранными в настройках пользователя.
	args := []interface{}{}
	where := []string{scope.IssueCond("", &args)}
	argIdx := len(args) + 1
	warnForeignProject(r, scope, projectID)

	// Отбор по типу задач — через группы статусов из таблицы statuses
	switch taskType {
	case "open", "testing", "closed":
		where = append(where, "status_id IN (SELECT external_id FROM statuses WHERE group_name = $"+strconv.Itoa(argIdx)+" AND data_source = 'redmine')")
		args = append(args, taskType)
		argIdx++
	case "all", "":
		// без отбора
	}

	// Отбор по проекту
	if projectID != "" {
		where = append(where, "project_id = $"+strconv.Itoa(argIdx))
		args = append(args, projectID)
		argIdx++
	}

	// Отбор по категории
	if category != "" {
		where = append(where, "category_name = $"+strconv.Itoa(argIdx))
		args = append(args, category)
		argIdx++
	}

	// Поиск
	if search != "" {
		where = append(where, "(LOWER(subject) LIKE $"+strconv.Itoa(argIdx)+" OR CAST(external_id AS TEXT) LIKE $"+strconv.Itoa(argIdx)+")")
		args = append(args, "%"+strings.ToLower(search)+"%")
		argIdx++
	}

	// Вид с группировкой: без группы ответ — заголовки групп (название,
	// число задач, итоги), с группой — задачи этой группы.
	groupExpr := ""
	switch groupBy {
	case "":
	case "assignee":
		groupExpr = "COALESCE(NULLIF(assigned_to_name, ''), 'Без исполнителя')"
	default:
		groupExpr = "COALESCE(NULLIF(project_name, ''), 'Без проекта')"
	}
	if groupExpr != "" && !r.URL.Query().Has("group") {
		h.listTaskGroups(w, groupExpr, strings.Join(where, " AND "), args)
		return
	}
	if groupExpr != "" {
		where = append(where, groupExpr+" = $"+strconv.Itoa(argIdx))
		args = append(args, r.URL.Query().Get("group"))
		argIdx++
	}

	// Сортировка
	orderBy := "external_id DESC"
	if sortBy != "" {
		validSorts := map[string]string{
			"subject":       "LOWER(subject)",
			"external_id":   "external_id",
			"project_name":  "LOWER(project_name)",
			"priority_name": priorityRank("priority_id"),
			"assigned_to":   "NULLIF(assigned_to_name, '')",
			"estimate":      "estimated_hours",
			"fact":          "spent_hours",
			"status_name":   "LOWER(status_name)",
			"bug_fix_hours": "bug_fix_hours",
			"bug_fix_pct":   "COALESCE(bug_fix_hours, 0) / NULLIF(spent_hours, 0)",
			"start_date":    "start_date",
			"due_date":      "due_date",
			"category_name": "NULLIF(category_name, '')",
		}
		if col, ok := validSorts[sortBy]; ok {
			dir := "ASC"
			if sortDir == "desc" {
				dir = "DESC"
			}
			// Номер задачи разрешает равенство: без него строки с равными
			// значениями могли бы переходить со страницы на страницу от
			// запроса к запросу. Пустые значения идут в конец при любом
			// направлении.
			orderBy = col + " " + dir + " NULLS LAST, external_id DESC"
		}
	}

	condition := strings.Join(where, " AND ")
	limit, offset := pageOf(r)
	total := 0
	if limit > 0 {
		if err := (*h.DB).QueryRow("SELECT COUNT(*) FROM issues WHERE "+condition, args...).Scan(&total); err != nil {
			utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
			return
		}
	}

	query := `SELECT external_id, project_id, project_name, subject, description,
		status_name, status_id, priority_name, priority_id,
		assigned_to_name, assigned_to_id, category_name,
		start_date, due_date, estimated_hours, spent_hours,
		done_ratio, tracker_name, author_name, COALESCE(bug_fix_hours, 0)
		FROM issues WHERE ` + condition + `
		ORDER BY ` + orderBy + pageClause(limit, offset)

	rows, err := (*h.DB).Query(query, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	var tasks []TaskResponse
	for rows.Next() {
		var t TaskResponse
		err := rows.Scan(
			&t.ExternalID, &t.ProjectID, &t.ProjectName, &t.Subject, &t.Description,
			&t.StatusName, &t.StatusID, &t.PriorityName, &t.PriorityID,
			&t.AssignedToName, &t.AssignedToID, &t.CategoryName,
			&t.StartDate, &t.DueDate, &t.EstimatedHours, &t.SpentHours,
			&t.DoneRatio, &t.TrackerName, &t.AuthorName, &t.BugFixHours,
		)
		if err != nil {
			continue
		}
		tasks = append(tasks, t)
	}

	if tasks == nil {
		tasks = []TaskResponse{}
	}

	// Без страниц: возвращено всё
	if limit == 0 {
		total = len(tasks)
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": total,
	})
}

// listTaskGroups отвечает на вид с группировкой заголовками групп; задачи
// группы запрашиваются отдельно, постранично.
func (h *TaskHandler) listTaskGroups(w http.ResponseWriter, groupExpr, condition string, args []interface{}) {
	rows, err := (*h.DB).Query(`SELECT `+groupExpr+` AS name, COUNT(*),
		COALESCE(SUM(estimated_hours), 0), COALESCE(SUM(spent_hours), 0)
		FROM issues WHERE `+condition+`
		GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	groups := []TaskGroup{}
	total := 0
	for rows.Next() {
		g := TaskGroup{Tasks: []TaskResponse{}}
		if rows.Scan(&g.Name, &g.TaskCount, &g.EstimateTotal, &g.FactTotal) != nil {
			continue
		}
		total += g.TaskCount
		groups = append(groups, g)
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"groups": groups,
		"total":  total,
	})
}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	vars := mux.Vars(r)
	externalID, err := strconv.Atoi(vars["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	if !requireIssue(w, r, *h.DB, externalID) {
		return
	}

	var t TaskResponse
	err = (*h.DB).QueryRow(`SELECT external_id, project_id, project_name, subject, description,
		status_name, status_id, priority_name, priority_id,
		assigned_to_name, assigned_to_id, category_name,
		start_date, due_date, estimated_hours, spent_hours,
		done_ratio, tracker_name, author_name, COALESCE(bug_fix_hours, 0)
		FROM issues WHERE external_id = $1`, externalID).Scan(
		&t.ExternalID, &t.ProjectID, &t.ProjectName, &t.Subject, &t.Description,
		&t.StatusName, &t.StatusID, &t.PriorityName, &t.PriorityID,
		&t.AssignedToName, &t.AssignedToID, &t.CategoryName,
		&t.StartDate, &t.DueDate, &t.EstimatedHours, &t.SpentHours,
		&t.DoneRatio, &t.TrackerName, &t.AuthorName, &t.BugFixHours,
	)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "TASK_NOT_FOUND")
		return
	}

	utils.JSON(w, http.StatusOK, t)
}

func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	vars := mux.Vars(r)
	externalID, err := strconv.Atoi(vars["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	if !requireIssue(w, r, *h.DB, externalID) {
		return
	}

	// Категория ищется в пределах проекта задачи.
	var projectID int
	if err := (*h.DB).QueryRow("SELECT COALESCE(project_id, 0) FROM issues WHERE external_id = $1", externalID).Scan(&projectID); err != nil {
		utils.Error(w, http.StatusNotFound, "TASK_NOT_FOUND")
		return
	}

	// Изменения уходят в источник от имени самого пользователя, если он
	// ввёл личный ключ API.
	client := h.writeClient(r)

	// localSets: столбец -> новое значение для локальной копии;
	// redmineFields: поля для источника данных. Пустая строка очищает поле в Redmine.
	localSets := map[string]interface{}{}
	redmineFields := map[string]interface{}{}
	editable := []string{}

	for field, value := range body {
		strValue, _ := value.(string)
		switch field {
		case "status_name":
			var statusID int
			if err := (*h.DB).QueryRow("SELECT external_id FROM statuses WHERE name = $1 AND data_source = 'redmine'", strValue).Scan(&statusID); err != nil {
				utils.Error(w, http.StatusBadRequest, "STATUS_NOT_FOUND")
				return
			}
			localSets["status_name"] = strValue
			localSets["status_id"] = statusID
			redmineFields["status_id"] = statusID
		case "priority_name":
			var priorityID int
			if err := (*h.DB).QueryRow("SELECT external_id FROM priorities WHERE name = $1 AND data_source = 'redmine'", strValue).Scan(&priorityID); err != nil {
				utils.Error(w, http.StatusBadRequest, "PRIORITY_NOT_FOUND")
				return
			}
			localSets["priority_name"] = strValue
			localSets["priority_id"] = priorityID
			redmineFields["priority_id"] = priorityID
		case "assigned_to_name":
			if strValue == "" {
				localSets["assigned_to_name"] = ""
				localSets["assigned_to_id"] = nil
				redmineFields["assigned_to_id"] = ""
				break
			}
			var assigneeID int
			if err := (*h.DB).QueryRow("SELECT external_id FROM members WHERE name = $1 AND data_source = 'redmine' LIMIT 1", strValue).Scan(&assigneeID); err != nil {
				utils.Error(w, http.StatusBadRequest, "MEMBER_NOT_FOUND")
				return
			}
			// Назначить задачу можно только сотруднику, которого пользователь вправе видеть.
			if !middleware.GetScope(r).AllowsMember(assigneeID) {
				utils.Error(w, http.StatusForbidden, "MEMBER_NOT_ALLOWED")
				return
			}
			localSets["assigned_to_name"] = strValue
			localSets["assigned_to_id"] = assigneeID
			redmineFields["assigned_to_id"] = assigneeID
		case "start_date", "due_date":
			if strValue == "" {
				localSets[field] = nil
			} else {
				localSets[field] = strValue
			}
			redmineFields[field] = strValue
		case "estimated_hours":
			hours, ok := parseHours(value)
			if !ok {
				utils.Error(w, http.StatusBadRequest, "INVALID_VALUE")
				return
			}
			if hours == nil {
				localSets[field] = nil
			} else {
				localSets[field] = *hours
			}
			if client == nil {
				break
			}
			// Пишется в оценку разработчика, если она у задачи уже есть,
			// иначе в стандартное estimated_hours.
			fields, err := client.EstimateFields(externalID, hours)
			if err != nil {
				slog.Warn("Failed to read issue estimate fields", "issue", externalID, "error", err)
				utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
				return
			}
			for k, v := range fields {
				redmineFields[k] = v
			}
		case "category_name":
			localSets["category_name"] = strValue
			if strValue == "" {
				redmineFields["category_id"] = ""
				break
			}
			if client == nil {
				break
			}
			categoryID, err := client.FindCategoryID(projectID, strValue)
			if err != nil {
				utils.Error(w, http.StatusBadRequest, "CATEGORY_NOT_FOUND")
				return
			}
			redmineFields["category_id"] = categoryID
		default:
			continue
		}
		editable = append(editable, field)
	}

	if len(localSets) == 0 {
		utils.Error(w, http.StatusBadRequest, "NO_FIELDS_TO_UPDATE")
		return
	}

	// Прежние значения — для уведомления об изменении. Имена полей берутся из switch выше.
	oldValues := make(map[string]interface{})
	for _, field := range editable {
		var oldVal sql.NullString
		(*h.DB).QueryRow("SELECT "+field+"::text FROM issues WHERE external_id = $1", externalID).Scan(&oldVal)
		if oldVal.Valid {
			oldValues[field] = oldVal.String
		} else {
			oldValues[field] = nil
		}
	}

	// Главный — источник данных: сначала запись туда, а локальная копия
	// меняется только при успехе, иначе следующая синхронизация молча
	// откатила бы изменение.
	if client != nil {
		if err := client.UpdateIssue(externalID, redmineFields); err != nil {
			slog.Warn("Failed to update issue in Redmine", "issue", externalID, "error", err)
			utils.JSON(w, http.StatusOK, map[string]interface{}{
				"success":       false,
				"old_values":    oldValues,
				"new_values":    body,
				"redmine_ok":    false,
				"redmine_error": err.Error(),
			})
			return
		}
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1
	for col, value := range localSets {
		setClauses = append(setClauses, col+" = $"+strconv.Itoa(argIdx))
		args = append(args, value)
		argIdx++
	}
	args = append(args, externalID)
	query := "UPDATE issues SET " + strings.Join(setClauses, ", ") + ", synced_at = NOW() WHERE external_id = $" + strconv.Itoa(argIdx)
	if _, err := (*h.DB).Exec(query, args...); err != nil {
		utils.Error(w, http.StatusInternalServerError, "UPDATE_FAILED")
		return
	}

	notifyIssuesUpdated(h.Events, externalID)
	issueEdited(r, externalID)

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"old_values":    oldValues,
		"new_values":    body,
		"redmine_ok":    true,
		"redmine_error": "",
	})
}

// parseHours принимает число JSON, строку с числом ("1.5" / "1,5") или
// пустое значение. Результат nil при ok=true значит «очистить оценку».
func parseHours(value interface{}) (*float64, bool) {
	switch v := value.(type) {
	case nil:
		return nil, true
	case float64:
		if v < 0 {
			return nil, false
		}
		return &v, true
	case string:
		s := strings.TrimSpace(strings.Replace(v, ",", ".", 1))
		if s == "" {
			return nil, true
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f < 0 {
			return nil, false
		}
		return &f, true
	default:
		return nil, false
	}
}

// writeClient возвращает клиента для изменения данных: личного, если у
// пользователя есть ключ, иначе системного.
func (h *TaskHandler) writeClient(r *http.Request) *redmine.Client {
	if h.Keys != nil {
		return h.Keys.ClientFor(middleware.GetUserID(r))
	}
	return h.getRedmineClient()
}

// getRedmineClient возвращает текущего клиента источника данных; nil —
// источник не настроен.
func (h *TaskHandler) getRedmineClient() *redmine.Client {
	if h.Source == nil {
		return nil
	}
	return h.Source.Client()
}

func (h *TaskHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	vars := mux.Vars(r)
	externalID, err := strconv.Atoi(vars["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	if !requireIssue(w, r, *h.DB, externalID) {
		return
	}

	client := h.getRedmineClient()
	if client == nil {
		utils.JSON(w, http.StatusOK, map[string]interface{}{"comments": []interface{}{}})
		return
	}

	comments, err := client.GetIssueComments(externalID)
	if err != nil {
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}
	if comments == nil {
		comments = []redmine.Comment{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"comments": comments})
}

// GetTaskDetails возвращает ту часть карточки задачи, которая читается из
// Redmine напрямую: исходную разметку описания, файлы и историю комментариев.
func (h *TaskHandler) GetTaskDetails(w http.ResponseWriter, r *http.Request) {
	externalID, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	if !requireIssue(w, r, *h.DB, externalID) {
		return
	}
	client := h.getRedmineClient()
	if client == nil {
		utils.Error(w, http.StatusServiceUnavailable, "REDMINE_NOT_CONFIGURED")
		return
	}
	details, err := client.GetIssueDetails(externalID)
	if err != nil {
		slog.Warn("Failed to get issue details", "issue", externalID, "error", err)
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}
	describeHistory(*h.DB, client, externalID, details.History)
	describeRelated(*h.DB, middleware.GetScope(r), details.Related)
	utils.JSON(w, http.StatusOK, details)
}

// inlineImageTypes можно показывать на странице; всё остальное отдаётся как
// скачиваемый файл, чтобы загруженный HTML или SVG не выполнился на нашем
// адресе.
var inlineImageTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"image/bmp":  true,
}

// GetTaskAttachment отдаёт файл задачи из Redmine через сервер (у браузера
// нет учётных данных Redmine). Отдаются только файлы этой задачи.
func (h *TaskHandler) GetTaskAttachment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	externalID, err1 := strconv.Atoi(vars["id"])
	attachmentID, err2 := strconv.Atoi(vars["attachment_id"])
	if err1 != nil || err2 != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	if !requireIssue(w, r, *h.DB, externalID) {
		return
	}
	client := h.getRedmineClient()
	if client == nil {
		utils.Error(w, http.StatusServiceUnavailable, "REDMINE_NOT_CONFIGURED")
		return
	}
	details, err := client.GetIssueDetails(externalID)
	if err != nil {
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}
	var att *redmine.Attachment
	for i := range details.Attachments {
		if details.Attachments[i].ID == attachmentID {
			att = &details.Attachments[i]
			break
		}
	}
	if att == nil {
		utils.Error(w, http.StatusNotFound, "ATTACHMENT_NOT_FOUND")
		return
	}

	body, err := client.DownloadAttachment(attachmentID)
	if err != nil {
		slog.Warn("Failed to download attachment", "issue", externalID, "attachment", attachmentID, "error", err)
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}
	defer body.Close()

	contentType := strings.ToLower(att.ContentType)
	disposition := "attachment"
	if inlineImageTypes[contentType] {
		disposition = "inline"
	} else {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": att.Filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if att.Filesize > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(att.Filesize, 10))
	}
	io.Copy(w, body)
}

func (h *TaskHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}
	vars := mux.Vars(r)
	externalID, err := strconv.Atoi(vars["id"])
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "INVALID_ID")
		return
	}
	if !requireIssue(w, r, *h.DB, externalID) {
		return
	}

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		utils.Error(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	// Комментарий по возможности пишется от имени самого пользователя
	client := h.writeClient(r)
	if client == nil {
		utils.Error(w, http.StatusServiceUnavailable, "REDMINE_NOT_CONFIGURED")
		return
	}

	if err := client.AddIssueComment(externalID, body.Text); err != nil {
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *TaskHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	// Категории только тех задач, которые пользователь вправе видеть
	scope := middleware.GetScope(r)
	args := []interface{}{}
	where := "category_name IS NOT NULL AND category_name != '' AND " + scope.ProjectIssueCond("", &args)
	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		warnForeignProject(r, scope, projectID)
		args = append(args, projectID)
		where += " AND project_id = $" + strconv.Itoa(len(args))
	}
	rows, err := (*h.DB).Query("SELECT DISTINCT category_name FROM issues WHERE "+where+" ORDER BY category_name", args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var c string
		if rows.Scan(&c) == nil {
			categories = append(categories, c)
		}
	}

	if categories == nil {
		categories = []string{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"categories": categories})
}

// GetProjectCategories возвращает все категории проекта (а не только уже
// использованные в задачах) — варианты для правки задачи.
func (h *TaskHandler) GetProjectCategories(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.Atoi(r.URL.Query().Get("project_id"))
	if err != nil || projectID <= 0 {
		utils.Error(w, http.StatusBadRequest, "INVALID_PROJECT_ID")
		return
	}
	if scope := middleware.GetScope(r); *h.DB == nil || !canSeeProject(*h.DB, scope, projectID) {
		slog.Warn("Request for a project outside of user rights", "user", scope.UserID, "project", projectID, "path", r.URL.Path)
		utils.JSON(w, http.StatusOK, map[string]interface{}{"categories": []string{}})
		return
	}

	client := h.getRedmineClient()
	if client == nil {
		utils.Error(w, http.StatusServiceUnavailable, "REDMINE_NOT_CONFIGURED")
		return
	}

	list, err := client.GetCategories(projectID)
	if err != nil {
		slog.Warn("Failed to get project categories", "project_id", projectID, "error", err)
		utils.Error(w, http.StatusBadGateway, "REDMINE_ERROR")
		return
	}

	categories := make([]string, 0, len(list))
	for _, c := range list {
		categories = append(categories, c.Name)
	}
	sort.Strings(categories)

	utils.JSON(w, http.StatusOK, map[string]interface{}{"categories": categories})
}

func (h *TaskHandler) GetProjects(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	// Проекты, разрешённые администратором; собственный выбор пользователя
	// здесь не действует — из этого списка выбор и делается.
	cond, args := allowedProjectsCond(middleware.GetScope(r), "external_id")
	rows, err := (*h.DB).Query("SELECT external_id, name, parent_id FROM projects WHERE "+cond+" ORDER BY name", args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type ProjectInfo struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		ParentID *int   `json:"parent_id"`
	}
	var projects []ProjectInfo
	for rows.Next() {
		var p ProjectInfo
		if rows.Scan(&p.ID, &p.Name, &p.ParentID) == nil {
			projects = append(projects, p)
		}
	}
	if projects == nil {
		projects = []ProjectInfo{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"projects": projects})
}

func (h *TaskHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	// Сотрудники, разрешённые администратором (см. GetProjects)
	cond, args := allowedMembersCond(middleware.GetScope(r), "external_id")
	rows, err := (*h.DB).Query("SELECT external_id, name FROM members WHERE data_source = 'redmine' AND name != '' AND "+cond+" ORDER BY name", args...)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "DB_ERROR")
		return
	}
	defer rows.Close()

	members := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		members = append(members, map[string]interface{}{
			"id":   id,
			"name": name,
		})
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"members": members})
}

func (h *TaskHandler) GetPriorities(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query("SELECT external_id, name, sort_order, COALESCE(color, '') FROM priorities WHERE data_source = 'redmine' ORDER BY external_id DESC")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type PriorityInfo struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
		// Цвет, заданный администратором в «Порядке приоритетов»
		Color string `json:"color"`
	}
	var priorities []PriorityInfo
	for rows.Next() {
		var p PriorityInfo
		if rows.Scan(&p.ID, &p.Name, &p.SortOrder, &p.Color) == nil {
			priorities = append(priorities, p)
		}
	}
	if priorities == nil {
		priorities = []PriorityInfo{}
	}
	utils.JSON(w, http.StatusOK, map[string]interface{}{"priorities": priorities})
}

func (h *TaskHandler) GetStatuses(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	rows, err := (*h.DB).Query("SELECT external_id, name, is_closed, group_name, is_bug FROM statuses ORDER BY external_id")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}
	defer rows.Close()

	type StatusInfo struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		IsClosed bool   `json:"is_closed"`
		Group    string `json:"group"`
		IsBug    bool   `json:"is_bug"`
	}
	var statuses []StatusInfo
	for rows.Next() {
		var s StatusInfo
		if rows.Scan(&s.ID, &s.Name, &s.IsClosed, &s.Group, &s.IsBug) == nil {
			statuses = append(statuses, s)
		}
	}
	if statuses == nil {
		statuses = []StatusInfo{}
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{"statuses": statuses})
}
