package handlers

import "strings"

// Группы статусов задаются в «Настройки → Статусы по типам» и хранятся в
// statuses.group_name. Каждая вкладка относит задачи к группам только через
// них, а не по названиям статусов.
const (
	GroupOpen    = "open"
	GroupTesting = "testing"
	GroupClosed  = "closed"
)

// statusIn возвращает условие SQL, истинное, когда статус в столбце col
// входит в одну из групп. Группы — константы выше, а не ввод пользователя.
func statusIn(col string, groups ...string) string {
	return col + " IN (" + statusGroupSubquery(groups) + ")"
}

// statusNotIn — обратное statusIn. Статус, которого нет в таблице statuses
// (ещё не синхронизирован), подходит: такие задачи остаются видны как
// незакрытые.
func statusNotIn(col string, groups ...string) string {
	return col + " NOT IN (" + statusGroupSubquery(groups) + ")"
}

// bugStatus истинно для статусов, отмеченных в настройках как Bugs. Отметка
// не зависит от группы: статус-баг обычно ещё и открытый.
func bugStatus(col string) string {
	return col + " IN (SELECT external_id FROM statuses WHERE data_source = 'redmine' AND is_bug)"
}

func statusGroupSubquery(groups []string) string {
	return "SELECT external_id FROM statuses WHERE data_source = 'redmine' AND group_name IN ('" +
		strings.Join(groups, "','") + "')"
}
