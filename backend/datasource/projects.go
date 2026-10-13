package datasource

import "database/sql"

// ExpandProjects возвращает заданные проекты вместе со всеми их потомками
// (дочерними, внучатыми, …), прочитанными из таблицы projects.
func ExpandProjects(db *sql.DB, ids []int) ([]int, error) {
	rows, err := db.Query("SELECT external_id, parent_id FROM projects WHERE data_source = 'redmine'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	children := make(map[int][]int)
	for rows.Next() {
		var id int
		var parentID *int
		if err := rows.Scan(&id, &parentID); err != nil {
			return nil, err
		}
		if parentID != nil {
			children[*parentID] = append(children[*parentID], id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return withDescendants(ids, children), nil
}

// withDescendants обходит карту «родитель → дочерние» в ширину. В результате
// сначала идут заданные проекты, повторов нет.
func withDescendants(ids []int, children map[int][]int) []int {
	seen := make(map[int]bool, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	for i := 0; i < len(result); i++ {
		for _, child := range children[result[i]] {
			if !seen[child] {
				seen[child] = true
				result = append(result, child)
			}
		}
	}
	return result
}
