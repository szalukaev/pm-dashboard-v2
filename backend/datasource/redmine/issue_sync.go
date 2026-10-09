package redmine

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

const (
	// fullSyncInterval: how often a project is re-read from Redmine
	// completely. In between only changes are fetched; the full pass picks
	// up what the changes cannot show: deleted issues, deleted time entries
	// and time logged for long-past dates.
	fullSyncInterval = 24 * time.Hour
	// syncOverlap widens the incremental window against clock skew between
	// this server and Redmine.
	syncOverlap = 10 * time.Minute
	// fullSyncWorkers projects are read completely at the same time.
	fullSyncWorkers = 3
	// emptyAnswerGuard: a project that has more issues than this here and
	// none in the answer of Redmine looks like a failure, not like the truth;
	// nothing is deleted on such an answer.
	emptyAnswerGuard = 50
	// issueBatchSize rows per INSERT: 18 params each, well below the 65535 limit.
	issueBatchSize = 500
	// timeEntryBatchSize rows per INSERT: 5 params each.
	timeEntryBatchSize = 1000
)

// projectState is when a project was last read: any successful load and the
// last complete one.
type projectState struct {
	lastSync time.Time
	lastFull time.Time
}

// syncStore is where a sync keeps the issues and its own state: the
// database (pgStore), or a stand-in in tests.
type syncStore interface {
	loadStates() (map[int]projectState, error)
	saveState(projectID int, state projectState) error
	storeTimeEntries(projectID int, entries []TimeEntry, replace bool) error
	upsertIssues(batch []Issue) error
	updateIssueHours(issueIDs []int64) error
	// countIssues: how many issues of the project are stored.
	countIssues(projectID int) (int, error)
	// deleteMissing removes the issues of the project that are not in keep
	// and returns their numbers.
	deleteMissing(projectID int, keep []int) ([]int, error)
	// deleteIssues removes the given issues wherever they are.
	deleteIssues(issueIDs []int) error
	cleanupUnselected(projectIDs []int) error
}

// fullLoad is everything Redmine has for one project.
type fullLoad struct {
	projectID int
	issues    []Issue
	entries   []TimeEntry
	// noTimeAccess: the API key may not read the time entries of the project.
	noTimeAccess bool
	err          error
}

// syncProjectIssues loads issues and time entries of the given projects.
//
// A project that was never read, or was last read completely more than
// fullSyncInterval ago, is read completely; issues that are no longer in
// Redmine are deleted then. For the other projects only the issues updated
// and the time spent since their last sync are fetched. Fact and bug fix hours are then recomputed from the stored time
// entries of the affected issues.
//
// The state is kept per project: a project that failed is read again next
// time and does not make the others be re-read.
func (s *Syncer) syncProjectIssues(store syncStore, projectIDs []int, started time.Time) error {
	states, err := store.loadStates()
	if err != nil {
		return fmt.Errorf("load sync state: %w", err)
	}

	selected := make(map[int]bool, len(projectIDs))
	var fullIDs, quickIDs []int
	for _, pid := range projectIDs {
		if selected[pid] {
			continue
		}
		selected[pid] = true
		if st, known := states[pid]; known && started.Sub(st.lastFull) < fullSyncInterval {
			quickIDs = append(quickIDs, pid)
		} else {
			fullIDs = append(fullIDs, pid)
		}
	}

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

	// Changes of the projects that are up to date: two requests for all of them
	quickDone, movedOut := s.loadChanges(store, quickIDs, selected, states, addIssues, touched)
	failed += len(quickIDs) - len(quickDone)

	// Complete loads, a few projects at a time
	loads := s.loadFull(fullIDs)
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
			return fmt.Errorf("upsert issues: %w", err)
		}
	}

	// Issues that are gone from Redmine. After the upsert: an issue moved to
	// another synced project already belongs to it and is not touched here.
	deleted := 0
	for _, load := range complete {
		deleted += deleteGone(store, load)
	}
	if len(movedOut) > 0 {
		// Moved to a project that is not synced
		if err := store.deleteIssues(movedOut); err != nil {
			slog.Warn("Failed to delete issues moved out of the synced projects", "error", err)
		}
	}

	ids := make([]int64, 0, len(touched))
	for id := range touched {
		ids = append(ids, int64(id))
	}
	if err := store.updateIssueHours(ids); err != nil {
		return fmt.Errorf("update issue hours: %w", err)
	}

	// Everything is stored: the projects that were read move on
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
	return nil
}

// loadChanges fetches what changed in the given projects since their last
// sync and stores the time entries. It returns the projects whose changes
// were read (the others may not be taken as synced) and the changed issues
// that now belong to a project that is not synced at all.
//
// The changed issues of all projects come with one request. The time is
// asked project by project: Redmine is too slow to answer for all of them
// at once.
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

	// Entries are filtered by the day they were spent on; one extra day
	// covers time logged for yesterday.
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
				// No right to the time of this project: its issues are
				// synced without hours, see loadProject
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
		// An issue of a project that is being read completely comes with
		// it; one of a project that failed is read again next time
	}
	addIssues(mine)
	slog.Info("Loaded changes from Redmine", "projects", len(done), "since", since.Format(time.RFC3339),
		"issues", len(mine), "time_entries", total)
	return done, movedOut
}

// loadFull reads the given projects completely, fullSyncWorkers at a time.
func (s *Syncer) loadFull(projectIDs []int) []fullLoad {
	loads := make([]fullLoad, len(projectIDs))
	var wg sync.WaitGroup
	slots := make(chan struct{}, fullSyncWorkers)
	for i, pid := range projectIDs {
		wg.Add(1)
		slots <- struct{}{}
		go func(i, pid int) {
			defer wg.Done()
			defer func() { <-slots }()
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
		// The key has no right to the time of this project. It is not a
		// failure and will not pass by itself: the issues are synced
		// without their hours.
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

// deleteGone removes the issues of a completely read project that Redmine
// no longer has, and returns how many were removed.
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

// pgStore keeps the synced data in PostgreSQL.
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

// deleteIssues removes issues together with what exists only for them:
// their time entries and their places in sprints. Snapshots keep the history.
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
	// A project that is selected again later starts from a complete load
	_, err := p.db.Exec(`DELETE FROM sync_state WHERE data_source = 'redmine' AND NOT (project_id = ANY($1::int[]))`,
		pq.Array(projectIDs))
	return err
}

// storeTimeEntries saves time entries of a project. A full load replaces the
// project's entries, so entries deleted in Redmine disappear as well.
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

	// A parent project may still return entries of a subproject: keep each id once.
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

// upsertIssues writes a batch of issues with a single multi-row statement —
// the database is remote, so one round trip per issue made the sync slow.
// Fact and bug fix hours are maintained by updateIssueHours.
func upsertIssues(db *sql.DB, batch []Issue) error {
	const cols = 18
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
			it.DoneRatio, it.TrackerName, it.AuthorName)
	}
	_, err := db.Exec(`
		INSERT INTO issues (
			external_id, project_id, project_name, subject, description,
			status_name, status_id, priority_name, priority_id,
			assigned_to_name, assigned_to_id, category_name,
			start_date, due_date, estimated_hours,
			done_ratio, tracker_name, author_name,
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
			synced_at = NOW()`, args...)
	return err
}

// updateIssueHours recomputes fact (all time entries) and bug fix hours
// (the "Fixing bugs" activity) of the given issues from stored time entries.
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

// cleanupUnselected removes issues and time entries of projects that are no
// longer selected by any user.
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
