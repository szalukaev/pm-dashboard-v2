package access

// SprintVisibleTo сообщает, виден ли спринт пользователю. Свой спринт виден
// всегда. Чужой — только общий и только тому, кто связан с сотрудником
// источника, входящим в выбранную команду создателя; если создатель команду
// не выбирал (ownerTeam пуст), его команда — все.
func SprintVisibleTo(viewerID int, viewerMember *int, ownerID int, shared bool, ownerTeam []int) bool {
	if ownerID == viewerID {
		return true
	}
	if !shared || viewerMember == nil {
		return false
	}
	return len(ownerTeam) == 0 || containsInt(ownerTeam, *viewerMember)
}

// SeesIssue сообщает, вправе ли пользователь видеть задачу проекта projectID
// с ответственным assignee (nil — без ответственного). Это то же правило,
// что AllowedIssueCond записывает условием SQL: всё, что разрешил
// администратор, без оглядки на собственный выбор пользователя.
func (s *Scope) SeesIssue(projectID int, assignee *int) bool {
	if s.OwnMember != nil && assignee != nil && *assignee == *s.OwnMember {
		return true
	}
	if !s.AllowedAllProjects && !containsInt(s.AllowedProjects, projectID) {
		return false
	}
	// Задача без ответственного не входит ни в чью команду: её видят только
	// те, кому дано такое право.
	if assignee == nil {
		return s.ShowUnassigned
	}
	return s.AllowedAllTeam || containsInt(s.AllowedTeam, *assignee)
}
