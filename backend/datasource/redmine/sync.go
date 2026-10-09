package redmine

import (
	"context"
	"strings"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"pm-dashboard/datasource"
)

type Syncer struct {
	client *Client

	// Projects returns the projects whose issues are synced; optional.
	Projects func() ([]int, error)

	// FullSyncTime returns the time of day ("HH:MM", server time) after
	// which the projects are re-read completely once a day; optional.
	FullSyncTime func() string

	// One sync of issues at a time.
	mu sync.Mutex
}

// Statuses a sync is recorded with in collection_log.
const (
	StatusSuccess = "success"
	// StatusPartial: the sync finished, but some projects could not be read.
	// error_text holds "<failed>/<total>".
	StatusPartial = "partial"
	StatusError   = "error"
	// StatusStopped: the sync was stopped by the administrator.
	StatusStopped = "stopped"
)

type fullSyncKey struct{}

// WithFullSync marks a sync that must re-read every project completely
// whatever the schedule says.
func WithFullSync(ctx context.Context) context.Context {
	return context.WithValue(ctx, fullSyncKey{}, true)
}

func fullSyncRequested(ctx context.Context) bool {
	requested, _ := ctx.Value(fullSyncKey{}).(bool)
	return requested
}

func NewSyncer(client *Client) *Syncer {
	return &Syncer{client: client}
}

func (s *Syncer) SyncAll(ctx context.Context, db **sql.DB) error {
	startTime := time.Now()
	slog.Info("Starting sync from Redmine")

	// Log sync start
	var logID int
	(*db).QueryRow(`INSERT INTO collection_log (started_at, status, data_source)
		VALUES ($1, 'running', 'redmine') RETURNING id`, startTime).Scan(&logID)

	var stats syncStats
	var syncErr error

	// The requests of this sync end when it is stopped
	original := s.client
	s.client = original.withContext(ctx)
	defer func() { s.client = original }()

	// Sync statuses
	if err := s.syncStatuses(ctx, db); err != nil {
		syncErr = fmt.Errorf("sync statuses: %w", err)
	}

	// Sync priorities
	if syncErr == nil {
		if err := s.syncPriorities(ctx, db); err != nil {
			syncErr = fmt.Errorf("sync priorities: %w", err)
		}
	}

	// Sync projects
	if syncErr == nil {
		if err := s.syncProjects(ctx, db); err != nil {
			syncErr = fmt.Errorf("sync projects: %w", err)
		}
	}

	// Sync members (from all projects)
	if syncErr == nil {
		if err := s.syncMembers(ctx, db); err != nil {
			syncErr = fmt.Errorf("sync members: %w", err)
		}
	}

	// Sync issues (from all projects)
	if syncErr == nil {
		var err error
		if stats, err = s.syncIssues(ctx, db); err != nil {
			syncErr = fmt.Errorf("sync issues: %w", err)
		}
	}

	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	// Update collection log
	if ctx.Err() != nil {
		(*db).Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, status=$3 WHERE id=$4`,
			time.Now(), durationMs, StatusStopped, logID)
		slog.Info("Sync stopped", "duration", duration)
		return ctx.Err()
	}
	if syncErr != nil {
		(*db).Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, status=$3, error_text=$4 WHERE id=$5`,
			time.Now(), durationMs, StatusError, syncErr.Error(), logID)
		slog.Error("Sync failed", "duration", duration, "error", syncErr)
		return syncErr
	}

	// issues_collected is what this sync brought: the issues it wrote.
	status, note := StatusSuccess, ""
	if stats.failed > 0 {
		status, note = StatusPartial, fmt.Sprintf("%d/%d", stats.failed, stats.projects)
	}
	(*db).Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, issues_collected=$3, status=$4, error_text=NULLIF($5, '') WHERE id=$6`,
		time.Now(), durationMs, stats.issues, status, note, logID)

	slog.Info("Sync completed", "duration", duration, "issues", stats.issues, "status", status)
	return nil
}

func (s *Syncer) syncStatuses(_ context.Context, db **sql.DB) error {
	statuses, err := s.client.GetStatuses()
	if err != nil {
		return err
	}
	for _, st := range statuses {
		// group_name is user configuration (Настройки → Данные: Открытые/
		// Тестирование/Закрытые). Redmine only knows is_closed — overwriting
		// group_name on every sync used to wipe the custom layout (and the
		// "testing" group entirely), which emptied the Tasks tab filters.
		// On first insert only, seed a default group from is_closed.
		group := "open"
		if st.IsClosed {
			group = "closed"
		}
		_, err := (*db).Exec(`
			INSERT INTO statuses (external_id, name, is_closed, group_name, data_source, synced_at)
			VALUES ($1, $2, $3, $4, 'redmine', NOW())
			ON CONFLICT (external_id, data_source) DO UPDATE SET
				name = EXCLUDED.name,
				is_closed = EXCLUDED.is_closed,
				group_name = COALESCE(statuses.group_name, EXCLUDED.group_name),
				synced_at = NOW()
		`, st.ExternalID, st.Name, st.IsClosed, group)
		if err != nil {
			return err
		}
	}
	slog.Info("Synced statuses", "count", len(statuses))
	return nil
}

func (s *Syncer) syncPriorities(_ context.Context, db **sql.DB) error {
	priorities, err := s.client.GetPriorities()
	if err != nil {
		return err
	}
	for i, p := range priorities {
		// sort_order is user configuration (the administrator reorders
		// priorities in the settings): seed it from the Redmine order on
		// first insert only, never overwrite it afterwards.
		_, err := (*db).Exec(`
			INSERT INTO priorities (external_id, name, sort_order, data_source, synced_at)
			VALUES ($1, $2, $3, 'redmine', NOW())
			ON CONFLICT (external_id, data_source) DO UPDATE SET
				name = EXCLUDED.name,
				synced_at = NOW()
		`, p.ExternalID, p.Name, i)
		if err != nil {
			return err
		}
	}
	slog.Info("Synced priorities", "count", len(priorities))
	return nil
}

func (s *Syncer) syncProjects(_ context.Context, db **sql.DB) error {
	projects, err := s.client.GetProjects()
	if err != nil {
		return err
	}
	for _, p := range projects {
		_, err := (*db).Exec(`
			INSERT INTO projects (external_id, name, parent_id, data_source, synced_at)
			VALUES ($1, $2, $3, 'redmine', NOW())
			ON CONFLICT (external_id, data_source) DO UPDATE SET
				name = EXCLUDED.name,
				parent_id = EXCLUDED.parent_id,
				synced_at = NOW()
		`, p.ExternalID, p.Name, p.ParentID)
		if err != nil {
			return err
		}
	}
	slog.Info("Synced projects", "count", len(projects))
	return nil
}

func (s *Syncer) syncMembers(_ context.Context, db **sql.DB) error {
	// Get all active users in one API call instead of iterating all projects
	members, err := s.client.GetUsers()
	if err != nil {
		return fmt.Errorf("get users: %w", err)
	}
	count := 0
	for _, m := range members {
		_, err := (*db).Exec(`
			INSERT INTO members (external_id, name, login, data_source, synced_at)
			VALUES ($1, $2, $3, 'redmine', NOW())
			ON CONFLICT (external_id, data_source) DO UPDATE SET
				name = EXCLUDED.name,
				login = EXCLUDED.login,
				synced_at = NOW()
		`, m.ExternalID, m.Name, m.Login)
		if err != nil {
			slog.Warn("Failed to upsert member", "member_id", m.ExternalID, "error", err)
			continue
		}
		count++
	}
	slog.Info("Synced members", "count", count, "total", len(members))

	// Cleanup: remove members not in the active users list (stale data from old project-membership sync)
	if len(members) > 0 {
		placeholders := make([]string, len(members))
		args := make([]interface{}, len(members))
		for i, m := range members {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = m.ExternalID
		}
		delQuery := fmt.Sprintf(`DELETE FROM members WHERE data_source = 'redmine' AND external_id NOT IN (%s)`,
			strings.Join(placeholders, ","))
		delRes, err := (*db).Exec(delQuery, args...)
		if err != nil {
			slog.Warn("Failed to cleanup stale members", "error", err)
		} else if n, _ := delRes.RowsAffected(); n > 0 {
			slog.Info("Cleaned up stale members", "deleted", n)
		}
	}

	return nil
}

func (s *Syncer) syncIssues(ctx context.Context, db **sql.DB) (syncStats, error) {
	// The projects to sync: what the users are shown within their rights
	// (Projects), or — without access control wired in — the union of the
	// projects selected by all users.
	var projectIDs []int
	if s.Projects != nil {
		ids, err := s.Projects()
		if err != nil {
			return syncStats{}, err
		}
		projectIDs = ids
	} else {
		rows, err := (*db).Query(`SELECT DISTINCT jsonb_array_elements_text(selected_projects)::int AS pid
			FROM user_settings WHERE selected_projects != '[]'::jsonb`)
		if err != nil {
			return syncStats{}, err
		}
		for rows.Next() {
			var pid int
			if rows.Scan(&pid) == nil {
				projectIDs = append(projectIDs, pid)
			}
		}
		rows.Close()
	}

	if len(projectIDs) == 0 {
		slog.Info("No projects selected in user_settings, skipping issue sync")
		return syncStats{}, nil
	}

	// Selected projects include all their descendants
	if expanded, err := datasource.ExpandProjects(*db, projectIDs); err == nil {
		projectIDs = expanded
	}

	slog.Info("Syncing issues for selected projects", "count", len(projectIDs))
	s.mu.Lock()
	defer s.mu.Unlock()
	fullTime := ""
	if s.FullSyncTime != nil {
		fullTime = s.FullSyncTime()
	}
	return s.syncProjectIssues(ctx, pgStore{db: *db}, projectIDs, time.Now(), fullTime)
}
