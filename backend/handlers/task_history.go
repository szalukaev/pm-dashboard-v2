package handlers

import (
	"database/sql"
	"log/slog"
	"strconv"

	"pm-dashboard/access"
	"pm-dashboard/datasource/redmine"

	"github.com/lib/pq"
)

// Fields of the history whose values Redmine reports as ids, and the table
// the names are read from.
var historyNameTables = map[string]string{
	"status":      "statuses",
	"priority":    "priorities",
	"assigned_to": "members",
	"project":     "projects",
}

// describeHistory replaces the ids in the history of an issue with names,
// so that the card says "Status: New → In progress" and not "1 → 2". A value
// that cannot be resolved is left as Redmine gave it.
func describeHistory(db *sql.DB, client *redmine.Client, issueID int, history []redmine.HistoryEntry) {
	// Which ids of which field are mentioned
	ids := map[string]map[int]bool{}
	for _, entry := range history {
		for _, change := range entry.Changes {
			for _, value := range []string{change.Old, change.New} {
				if id, err := strconv.Atoi(value); err == nil {
					if ids[change.Field] == nil {
						ids[change.Field] = map[int]bool{}
					}
					ids[change.Field][id] = true
				}
			}
		}
	}

	names := map[string]map[int]string{}
	for field, table := range historyNameTables {
		if len(ids[field]) > 0 {
			names[field] = namesByID(db, table, ids[field])
		}
	}
	// Categories and versions are not synced: they are asked from the source
	// only when the history mentions them.
	if len(ids["category"]) > 0 || len(ids["fixed_version"]) > 0 {
		var projectID int
		if db.QueryRow("SELECT project_id FROM issues WHERE external_id = $1 AND data_source = 'redmine'", issueID).Scan(&projectID) == nil {
			if len(ids["category"]) > 0 {
				if list, err := client.GetCategories(projectID); err == nil {
					names["category"] = map[int]string{}
					for _, c := range list {
						names["category"][c.ID] = c.Name
					}
				} else {
					slog.Debug("History: categories of the project are not available", "project", projectID, "error", err)
				}
			}
			if len(ids["fixed_version"]) > 0 {
				if versions, err := client.GetVersionNames(projectID); err == nil {
					names["fixed_version"] = versions
				} else {
					slog.Debug("History: versions of the project are not available", "project", projectID, "error", err)
				}
			}
		}
	}

	resolve := func(field, value string) string {
		id, err := strconv.Atoi(value)
		if err != nil {
			return value
		}
		if field == "parent" {
			return "#" + value
		}
		if name, ok := names[field][id]; ok {
			return name
		}
		return value
	}
	for i := range history {
		for j := range history[i].Changes {
			change := &history[i].Changes[j]
			if change.Field == "description" {
				// A description is long; the card only says that it changed
				change.Old, change.New = "", ""
				continue
			}
			change.Old, change.New = resolve(change.Field, change.Old), resolve(change.Field, change.New)
		}
	}
}

// namesByID reads the names of the given ids from a reference table of the
// data source (statuses, priorities, members, projects).
func namesByID(db *sql.DB, table string, ids map[int]bool) map[int]string {
	list := make([]int, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	result := map[int]string{}
	rows, err := db.Query("SELECT external_id, name FROM "+table+" WHERE data_source = 'redmine' AND external_id = ANY($1)", pq.Array(list))
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if rows.Scan(&id, &name) == nil {
			result[id] = name
		}
	}
	return result
}

// describeRelated fills the subject and the status of the related issues
// the user may see. The others keep only their number: that an issue exists
// is visible in the source anyway, what it is about is not shown.
func describeRelated(db *sql.DB, scope *access.Scope, related []redmine.RelatedIssue) {
	if len(related) == 0 {
		return
	}
	ids := make([]int, 0, len(related))
	for _, r := range related {
		ids = append(ids, r.ID)
	}
	args := []interface{}{pq.Array(ids)}
	visible := scope.AllowedIssueCond("", &args)
	rows, err := db.Query(`SELECT external_id, subject, status_name FROM issues
		WHERE external_id = ANY($1) AND data_source = 'redmine' AND `+visible, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	type known struct{ subject, status string }
	found := map[int]known{}
	for rows.Next() {
		var id int
		var k known
		if rows.Scan(&id, &k.subject, &k.status) == nil {
			found[id] = k
		}
	}
	for i := range related {
		if k, ok := found[related[i].ID]; ok {
			related[i].Subject, related[i].Status, related[i].Accessible = k.subject, k.status, true
		}
	}
}
