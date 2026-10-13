package redmine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

// DefaultFullSyncTime — когда начинается ежедневный полный проход, если не
// задано иное: ночью, когда свежих данных никто не ждёт.
const DefaultFullSyncTime = "03:00"

const (
	// fullSyncInterval: как часто проект перечитывается из Redmine целиком.
	// В промежутке забираются только изменения; полный проход находит то,
	// чего по изменениям не видно: удалённые задачи, удалённые трудозатраты
	// и время, списанное на давние даты.
	fullSyncInterval = 24 * time.Hour
	// syncOverlap расширяет окно изменений на случай расхождения часов
	// этого сервера и Redmine.
	syncOverlap = 10 * time.Minute
	// fullSyncWorkers — сколько проектов читается целиком одновременно.
	fullSyncWorkers = 3
	// emptyAnswerGuard: проект, у которого здесь задач больше этого числа,
	// а в ответе Redmine ни одной, похож на сбой, а не на правду; по такому
	// ответу ничего не удаляется.
	emptyAnswerGuard = 50
	// issueBatchSize — строк в одном INSERT: по 22 параметра, много меньше предела 65535.
	issueBatchSize = 500
	// timeEntryBatchSize — строк в одном INSERT: по 5 параметров.
	timeEntryBatchSize = 1000
)

// projectState — когда проект читался в последний раз: любая успешная
// загрузка и последняя полная.
type projectState struct {
	lastSync time.Time
	lastFull time.Time
}

// syncStore — где синхронизация хранит задачи и своё состояние: база
// (pgStore) или заглушка в тестах.
type syncStore interface {
	loadStates() (map[int]projectState, error)
	saveState(projectID int, state projectState) error
	storeTimeEntries(projectID int, entries []TimeEntry, replace bool) error
	upsertIssues(batch []Issue) error
	updateIssueHours(issueIDs []int64) error
	// countIssues: сколько задач проекта хранится.
	countIssues(projectID int) (int, error)
	// deleteMissing удаляет задачи проекта, которых нет в keep, и
	// возвращает их номера.
	deleteMissing(projectID int, keep []int) ([]int, error)
	// deleteIssues удаляет заданные задачи, где бы они ни были.
	deleteIssues(issueIDs []int) error
	cleanupUnselected(projectIDs []int) error
}

// syncStats — что сделала синхронизация задач.
type syncStats struct {
	issues   int // записано задач
	deleted  int // удалено задач, пропавших из источника
	projects int // проектов нужно было синхронизировать
	failed   int // проектов не удалось прочитать
}

// ParseTimeOfDay разбирает "HH:MM".
func ParseTimeOfDay(value string) (hour, minute int, ok bool) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, false
	}
	return t.Hour(), t.Minute(), true
}

// lastScheduled возвращает последний момент не позже now, когда часы
// показывали заданное время суток.
func lastScheduled(now time.Time, timeOfDay string) time.Time {
	hour, minute, ok := ParseTimeOfDay(timeOfDay)
	if !ok {
		hour, minute, _ = ParseTimeOfDay(DefaultFullSyncTime)
	}
	at := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if at.After(now) {
		at = at.AddDate(0, 0, -1)
	}
	return at
}

// needsFull сообщает, нужно ли перечитать проект целиком: он не читался ни
// разу либо его последнее полное чтение старше последнего времени полного
// прохода по расписанию. Без расписания полный проход нужен через сутки
// после предыдущего.
func needsFull(state projectState, known bool, now time.Time, fullTime string) bool {
	if !known {
		return true
	}
	if fullTime == "" {
		return now.Sub(state.lastFull) >= fullSyncInterval
	}
	return state.lastFull.Before(lastScheduled(now, fullTime))
}

// fullLoad — всё, что есть в Redmine по одному проекту.
type fullLoad struct {
	projectID int
	issues    []Issue
	entries   []TimeEntry
	// noTimeAccess: ключу API нельзя читать трудозатраты проекта.
	noTimeAccess bool
	err          error
}

// syncProjectIssues загружает задачи и трудозатраты заданных проектов.
//
// Проект, который не читался ни разу или был прочитан целиком больше
// fullSyncInterval назад, читается целиком; задачи, которых в Redmine больше
// нет, при этом удаляются. По остальным проектам забираются только задачи,
// изменённые с прошлой синхронизации, и списанное с тех пор время. Затем
// фактические часы и часы на исправление багов пересчитываются по
// сохранённым трудозатратам затронутых задач.
//
// Состояние хранится по каждому проекту: проект со сбоем читается заново в
// следующий раз и не заставляет перечитывать остальные.
func (s *Syncer) syncProjectIssues(ctx context.Context, store syncStore, projectIDs []int, started time.Time, fullTime string) (syncStats, error) {
	var stats syncStats
	states, err := store.loadStates()
	if err != nil {
		return stats, fmt.Errorf("load sync state: %w", err)
	}
	forceFull := fullSyncRequested(ctx)

	selected := make(map[int]bool, len(projectIDs))
	var fullIDs, quickIDs []int
	for _, pid := range projectIDs {
		if selected[pid] {
			continue
		}
		selected[pid] = true
		if st, known := states[pid]; forceFull || needsFull(st, known, started, fullTime) {
			fullIDs = append(fullIDs, pid)
		} else {
			quickIDs = append(quickIDs, pid)
		}
	}
	stats.projects = len(selected)

	var issues []Issue
	seenIssues := make(map[int]bool)
	touched := make(map[int]bool)
	addIssues := func(list []Issue) {
		for _, iss := range list {
			if !seenIssues[iss.ExternalID] {
				seenIssues[iss.ExternalID] = true
				touched[iss.ExternalID] = true
				issues = append(issues, iss)
			}
		}
	}
	failed := 0

	// Изменения актуальных проектов: два запроса на все сразу
	quickDone, movedOut := s.loadChanges(store, quickIDs, selected, states, addIssues, touched)
	failed += len(quickIDs) - len(quickDone)

	// Полные загрузки, по несколько проектов одновременно
	loads := s.loadFull(ctx, fullIDs)
	// Остановленная синхронизация не сохраняет то, на чём её прервали:
	// проекты читаются заново в следующий раз.
	if ctx.Err() != nil {
		return stats, ctx.Err()
	}
	var complete []fullLoad
	for _, load := range loads {
		if load.err != nil {
			slog.Warn("Failed to load project from Redmine", "project_id", load.projectID, "error", load.err)
			failed++
			continue
		}
		if !load.noTimeAccess {
			if err := store.storeTimeEntries(load.projectID, load.entries, true); err != nil {
				slog.Warn("Failed to store time entries for project", "project_id", load.projectID, "error", err)
				failed++
				continue
			}
		}
		for _, e := range load.entries {
			touched[e.IssueID] = true
		}
		addIssues(load.issues)
		complete = append(complete, load)
		slog.Info("Loaded project from Redmine", "project_id", load.projectID, "full", true,
			"issues", len(load.issues), "time_entries", len(load.entries))
	}

	for start := 0; start < len(issues); start += issueBatchSize {
		end := min(start+issueBatchSize, len(issues))
		if err := store.upsertIssues(issues[start:end]); err != nil {
			return stats, fmt.Errorf("upsert issues: %w", err)
		}
	}

	// Задачи, пропавшие из Redmine. После записи: задача, перенесённая в
	// другой синхронизируемый проект, уже принадлежит ему и здесь не затрагивается.
	deleted := 0
	for _, load := range complete {
		deleted += deleteGone(store, load)
	}
	if len(movedOut) > 0 {
		// Перенесены в проект, который не синхронизируется
		if err := store.deleteIssues(movedOut); err != nil {
			slog.Warn("Failed to delete issues moved out of the synced projects", "error", err)
		}
	}

	ids := make([]int64, 0, len(touched))
	for id := range touched {
		ids = append(ids, int64(id))
	}
	if err := store.updateIssueHours(ids); err != nil {
		return stats, fmt.Errorf("update issue hours: %w", err)
	}

	// Всё сохранено: у прочитанных проектов сдвигаются отметки
	for _, load := range complete {
		if err := store.saveState(load.projectID, projectState{lastSync: started, lastFull: started}); err != nil {
			slog.Warn("Failed to save sync state", "project_id", load.projectID, "error", err)
		}
	}
	for pid := range quickDone {
		if err := store.saveState(pid, projectState{lastSync: started, lastFull: states[pid].lastFull}); err != nil {
			slog.Warn("Failed to save sync state", "project_id", pid, "error", err)
		}
	}

	slog.Info("Synced issues", "issues", len(issues), "recalculated", len(ids), "deleted", deleted,
		"projects_full", len(complete), "projects_changes", len(quickDone), "failed", failed,
		"duration", time.Since(started))

	if err := store.cleanupUnselected(projectIDs); err != nil {
		slog.Warn("Failed to cleanup unselected projects", "error", err)
	}
	stats.issues, stats.deleted, stats.failed = len(issues), deleted, failed
	return stats, nil
}

// loadChanges забирает то, что изменилось в заданных проектах с их прошлой
// синхронизации, и сохраняет трудозатраты. Возвращает проекты, изменения
// которых прочитаны (остальные нельзя считать синхронизированными), и
// изменённые задачи, которые теперь принадлежат проекту, не
// синхронизируемому вовсе.
//
// Изменённые задачи всех проектов приходят одним запросом. Время
// запрашивается по каждому проекту: на все сразу Redmine отвечает слишком
// долго.
func (s *Syncer) loadChanges(store syncStore, projectIDs []int, selected map[int]bool, states map[int]projectState,
	addIssues func([]Issue), touched map[int]bool) (done map[int]bool, movedOut []int) {
	done = make(map[int]bool, len(projectIDs))
	if len(projectIDs) == 0 {
		return done, nil
	}
	quick := make(map[int]bool, len(projectIDs))
	since := states[projectIDs[0]].lastSync
	for _, pid := range projectIDs {
		quick[pid] = true
		if states[pid].lastSync.Before(since) {
			since = states[pid].lastSync
		}
	}
	since = since.Add(-syncOverlap)

	changed, err := s.client.GetIssues(0, &since)
	if err != nil {
		slog.Warn("Failed to get changed issues", "error", err)
		return done, nil
	}

	// Записи отбираются по дню, на который списано время; лишний день
	// покрывает время, списанное на вчера.
	from := since.AddDate(0, 0, -1).Format("2006-01-02")
	type recent struct {
		entries []TimeEntry
		err     error
	}
	loaded := make([]recent, len(projectIDs))
	var wg sync.WaitGroup
	slots := make(chan struct{}, maxParallelLists)
	for i, pid := range projectIDs {
		wg.Add(1)
		slots <- struct{}{}
		go func(i, pid int) {
			defer wg.Done()
			defer func() { <-slots }()
			entries, err := s.client.GetTimeEntries(pid, from)
			if errors.Is(err, ErrForbidden) {
				// Права на время этого проекта нет: его задачи
				// синхронизируются без часов, см. loadProject
				err = nil
			}
			loaded[i] = recent{entries: entries, err: err}
		}(i, pid)
	}
	wg.Wait()

	total := 0
	for i, pid := range projectIDs {
		if loaded[i].err != nil {
			slog.Warn("Failed to get recent time entries for project", "project_id", pid, "error", loaded[i].err)
			continue
		}
		if len(loaded[i].entries) > 0 {
			if err := store.storeTimeEntries(pid, loaded[i].entries, false); err != nil {
				slog.Warn("Failed to store time entries for project", "project_id", pid, "error", err)
				continue
			}
			for _, e := range loaded[i].entries {
				touched[e.IssueID] = true
			}
			total += len(loaded[i].entries)
		}
		done[pid] = true
	}

	var mine []Issue
	for _, iss := range changed {
		switch {
		case done[iss.ProjectID]:
			mine = append(mine, iss)
		case !selected[iss.ProjectID]:
			movedOut = append(movedOut, iss.ExternalID)
		}
		// Задача проекта, который читается целиком, придёт вместе с ним;
		// задача проекта со сбоем прочитается в следующий раз
	}
	addIssues(mine)
	slog.Info("Loaded changes from Redmine", "projects", len(done), "since", since.Format(time.RFC3339),
		"issues", len(mine), "time_entries", total)
	return done, movedOut
}

// loadFull читает заданные проекты целиком, по fullSyncWorkers одновременно.
func (s *Syncer) loadFull(ctx context.Context, projectIDs []int) []fullLoad {
	loads := make([]fullLoad, len(projectIDs))
	var wg sync.WaitGroup
	slots := make(chan struct{}, fullSyncWorkers)
	for i, pid := range projectIDs {
		wg.Add(1)
		slots <- struct{}{}
		go func(i, pid int) {
			defer wg.Done()
			defer func() { <-slots }()
			if ctx.Err() != nil {
				loads[i] = fullLoad{projectID: pid, err: ctx.Err()}
				return
			}
			loads[i] = s.loadProject(pid)
		}(i, pid)
	}
	wg.Wait()
	return loads
}

func (s *Syncer) loadProject(pid int) fullLoad {
	load := fullLoad{projectID: pid}
	entries, err := s.client.GetTimeEntries(pid, "")
	switch {
	case errors.Is(err, ErrForbidden):
		// У ключа нет права на время этого проекта. Это не сбой и само не
		// пройдёт: задачи синхронизируются без часов.
		slog.Warn("The API key has no access to the time entries of a project: its issues are synced without hours",
			"project_id", pid)
		load.noTimeAccess = true
	case err != nil:
		load.err = fmt.Errorf("time entries: %w", err)
		return load
	}
	load.entries = entries

	load.issues, err = s.client.GetIssues(pid, nil)
	if err != nil {
		load.err = fmt.Errorf("issues: %w", err)
	}
	return load
}

// deleteGone удаляет задачи целиком прочитанного проекта, которых в Redmine
// больше нет, и возвращает их число.
func deleteGone(store syncStore, load fullLoad) int {
	if len(load.issues) == 0 {
		if stored, err := store.countIssues(load.projectID); err != nil || stored > emptyAnswerGuard {
			slog.Warn("Redmine returned no issues for a project that has many here: nothing is deleted",
				"project_id", load.projectID, "stored", stored, "error", err)
			return 0
		}
	}
	keep := make([]int, len(load.issues))
	for i, iss := range load.issues {
		keep[i] = iss.ExternalID
	}
	gone, err := store.deleteMissing(load.projectID, keep)
	if err != nil {
		slog.Warn("Failed to delete issues that are gone from Redmine", "project_id", load.projectID, "error", err)
		return 0
	}
	if len(gone) > 0 {
		slog.Info("Deleted issues that are gone from Redmine", "project_id", load.projectID, "count", len(gone), "issues", gone)
	}
	return len(gone)
}

// pgStore хранит синхронизированные данные в PostgreSQL.
type pgStore struct {
	db *sql.DB
}

func (p pgStore) loadStates() (map[int]projectState, error) {
	rows, err := p.db.Query(`SELECT project_id, last_sync_at, last_full_at FROM sync_state WHERE data_source = 'redmine'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := make(map[int]projectState)
	for rows.Next() {
		var pid int
		var st projectState
		if err := rows.Scan(&pid, &st.lastSync, &st.lastFull); err != nil {
			return nil, err
		}
		states[pid] = st
	}
	return states, rows.Err()
}

func (p pgStore) saveState(projectID int, state projectState) error {
	_, err := p.db.Exec(`INSERT INTO sync_state (project_id, data_source, last_sync_at, last_full_at)
		VALUES ($1, 'redmine', $2, $3)
		ON CONFLICT (project_id, data_source) DO UPDATE SET
			last_sync_at = EXCLUDED.last_sync_at, last_full_at = EXCLUDED.last_full_at`,
		projectID, state.lastSync, state.lastFull)
	return err
}

func (p pgStore) storeTimeEntries(projectID int, entries []TimeEntry, replace bool) error {
	return storeTimeEntries(p.db, projectID, entries, replace)
}

func (p pgStore) upsertIssues(batch []Issue) error { return upsertIssues(p.db, batch) }

func (p pgStore) updateIssueHours(issueIDs []int64) error { return updateIssueHours(p.db, issueIDs) }

func (p pgStore) countIssues(projectID int) (int, error) {
	var n int
	err := p.db.QueryRow(`SELECT COUNT(*) FROM issues WHERE data_source = 'redmine' AND project_id = $1`, projectID).Scan(&n)
	return n, err
}

func (p pgStore) deleteMissing(projectID int, keep []int) ([]int, error) {
	rows, err := p.db.Query(`SELECT external_id FROM issues
		WHERE data_source = 'redmine' AND project_id = $1 AND NOT (external_id = ANY($2::int[]))`,
		projectID, pq.Array(keep))
	if err != nil {
		return nil, err
	}
	var gone []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		gone = append(gone, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(gone) == 0 {
		return nil, err
	}
	return gone, p.deleteIssues(gone)
}

// deleteIssues удаляет задачи вместе с тем, что существует только ради них:
// их трудозатратами и местами в спринтах. История остаётся в снимках.
func (p pgStore) deleteIssues(issueIDs []int) error {
	if len(issueIDs) == 0 {
		return nil
	}
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids := pq.Array(issueIDs)
	for _, query := range []string{
		`DELETE FROM time_entries WHERE data_source = 'redmine' AND issue_id = ANY($1::int[])`,
		`DELETE FROM sprint_issues WHERE issue_external_id = ANY($1::int[])`,
		`DELETE FROM issues WHERE data_source = 'redmine' AND external_id = ANY($1::int[])`,
	} {
		if _, err := tx.Exec(query, ids); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p pgStore) cleanupUnselected(projectIDs []int) error {
	if err := cleanupUnselected(p.db, projectIDs); err != nil {
		return err
	}
	// Проект, выбранный позже снова, начинает с полной загрузки
	_, err := p.db.Exec(`DELETE FROM sync_state WHERE data_source = 'redmine' AND NOT (project_id = ANY($1::int[]))`,
		pq.Array(projectIDs))
	return err
}

// storeTimeEntries сохраняет трудозатраты проекта. Полная загрузка заменяет
// записи проекта, поэтому удалённые в Redmine записи пропадают и здесь.
func storeTimeEntries(db *sql.DB, projectID int, entries []TimeEntry, replace bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if replace {
		if _, err := tx.Exec(`DELETE FROM time_entries WHERE project_id = $1 AND data_source = 'redmine'`, projectID); err != nil {
			return err
		}
	}

	// Родительский проект всё же может вернуть записи подпроекта: каждый id берётся один раз.
	seen := make(map[int]bool, len(entries))
	unique := make([]TimeEntry, 0, len(entries))
	for _, e := range entries {
		if !seen[e.ID] {
			seen[e.ID] = true
			unique = append(unique, e)
		}
	}

	const cols = 5
	for start := 0; start < len(unique); start += timeEntryBatchSize {
		batch := unique[start:min(start+timeEntryBatchSize, len(unique))]
		values := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*cols)
		for i, e := range batch {
			n := i * cols
			values[i] = fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,'redmine',NOW())", n+1, n+2, n+3, n+4, n+5)
			args = append(args, e.ID, e.IssueID, projectID, e.ActivityName, e.Hours)
		}
		_, err := tx.Exec(`
			INSERT INTO time_entries (external_id, issue_id, project_id, activity_name, hours, data_source, synced_at)
			VALUES `+strings.Join(values, ",")+`
			ON CONFLICT (external_id, data_source) DO UPDATE SET
				issue_id = EXCLUDED.issue_id,
				project_id = EXCLUDED.project_id,
				activity_name = EXCLUDED.activity_name,
				hours = EXCLUDED.hours,
				synced_at = NOW()`, args...)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// upsertIssues записывает пачку задач одним многострочным запросом: база
// удалённая, и запрос на каждую задачу делал синхронизацию медленной.
// Фактические часы и часы на исправление багов ведёт updateIssueHours.
func upsertIssues(db *sql.DB, batch []Issue) error {
	const cols = 22
	values := make([]string, len(batch))
	args := make([]interface{}, 0, len(batch)*cols)
	for i, it := range batch {
		ph := make([]string, cols)
		for j := range ph {
			ph[j] = fmt.Sprintf("$%d", i*cols+j+1)
		}
		values[i] = "(" + strings.Join(ph, ",") + ",0,0,'redmine',NOW())"
		args = append(args, it.ExternalID, it.ProjectID, it.ProjectName, it.Subject, it.Description,
			it.StatusName, it.StatusID, it.PriorityName, it.PriorityID,
			it.AssignedToName, it.AssignedToID, it.CategoryName,
			it.StartDate, it.DueDate, it.EstimatedHours,
			it.DoneRatio, it.TrackerName, it.AuthorName,
			it.CreatedOn, it.UpdatedOn, it.ClosedOn, it.FixedVersionName)
	}
	_, err := db.Exec(`
		INSERT INTO issues (
			external_id, project_id, project_name, subject, description,
			status_name, status_id, priority_name, priority_id,
			assigned_to_name, assigned_to_id, category_name,
			start_date, due_date, estimated_hours,
			done_ratio, tracker_name, author_name,
			created_on, updated_on, closed_on, fixed_version_name,
			spent_hours, bug_fix_hours, data_source, synced_at
		) VALUES `+strings.Join(values, ",")+`
		ON CONFLICT (external_id, data_source) DO UPDATE SET
			project_id = EXCLUDED.project_id,
			project_name = EXCLUDED.project_name,
			subject = EXCLUDED.subject,
			description = EXCLUDED.description,
			status_name = EXCLUDED.status_name,
			status_id = EXCLUDED.status_id,
			priority_name = EXCLUDED.priority_name,
			priority_id = EXCLUDED.priority_id,
			assigned_to_name = EXCLUDED.assigned_to_name,
			assigned_to_id = EXCLUDED.assigned_to_id,
			category_name = EXCLUDED.category_name,
			start_date = EXCLUDED.start_date,
			due_date = EXCLUDED.due_date,
			estimated_hours = EXCLUDED.estimated_hours,
			done_ratio = EXCLUDED.done_ratio,
			tracker_name = EXCLUDED.tracker_name,
			author_name = EXCLUDED.author_name,
			created_on = EXCLUDED.created_on,
			updated_on = EXCLUDED.updated_on,
			closed_on = EXCLUDED.closed_on,
			fixed_version_name = EXCLUDED.fixed_version_name,
			synced_at = NOW()`, args...)
	return err
}

// updateIssueHours пересчитывает фактические часы (все трудозатраты) и часы
// на исправление багов (деятельность «Fixing bugs») заданных задач по
// сохранённым трудозатратам.
func updateIssueHours(db *sql.DB, issueIDs []int64) error {
	if len(issueIDs) == 0 {
		return nil
	}
	_, err := db.Exec(`
		UPDATE issues i SET
			spent_hours = t.spent,
			bug_fix_hours = t.bug_fix
		FROM (
			SELECT x.id,
				COALESCE(SUM(te.hours), 0) AS spent,
				COALESCE(SUM(te.hours) FILTER (WHERE LOWER(TRIM(te.activity_name)) = 'fixing bugs'), 0) AS bug_fix
			FROM unnest($1::int[]) AS x(id)
			LEFT JOIN time_entries te ON te.issue_id = x.id AND te.data_source = 'redmine'
			GROUP BY x.id
		) t
		WHERE i.external_id = t.id AND i.data_source = 'redmine'`, pq.Array(issueIDs))
	return err
}

// cleanupUnselected удаляет задачи и трудозатраты проектов, которые больше
// не выбраны ни одним пользователем.
func cleanupUnselected(db *sql.DB, projectIDs []int) error {
	ids := make([]int64, len(projectIDs))
	for i, pid := range projectIDs {
		ids[i] = int64(pid)
	}
	res, err := db.Exec(`DELETE FROM issues WHERE data_source = 'redmine' AND NOT (project_id = ANY($1::int[]))`, pq.Array(ids))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		slog.Info("Cleaned up issues from unselected projects", "deleted", n)
	}
	_, err = db.Exec(`DELETE FROM time_entries WHERE data_source = 'redmine' AND NOT (project_id = ANY($1::int[]))`, pq.Array(ids))
	return err
}
