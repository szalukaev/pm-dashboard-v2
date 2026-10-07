package redmine

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lib/pq"
)

const (
	// fullSyncInterval: how often everything is re-read from Redmine. In
	// between only changes are fetched; the full pass also picks up deleted
	// time entries and time logged for long-past dates.
	fullSyncInterval = time.Hour
	// syncOverlap widens the incremental window against clock skew between
	// this server and Redmine.
	syncOverlap = 10 * time.Minute
	// issueBatchSize rows per INSERT: 18 params each, well below the 65535 limit.
	issueBatchSize = 500
	// timeEntryBatchSize rows per INSERT: 5 params each.
	timeEntryBatchSize = 1000
)

// syncProjectIssues loads issues and time entries of the given projects.
// Projects not seen before and every project once per fullSyncInterval are
// read completely; otherwise only issues updated and time entries spent since
// the previous successful sync are fetched. Fact and bug fix hours are then
// recomputed from the stored time entries of the affected issues.
func (s *Syncer) syncProjectIssues(db *sql.DB, projectIDs []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	started := time.Now()
	full := s.lastFull.IsZero() || started.Sub(s.lastFull) >= fullSyncInterval
	since := s.lastSync.Add(-syncOverlap)

	var issues []Issue
	seenIssues := make(map[int]bool)
	touched := make(map[int]bool)
	loaded := make([]int, 0, len(projectIDs))
	failed := 0

	for _, pid := range projectIDs {
		projectFull := full || !s.knownProjects[pid]

		from, updatedSince := "", (*time.Time)(nil)
		if !projectFull {
			// Entries are filtered by the day they were spent on; one extra day
			// covers time logged for yesterday.
			from = since.AddDate(0, 0, -1).Format("2006-01-02")
			updatedSince = &since
		}

		entries, err := s.client.GetTimeEntries(pid, from)
		if err != nil {
			slog.Warn("Failed to get time entries for project", "project_id", pid, "error", err)
			failed++
			continue
		}
		projectIssues, err := s.client.GetIssues(pid, updatedSince)
		if err != nil {
			slog.Warn("Failed to get issues for project", "project_id", pid, "error", err)
			failed++
			continue
		}
		if err := storeTimeEntries(db, pid, entries, projectFull); err != nil {
			slog.Warn("Failed to store time entries for project", "project_id", pid, "error", err)
			failed++
			continue
		}

		for _, e := range entries {
			touched[e.IssueID] = true
		}
		for _, iss := range projectIssues {
			if seenIssues[iss.ExternalID] {
				continue
			}
			seenIssues[iss.ExternalID] = true
			touched[iss.ExternalID] = true
			issues = append(issues, iss)
		}
		loaded = append(loaded, pid)
		slog.Info("Loaded project from Redmine", "project_id", pid, "full", projectFull,
			"issues", len(projectIssues), "time_entries", len(entries))
	}

	for start := 0; start < len(issues); start += issueBatchSize {
		end := min(start+issueBatchSize, len(issues))
		if err := upsertIssues(db, issues[start:end]); err != nil {
			return fmt.Errorf("upsert issues: %w", err)
		}
	}

	ids := make([]int64, 0, len(touched))
	for id := range touched {
		ids = append(ids, int64(id))
	}
	if err := updateIssueHours(db, ids); err != nil {
		return fmt.Errorf("update issue hours: %w", err)
	}

	for _, pid := range loaded {
		s.knownProjects[pid] = true
	}
	// The window moves on only when every project was read: otherwise the
	// next run repeats it and nothing is lost.
	if failed == 0 {
		s.lastSync = started
		if full {
			s.lastFull = started
		}
	}

	slog.Info("Synced issues", "full", full, "issues", len(issues), "recalculated", len(ids),
		"projects", len(loaded), "failed", failed, "duration", time.Since(started))

	if err := cleanupUnselected(db, projectIDs); err != nil {
		slog.Warn("Failed to cleanup unselected projects", "error", err)
	}
	return nil
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
