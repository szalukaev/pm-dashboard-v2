package access

import (
	"database/sql"
	"strconv"
	"strings"
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

// columnNames возвращает столбцы прав по одному, в порядке Columns.
func columnNames() []string {
	return strings.Split(Columns, ", ")
}

// assignments возвращает "col = $n, …" для Columns, номера параметров — с first.
func assignments(first int) string {
	result := ""
	for i, name := range columnNames() {
		if i > 0 {
			result += ", "
		}
		result += name + " = $" + strconv.Itoa(first+i)
	}
	return result
}

// placeholders возвращает "$first, …" — count номеров параметров подряд.
func placeholders(first, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		if i > 0 {
			result += ", "
		}
		result += "$" + strconv.Itoa(first+i)
	}
	return result
}

// saveQuery собирает запрос записи набора прав: $1 — ключ, дальше столбцы
// Columns. Число параметров считается по Columns, чтобы новый столбец прав
// не разошёлся со списком VALUES.
func saveQuery(table, keyColumn string) string {
	return `INSERT INTO ` + table + ` (` + keyColumn + `, ` + Columns + `)
		VALUES (` + placeholders(1, len(columnNames())+1) + `)
		ON CONFLICT (` + keyColumn + `) DO UPDATE SET ` + assignments(2)
}

// Save writes the permission set of a user ("user_permissions", "user_id")
// or a group ("group_permissions", "group_id"), replacing the previous one.
func Save(db *sql.DB, table, keyColumn string, id int, p Permissions) error {
	args := append([]interface{}{id}, p.Values()...)
	_, err := db.Exec(saveQuery(table, keyColumn), args...)
	return err
}

// SaveTemplate writes the permission set of a role template.
func SaveTemplate(db *sql.DB, templateID int, p Permissions) error {
	args := append([]interface{}{templateID}, p.Values()...)
	_, err := db.Exec(`UPDATE role_templates SET `+assignments(2)+` WHERE id = $1`, args...)
	return err
}
