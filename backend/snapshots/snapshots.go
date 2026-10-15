// Package snapshots хранит историю данных: что дашборд знал в каждый из
// дней. Текущее состояние лежит в таблице issues и заменяется каждой
// синхронизацией; снимки — то, что остаётся от прошедших дней.
package snapshots

import (
	"database/sql"
	"fmt"
	"time"
)

// dateLayout — в каком виде день передаётся в базу.
const dateLayout = "2006-01-02"

// classified — задачи с группой их статуса из настроек (open / testing /
// closed, см. statuses.group_name) и отметкой Bugs. Статус, который ещё не
// синхронизирован, считается открытым, как и везде.
const classified = `(
	SELECT i.*, COALESCE(st.group_name, 'open') AS grp, COALESCE(st.is_bug, false) AS is_bug
	FROM issues i
	LEFT JOIN statuses st ON st.external_id = i.status_id AND st.data_source = 'redmine'
	WHERE i.data_source = 'redmine'
)`

// topPriority — самый важный приоритет по порядку из настроек.
const topPriority = `(SELECT external_id FROM priorities WHERE data_source = 'redmine'
	ORDER BY sort_order DESC, external_id DESC LIMIT 1)`

// Counts — сколько строк записал снимок.
type Counts struct {
	Members int64 // строк daily_snapshots за день
	Issues  int64 // строк issue_snapshots добавлено или изменено
}

// Record записывает снимок заданного дня по текущему состоянию задач.
// Вызывается после каждой синхронизации: снимок сегодняшнего дня в течение
// дня следует за данными и остаётся таким, каким день закончился.
func Record(db *sql.DB, day time.Time) (Counts, error) {
	var counts Counts
	date := day.Format(dateLayout)
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())

	members, err := recordDaily(db, date)
	if err != nil {
		return counts, fmt.Errorf("daily snapshot: %w", err)
	}
	counts.Members = members

	issues, err := recordIssues(db, date, dayStart, dayStart.AddDate(0, 0, 1))
	if err != nil {
		return counts, fmt.Errorf("issue snapshot: %w", err)
	}
	counts.Issues = issues
	return counts, nil
}

// recordDaily переписывает итоги дня по сотрудникам. Часы и оценка
// суммируются по незакрытым задачам: они описывают работу, которая есть у
// сотрудника, а не всё, что он когда-либо сделал.
func recordDaily(db *sql.DB, date string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Переписывается целиком: у сотрудника, чью последнюю задачу днём
	// переназначили, не должна остаться утренняя строка.
	if _, err := tx.Exec(`DELETE FROM daily_snapshots WHERE date = $1::date`, date); err != nil {
		return 0, err
	}
	res, err := tx.Exec(`
		INSERT INTO daily_snapshots (
			date, redmine_id, user_login, user_name,
			total_open, total_closed, total_overdue, testing_count, bugs_count,
			high_priority_count, no_estimate_count,
			total_estimated_hours, total_actual_hours, total_bug_hours, bug_percent,
			status_counts, priority_counts
		)
		SELECT $1::date, a.rid, COALESCE(m.login, ''), COALESCE(m.name, a.name, ''),
			a.total_open, a.total_closed, a.total_overdue, a.testing_count, a.bugs_count,
			a.high_priority_count, a.no_estimate_count,
			a.estimated, a.actual, a.bug_hours,
			CASE WHEN a.actual > 0 THEN a.bug_hours / a.actual * 100 ELSE 0 END,
			(SELECT COALESCE(jsonb_object_agg(x.name, x.n), '{}'::jsonb) FROM (
				SELECT COALESCE(NULLIF(i2.status_name, ''), '?') AS name, COUNT(*) AS n
				FROM issues i2
				WHERE i2.data_source = 'redmine' AND COALESCE(i2.assigned_to_id, 0) = a.rid
				GROUP BY 1) x),
			(SELECT COALESCE(jsonb_object_agg(x.name, x.n), '{}'::jsonb) FROM (
				SELECT COALESCE(NULLIF(i2.priority_name, ''), '?') AS name, COUNT(*) AS n
				FROM `+classified+` i2
				WHERE COALESCE(i2.assigned_to_id, 0) = a.rid AND i2.grp <> 'closed'
				GROUP BY 1) x)
		FROM (
			SELECT COALESCE(i.assigned_to_id, 0) AS rid,
				MAX(i.assigned_to_name) AS name,
				COUNT(*) FILTER (WHERE i.grp = 'open') AS total_open,
				COUNT(*) FILTER (WHERE i.grp = 'closed') AS total_closed,
				COUNT(*) FILTER (WHERE i.grp <> 'closed' AND i.due_date < $1::date) AS total_overdue,
				COUNT(*) FILTER (WHERE i.grp = 'testing') AS testing_count,
				COUNT(*) FILTER (WHERE i.is_bug AND i.grp <> 'closed') AS bugs_count,
				COUNT(*) FILTER (WHERE i.grp <> 'closed' AND i.priority_id = `+topPriority+`) AS high_priority_count,
				COUNT(*) FILTER (WHERE i.grp = 'open' AND COALESCE(i.estimated_hours, 0) = 0) AS no_estimate_count,
				COALESCE(SUM(i.estimated_hours) FILTER (WHERE i.grp <> 'closed'), 0)::real AS estimated,
				COALESCE(SUM(i.spent_hours) FILTER (WHERE i.grp <> 'closed'), 0)::real AS actual,
				COALESCE(SUM(i.bug_fix_hours) FILTER (WHERE i.grp <> 'closed'), 0)::real AS bug_hours
			FROM `+classified+` i
			GROUP BY 1
		) a
		LEFT JOIN members m ON m.external_id = a.rid AND m.data_source = 'redmine'`, date)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// recordIssues записывает задачи дня: каждую незакрытую, а закрытую — в
// день закрытия: когда источник сообщает, что она закрыта в этот день,
// когда её предыдущий снимок показывает её открытой или когда она уже есть
// в снимке этого дня. Давно закрытая задача изо дня в день не копируется.
//
// Строка, которая не изменилась бы, не трогается: синхронизация идёт каждые
// несколько минут, и большинство задач остаются прежними.
func recordIssues(db *sql.DB, date string, dayStart, dayEnd time.Time) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO issue_snapshots AS s (
			date, issue_id, redmine_id, user_login, subject, project_id, project_name,
			status, priority, due_date, estimated_hours, is_overdue, is_closed, bug_percent,
			last_updated_on, created_on, sprint_name
		)
		SELECT $1::date, i.external_id, COALESCE(i.assigned_to_id, 0), COALESCE(m.login, ''),
			i.subject, i.project_id, i.project_name,
			i.status_name, i.priority_name, i.due_date, i.estimated_hours::real,
			COALESCE(i.grp <> 'closed' AND i.due_date < $1::date, false),
			i.grp = 'closed',
			CASE WHEN COALESCE(i.spent_hours, 0) > 0
				THEN (COALESCE(i.bug_fix_hours, 0) / i.spent_hours * 100)::real ELSE 0 END,
			i.updated_on, i.created_on, NULLIF(i.fixed_version_name, '')
		FROM `+classified+` i
		LEFT JOIN members m ON m.external_id = i.assigned_to_id AND m.data_source = 'redmine'
		WHERE i.grp <> 'closed'
			OR (i.closed_on >= $2 AND i.closed_on < $3)
			OR EXISTS (SELECT 1 FROM issue_snapshots t WHERE t.date = $1::date AND t.issue_id = i.external_id)
			OR COALESCE((SELECT NOT p.is_closed FROM issue_snapshots p
				WHERE p.issue_id = i.external_id AND p.date < $1::date
				ORDER BY p.date DESC LIMIT 1), false)
		ON CONFLICT (date, issue_id) DO UPDATE SET
			redmine_id = EXCLUDED.redmine_id,
			user_login = EXCLUDED.user_login,
			subject = EXCLUDED.subject,
			project_id = EXCLUDED.project_id,
			project_name = EXCLUDED.project_name,
			status = EXCLUDED.status,
			priority = EXCLUDED.priority,
			due_date = EXCLUDED.due_date,
			estimated_hours = EXCLUDED.estimated_hours,
			is_overdue = EXCLUDED.is_overdue,
			is_closed = EXCLUDED.is_closed,
			bug_percent = EXCLUDED.bug_percent,
			last_updated_on = EXCLUDED.last_updated_on,
			created_on = EXCLUDED.created_on,
			sprint_name = EXCLUDED.sprint_name
		WHERE (s.redmine_id, s.user_login, s.subject, s.project_id, s.project_name, s.status, s.priority,
				s.due_date, s.estimated_hours, s.is_overdue, s.is_closed, s.bug_percent,
				s.last_updated_on, s.created_on, s.sprint_name)
			IS DISTINCT FROM
			(EXCLUDED.redmine_id, EXCLUDED.user_login, EXCLUDED.subject, EXCLUDED.project_id, EXCLUDED.project_name,
				EXCLUDED.status, EXCLUDED.priority, EXCLUDED.due_date, EXCLUDED.estimated_hours, EXCLUDED.is_overdue,
				EXCLUDED.is_closed, EXCLUDED.bug_percent, EXCLUDED.last_updated_on, EXCLUDED.created_on,
				EXCLUDED.sprint_name)`,
		date, dayStart, dayEnd)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// Stats — что хранится: для экрана управления историей.
type Stats struct {
	DailyRows int64   `json:"daily_rows"`
	IssueRows int64   `json:"issue_rows"`
	FirstDate *string `json:"first_date"`
	LastDate  *string `json:"last_date"`
	Days      int     `json:"days"`
}

// GetStats считает сохранённые снимки.
func GetStats(db *sql.DB) (Stats, error) {
	var s Stats
	if err := db.QueryRow(`SELECT COUNT(*) FROM daily_snapshots`).Scan(&s.DailyRows); err != nil {
		return s, err
	}
	err := db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT date), MIN(date)::text, MAX(date)::text FROM issue_snapshots`).
		Scan(&s.IssueRows, &s.Days, &s.FirstDate, &s.LastDate)
	return s, err
}

// Cleanup удаляет снимки старше keepDays дней от сегодняшнего и возвращает
// число удалённых строк. Сам день keepDays назад остаётся.
func Cleanup(db *sql.DB, today time.Time, keepDays int) (int64, error) {
	if keepDays < 1 {
		return 0, fmt.Errorf("retention of %d days", keepDays)
	}
	return deleteBefore(db, Cutoff(today, keepDays))
}

// Cutoff — первый день, который остаётся при заданном сроке хранения.
func Cutoff(today time.Time, keepDays int) string {
	return today.AddDate(0, 0, -keepDays).Format(dateLayout)
}

// Purge удаляет все снимки.
func Purge(db *sql.DB) (int64, error) {
	return deleteBefore(db, "")
}

// deleteBefore удаляет снимки дней до заданного, а при пустой дате — все.
func deleteBefore(db *sql.DB, before string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var total int64
	for _, table := range []string{"daily_snapshots", "issue_snapshots"} {
		var res sql.Result
		if before == "" {
			// #nosec G202 -- имя таблицы из фиксированного списка в коде
			res, err = tx.Exec(`DELETE FROM ` + table)
		} else {
			// #nosec G202 -- имя таблицы из фиксированного списка в коде
			res, err = tx.Exec(`DELETE FROM `+table+` WHERE date < $1::date`, before)
		}
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, tx.Commit()
}
