package access

import (
	"database/sql"
	"strconv"
)

// Scan reads a permission set from a row whose SELECT lists the leading
// columns first and Columns after them. scan is row.Scan or rows.Scan.
func Scan(scan func(dest ...interface{}) error, leading ...interface{}) (Permissions, error) {
	var raw rawPermissions
	if err := scan(append(leading, raw.dest()...)...); err != nil {
		return Permissions{}, err
	}
	return raw.permissions(), nil
}

// assignments returns "col = $n, …" for Columns, placeholders starting at first.
func assignments(first int) string {
	names := []string{"project_ids", "team_ids", "visible_tabs", "widgets", "all_projects", "all_team", "own_tasks_only", "read_only", "from_source", "show_unassigned"}
	result := ""
	for i, name := range names {
		if i > 0 {
			result += ", "
		}
		result += name + " = $" + strconv.Itoa(first+i)
	}
	return result
}

// Save writes the permission set of a user ("user_permissions", "user_id")
// or a group ("group_permissions", "group_id"), replacing the previous one.
func Save(db *sql.DB, table, keyColumn string, id int, p Permissions) error {
	args := append([]interface{}{id}, p.Values()...)
	_, err := db.Exec(`INSERT INTO `+table+` (`+keyColumn+`, `+Columns+`)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (`+keyColumn+`) DO UPDATE SET `+assignments(2), args...)
	return err
}

// SaveTemplate writes the permission set of a role template.
func SaveTemplate(db *sql.DB, templateID int, p Permissions) error {
	args := append([]interface{}{templateID}, p.Values()...)
	_, err := db.Exec(`UPDATE role_templates SET `+assignments(2)+` WHERE id = $1`, args...)
	return err
}
