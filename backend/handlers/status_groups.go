package handlers

import "strings"

// Status groups are configured in Настройки → Статусы по типам and stored in
// statuses.group_name. Every tab must classify issues through them, never by
// status names.
const (
	GroupOpen    = "open"
	GroupTesting = "testing"
	GroupClosed  = "closed"
)

// statusIn returns a SQL condition that is true when the status in column col
// belongs to one of the groups. Groups are the constants above, not user input.
func statusIn(col string, groups ...string) string {
	return col + " IN (" + statusGroupSubquery(groups) + ")"
}

// statusNotIn is the opposite of statusIn. A status missing from the statuses
// table (not synced yet) matches, so such issues stay visible as not closed.
func statusNotIn(col string, groups ...string) string {
	return col + " NOT IN (" + statusGroupSubquery(groups) + ")"
}

func statusGroupSubquery(groups []string) string {
	return "SELECT external_id FROM statuses WHERE data_source = 'redmine' AND group_name IN ('" +
		strings.Join(groups, "','") + "')"
}
