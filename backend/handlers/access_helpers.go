package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"pm-dashboard/access"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/lib/pq"
)

// canSeeIssue сообщает, входит ли задача в права пользователя.
func canSeeIssue(db *sql.DB, scope *access.Scope, issueID int) bool {
	args := []interface{}{issueID}
	cond := scope.AllowedIssueCond("", &args)
	var visible bool
	// #nosec G701 -- условие прав из Scope и фильтры-константы из кода, значения через $n
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM issues WHERE external_id = $1 AND data_source = 'redmine' AND "+cond+")",
		args...).Scan(&visible)
	return err == nil && visible
}

// requireIssue отвечает 404 и возвращает false, если задачи нет или у
// пользователя нет права её видеть: снаружи оба случая выглядят одинаково,
// поэтому существование чужих задач не раскрывается.
func requireIssue(w http.ResponseWriter, r *http.Request, db *sql.DB, issueID int) bool {
	scope := middleware.GetScope(r)
	if canSeeIssue(db, scope, issueID) {
		return true
	}
	var exists bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM issues WHERE external_id = $1)", issueID).Scan(&exists); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("Database query failed", "at", "requireIssue", "error", err)
	}
	if exists {
		slog.Warn("Access to an issue outside of user rights", "user", scope.UserID, "issue", issueID, "path", r.URL.Path)
	}
	utils.Error(w, http.StatusNotFound, "TASK_NOT_FOUND")
	return false
}

// canSeeProject сообщает, видит ли пользователь в проекте хоть что-нибудь:
// проект разрешён или в нём есть задача, видимая пользователю.
func canSeeProject(db *sql.DB, scope *access.Scope, projectID int) bool {
	if scope.AllowsProject(projectID) {
		return true
	}
	args := []interface{}{projectID}
	cond := scope.AllowedIssueCond("", &args)
	var visible bool
	// #nosec G701 -- условие прав из Scope и фильтры-константы из кода, значения через $n
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM issues WHERE project_id = $1 AND "+cond+")", args...).Scan(&visible)
	return err == nil && visible
}

// warnForeignProject пишет в журнал запрос к проекту, на который у
// пользователя нет права: запрос по нему ничего не вернёт, но попытка
// должна быть видна.
func warnForeignProject(r *http.Request, scope *access.Scope, projectParam string) {
	if projectParam == "" {
		return
	}
	if id, err := strconv.Atoi(projectParam); err != nil || !scope.AllowsProject(id) {
		if scope.OwnMember != nil && err == nil {
			// Свои задачи могут быть в любом проекте; их отбирает сам запрос.
			return
		}
		slog.Warn("Request for a project outside of user rights", "user", scope.UserID, "project", projectParam, "path", r.URL.Path)
	}
}

// intList читает JSON-массив чисел, разобранный в interface{}; всё, что не
// является целым числом, пропускается.
func intList(v interface{}) []int {
	items, _ := v.([]interface{})
	result := make([]int, 0, len(items))
	for _, item := range items {
		if n, ok := item.(float64); ok && n == float64(int(n)) {
			result = append(result, int(n))
		}
	}
	return result
}

// uniqueIntCount возвращает различные значения списка.
func uniqueIntCount(list []int) map[int]bool {
	set := make(map[int]bool, len(list))
	for _, v := range list {
		set[v] = true
	}
	return set
}

// allowedProjectsCond возвращает условие для запроса проектов (в столбце
// col — идентификатор проекта), оставляющее проекты, которые пользователь
// вправе видеть, и его параметры.
func allowedProjectsCond(scope *access.Scope, col string) (string, []interface{}) {
	if scope.AllowedAllProjects {
		return "TRUE", nil
	}
	args := []interface{}{pq.Array(scope.AllowedProjects)}
	cond := col + " = ANY($1)"
	if scope.OwnMember != nil {
		// Проекты собственных задач пользователя тоже видны по названию.
		args = append(args, *scope.OwnMember)
		cond = "(" + cond + " OR " + col + " IN (SELECT project_id FROM issues WHERE assigned_to_id = $2))"
	}
	return cond, args
}

// allowedMembersCond — то же, что allowedProjectsCond, для сотрудников.
func allowedMembersCond(scope *access.Scope, col string) (string, []interface{}) {
	if scope.AllowedAllTeam {
		return "TRUE", nil
	}
	team := append([]int{}, scope.AllowedTeam...)
	if scope.OwnMember != nil {
		team = append(team, *scope.OwnMember)
	}
	return col + " = ANY($1)", []interface{}{pq.Array(team)}
}
