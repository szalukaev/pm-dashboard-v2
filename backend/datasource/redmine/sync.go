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

	// Incremental sync state, guarded by mu.
	mu            sync.Mutex
	lastSync      time.Time // start of the last sync that read every project
	lastFull      time.Time // start of the last such sync that was a full one
	knownProjects map[int]bool
}

func NewSyncer(client *Client) *Syncer {
	return &Syncer{client: client, knownProjects: make(map[int]bool)}
}

func (s *Syncer) SyncAll(ctx context.Context, db **sql.DB) error {
	startTime := time.Now()
	slog.Info("Starting sync from Redmine")

	// Log sync start
	var logID int
	(*db).QueryRow(`INSERT INTO collection_log (started_at, status, data_source)
		VALUES ($1, 'running', 'redmine') RETURNING id`, startTime).Scan(&logID)

	totalIssues := 0
	var syncErr error

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
		if err := s.syncIssues(ctx, db); err != nil {
			syncErr = fmt.Errorf("sync issues: %w", err)
		}
		// Count synced issues
		(*db).QueryRow("SELECT COUNT(*) FROM issues WHERE data_source = 'redmine'").Scan(&totalIssues)
	}

	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	// Update collection log
	if syncErr != nil {
		(*db).Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, status='error', error_text=$3 WHERE id=$4`,
			time.Now(), durationMs, syncErr.Error(), logID)
		slog.Error("Sync failed", "duration", duration, "error", syncErr)
		return syncErr
	}

	(*db).Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, issues_collected=$3, status='success' WHERE id=$4`,
		time.Now(), durationMs, totalIssues, logID)

	slog.Info("Sync completed", "duration", duration, "issues", totalIssues)
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

func (s *Syncer) syncIssues(_ context.Context, db **sql.DB) error {
	// Get union of selected_projects from all users
	rows, err := (*db).Query(`SELECT DISTINCT jsonb_array_elements_text(selected_projects)::int AS pid
		FROM user_settings WHERE selected_projects != '[]'::jsonb`)
	if err != nil {
		return err
	}
	var projectIDs []int
	for rows.Next() {
		var pid int
		if rows.Scan(&pid) == nil {
			projectIDs = append(projectIDs, pid)
		}
	}
	rows.Close()

	if len(projectIDs) == 0 {
		slog.Info("No projects selected in user_settings, skipping issue sync")
		return nil
	}

	// Selected projects include all their descendants
	if expanded, err := datasource.ExpandProjects(*db, projectIDs); err == nil {
		projectIDs = expanded
	}

	slog.Info("Syncing issues for selected projects", "count", len(projectIDs))
	return s.syncProjectIssues(*db, projectIDs)
}
