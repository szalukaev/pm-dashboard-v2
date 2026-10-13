package handlers

import (
	"net/url"
	"strconv"
	"time"
)

// Период отчётов «Аналитики». Он действует на закрытые задачи: в расчёт
// попадают только закрытые в этот период. Открытые задачи и задачи в
// тестировании показываются все — это текущее состояние, от периода оно не
// зависит. Без периода считаются все закрытые задачи, сколько их накоплено.

// maxVelocityWeeks — сколько недель самое большее показывает velocity.
const maxVelocityWeeks = 26

// Period — границы периода; To не входит в период (начало следующего дня).
// nil — граница не задана. Нулевое значение — «за весь период».
type Period struct {
	From *time.Time
	To   *time.Time
}

func (p Period) empty() bool {
	return p.From == nil && p.To == nil
}

// parsePeriod читает период из параметров запроса: period=all (или пусто),
// period=14 или 30 — последние дни, включая сегодня, period=range — с датами
// from и to (ГГГГ-ММ-ДД, обе входят; достаточно одной).
func parsePeriod(q url.Values, now time.Time) (Period, bool) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch mode := q.Get("period"); mode {
	case "", "all":
		return Period{}, true
	case "range":
		var p Period
		if v := q.Get("from"); v != "" {
			t, err := time.ParseInLocation("2006-01-02", v, now.Location())
			if err != nil {
				return Period{}, false
			}
			p.From = &t
		}
		if v := q.Get("to"); v != "" {
			t, err := time.ParseInLocation("2006-01-02", v, now.Location())
			if err != nil {
				return Period{}, false
			}
			next := t.AddDate(0, 0, 1)
			p.To = &next
		}
		if p.From != nil && p.To != nil && !p.From.Before(*p.To) {
			return Period{}, false
		}
		return p, true
	default:
		days, err := strconv.Atoi(mode)
		if err != nil || days < 1 || days > 3660 {
			return Period{}, false
		}
		from := today.AddDate(0, 0, -(days - 1))
		return Period{From: &from}, true
	}
}

// closedCond — добавка к условию отбора задач: закрытая задача остаётся,
// только если закрыта в периоде. Дата закрытия приходит из источника; когда
// её нет, берётся дата последнего изменения. Закрытая задача вовсе без дат
// в ограниченный период не попадает. Без периода возвращает пустую строку.
func (p Period) closedCond(args *[]interface{}) string {
	if p.empty() {
		return ""
	}
	closedAt := "COALESCE(closed_on, updated_on)"
	cond := " AND (" + statusNotIn("status_id", GroupClosed) + " OR (" + closedAt + " IS NOT NULL"
	if p.From != nil {
		*args = append(*args, *p.From)
		cond += " AND " + closedAt + " >= $" + strconv.Itoa(len(*args))
	}
	if p.To != nil {
		*args = append(*args, *p.To)
		cond += " AND " + closedAt + " < $" + strconv.Itoa(len(*args))
	}
	return cond + "))"
}

// velocityWeeksOf — недели, которые показывает velocity: без периода —
// последние 12, с периодом — недели, которые он захватывает, но не больше
// maxVelocityWeeks последних. Старшая первой.
func velocityWeeksOf(p Period, now time.Time) []time.Time {
	if p.empty() {
		return lastWeeks(now, velocityWeeks)
	}
	end := now
	if p.To != nil && p.To.Before(now) {
		// To — начало дня после периода: последняя неделя та, где его последний день
		end = p.To.AddDate(0, 0, -1)
	}
	last := weekStart(end)
	first := last.AddDate(0, 0, -7*(velocityWeeks-1))
	if p.From != nil {
		first = weekStart(*p.From)
	}
	if first.After(last) {
		first = last
	}
	weeks := []time.Time{}
	for w := first; !w.After(last); w = w.AddDate(0, 0, 7) {
		weeks = append(weeks, w)
	}
	if len(weeks) > maxVelocityWeeks {
		weeks = weeks[len(weeks)-maxVelocityWeeks:]
	}
	return weeks
}
