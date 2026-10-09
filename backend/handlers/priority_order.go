package handlers

// The order of priorities is configured in the settings (priorities.sort_order,
// the higher the more important). External ids say nothing about importance.

// priorityRank returns a SQL expression with the position of the priority in
// column col; sort by it DESC to put the most important issues first.
func priorityRank(col string) string {
	return "(SELECT sort_order FROM priorities WHERE external_id = " + col + " AND data_source = 'redmine')"
}

// highPriority returns a SQL condition that is true when the priority in
// column col is the most important one.
func highPriority(col string) string {
	return col + " = (SELECT external_id FROM priorities WHERE data_source = 'redmine' ORDER BY sort_order DESC, external_id DESC LIMIT 1)"
}
