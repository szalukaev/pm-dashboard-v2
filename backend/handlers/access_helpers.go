package handlers

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"pm-dashboard/access"
	"pm-dashboard/middleware"
	"pm-dashboard/utils"

	"github.com/lib/pq"
)

// canSeeIssue reports whether the issue is within the user's rights.
func canSeeIssue(db *sql.DB, scope *access.Scope, issueID int) bool {
	args := []interface{}{issueID}
	cond := scope.AllowedIssueCond("", &args)
	var visible bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM issues WHERE external_id = $1 AND data_source = 'redmine' AND "+cond+")",
		args...).Scan(&visible)
	return err == nil && visible
}

// requireIssue answers 404 and returns false when the issue does not exist
// or the user has no right to see it — the two cases look the same from
// outside, so the existence of foreign issues does not leak.
func requireIssue(w http.ResponseWriter, r *http.Request, db *sql.DB, issueID int) bool {
	scope := middleware.GetScope(r)
	if canSeeIssue(db, scope, issueID) {
		return true
	}
	var exists bool
	db.QueryRow("SELECT EXISTS(SELECT 1 FROM issues WHERE external_id = $1)", issueID).Scan(&exists)
	if exists {
		slog.Warn("Access to an issue outside of user rights", "user", scope.UserID, "issue", issueID, "path", r.URL.Path)
	}
	utils.Error(w, http.StatusNotFound, "TASK_NOT_FOUND")
	return false
}

// canSeeProject reports whether the user may see anything of the project:
// the project is allowed or holds an issue visible to the user.
func canSeeProject(db *sql.DB, scope *access.Scope, projectID int) bool {
	if scope.AllowsProject(projectID) {
		return true
	}
	args := []interface{}{projectID}
	cond := scope.AllowedIssueCond("", &args)
	var visible bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM issues WHERE project_id = $1 AND "+cond+")", args...).Scan(&visible)
	return err == nil && visible
}

// warnForeignProject logs a request for a project the user has no right to:
// the query returns nothing for it, but the attempt must be visible.
func warnForeignProject(r *http.Request, scope *access.Scope, projectParam string) {
	if projectParam == "" {
		return
	}
	if id, err := strconv.Atoi(projectParam); err != nil || !scope.AllowsProject(id) {
		if scope.OwnMember != nil && err == nil {
			// Own tasks may live in any project; the query itself filters them.
			return
		}
		slog.Warn("Request for a project outside of user rights", "user", scope.UserID, "project", projectParam, "path", r.URL.Path)
	}
}

// intList reads a JSON array of numbers decoded into interface{}; anything
// that is not a whole number is skipped.
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

// uniqueIntCount returns the distinct values of a list.
func uniqueIntCount(list []int) map[int]bool {
	set := make(map[int]bool, len(list))
	for _, v := range list {
		set[v] = true
	}
	return set
}

// allowedProjectsCond returns a condition on a projects query (column col
// holds the project id) keeping the projects the user may see, and its args.
func allowedProjectsCond(scope *access.Scope, col string) (string, []interface{}) {
	if scope.AllowedAllProjects {
		return "TRUE", nil
	}
	args := []interface{}{pq.Array(scope.AllowedProjects)}
	cond := col + " = ANY($1)"
	if scope.OwnMember != nil {
		// Projects of the user's own tasks are visible by name as well.
		args = append(args, *scope.OwnMember)
		cond = "(" + cond + " OR " + col + " IN (SELECT project_id FROM issues WHERE assigned_to_id = $2))"
	}
	return cond, args
}

// allowedMembersCond is allowedProjectsCond for members.
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
