package handlers

import (
	"strings"
	"testing"
)

func TestTaskGrouping(t *testing.T) {
	// Без группировки — ни выражения, ни порядка
	if expr, _ := taskGrouping(""); expr != "" {
		t.Errorf("без группировки выражение %q", expr)
	}

	// По статусу: группа — название статуса, без статуса — своя группа
	expr, order := taskGrouping("status")
	if expr != "COALESCE(NULLIF(status_name, ''), 'Без статуса')" {
		t.Errorf("по статусу: выражение %q", expr)
	}
	// Порядок рабочий: открытые, в тестировании, закрытые; внутри — номер статуса
	ranked := strings.Contains(order, "('"+GroupClosed+"')) THEN 2") &&
		strings.Contains(order, "('"+GroupTesting+"')) THEN 1") && strings.Contains(order, "ELSE 0")
	if !ranked || !strings.Contains(order, "MIN(status_id)") {
		t.Errorf("по статусу: порядок %q", order)
	}

	// Исполнитель и проект — по алфавиту, как раньше
	if expr, order := taskGrouping("assignee"); !strings.Contains(expr, "assigned_to_name") || order != "1" {
		t.Errorf("по исполнителю: %q, %q", expr, order)
	}
	if expr, order := taskGrouping("project"); !strings.Contains(expr, "project_name") || order != "1" {
		t.Errorf("по проекту: %q, %q", expr, order)
	}
	// Неизвестное значение — по проекту, как было
	if expr, _ := taskGrouping("bogus"); !strings.Contains(expr, "project_name") {
		t.Errorf("неизвестное значение: %q", expr)
	}
}
