package access

import "testing"

func TestCanKanban(t *testing.T) {
	// Без настройки доступны оба вида доски
	open := &Scope{}
	if !open.CanKanban(KanbanUsers) || !open.CanKanban(KanbanStatuses) {
		t.Error("no setting: both board kinds must be allowed")
	}

	usersOnly := []string{KanbanUsers}
	limited := &Scope{KanbanModes: &usersOnly}
	if !limited.CanKanban(KanbanUsers) || limited.CanKanban(KanbanStatuses) {
		t.Error("only the board by members must be allowed")
	}

	// Администратора настройка не ограничивает
	admin := &Scope{Admin: true, KanbanModes: &usersOnly}
	if !admin.CanKanban(KanbanStatuses) {
		t.Error("an administrator is never limited")
	}
}

func TestKanbanModesNormalized(t *testing.T) {
	// Неизвестный вид доски отбрасывается, «всё» остаётся «всем»
	keys := []string{KanbanStatuses, "bogus"}
	p := Permissions{KanbanModes: &keys}.Normalized()
	if p.KanbanModes == nil || len(*p.KanbanModes) != 1 || (*p.KanbanModes)[0] != KanbanStatuses {
		t.Errorf("normalized = %v", p.KanbanModes)
	}
	if (Permissions{}).Normalized().KanbanModes != nil {
		t.Error("nil must stay nil: every board kind")
	}
}
