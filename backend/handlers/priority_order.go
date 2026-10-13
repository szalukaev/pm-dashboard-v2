package handlers

// Порядок приоритетов задаётся в настройках (priorities.sort_order: чем
// больше, тем важнее). Внешние идентификаторы о важности ничего не говорят.

// priorityRank возвращает выражение SQL с местом приоритета из столбца col;
// сортировка по нему DESC ставит самые важные задачи первыми.
func priorityRank(col string) string {
	return "(SELECT sort_order FROM priorities WHERE external_id = " + col + " AND data_source = 'redmine')"
}

// highPriority возвращает условие SQL, истинное, когда приоритет в столбце
// col — самый важный.
func highPriority(col string) string {
	return col + " = (SELECT external_id FROM priorities WHERE data_source = 'redmine' ORDER BY sort_order DESC, external_id DESC LIMIT 1)"
}
