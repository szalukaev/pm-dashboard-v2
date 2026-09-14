package redmine

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

type Syncer struct {
	client *Client
}

func NewSyncer(client *Client) *Syncer {
	return &Syncer{client: client}
}

func (s *Syncer) SyncAll(ctx context.Context, db *sql.DB) error {
	startTime := time.Now()
	slog.Info("Starting full sync from Redmine")

	// Log sync start
	var logID int
	db.QueryRow(`INSERT INTO collection_log (started_at, status, data_source)
		VALUES ($1, 'running', 'redmine') RETURNING id`, startTime).Scan(&logID)

	totalIssues := 0
	var syncErr error

	// Sync statuses
	if err := s.syncStatuses(ctx, db); err != nil {
		syncErr = fmt.Errorf("sync statuses: %w", err)
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
		db.QueryRow("SELECT COUNT(*) FROM issues WHERE data_source = 'redmine'").Scan(&totalIssues)
	}

	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	// Update collection log
	if syncErr != nil {
		db.Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, status='error', error_text=$3 WHERE id=$4`,
			time.Now(), durationMs, syncErr.Error(), logID)
		slog.Error("Sync failed", "duration", duration, "error", syncErr)
		return syncErr
	}

	db.Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, issues_collected=$3, status='success' WHERE id=$4`,
		time.Now(), durationMs, totalIssues, logID)

	slog.Info("Full sync completed", "duration", duration, "issues", totalIssues)
	return nil
}

func (s *Syncer) syncStatuses(_ context.Context, db *sql.DB) error {
	statuses, err := s.client.GetStatuses()
	if err != nil {
		return err
	}
	for _, st := range statuses {
		group := "open"
		if st.IsClosed {
			group = "closed"
		}
		_, err := db.Exec(`
			INSERT INTO statuses (external_id, name, is_closed, group_name, data_source, synced_at)
			VALUES ($1, $2, $3, $4, 'redmine', NOW())
			ON CONFLICT (external_id, data_source) DO UPDATE SET
				name = EXCLUDED.name,
				is_closed = EXCLUDED.is_closed,
				group_name = EXCLUDED.group_name,
				synced_at = NOW()
		`, st.ExternalID, st.Name, st.IsClosed, group)
		if err != nil {
			return err
		}
	}
	slog.Info("Synced statuses", "count", len(statuses))
	return nil
}

func (s *Syncer) syncProjects(_ context.Context, db *sql.DB) error {
	projects, err := s.client.GetProjects()
	if err != nil {
		return err
	}
	for _, p := range projects {
		_, err := db.Exec(`
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

func (s *Syncer) syncMembers(_ context.Context, db *sql.DB) error {
	// Get all project IDs
	rows, err := db.Query("SELECT external_id FROM projects WHERE data_source = 'redmine'")
	if err != nil {
		return err
	}
	defer rows.Close()

	seen := make(map[int]bool)
	for rows.Next() {
		var projectID int
		if err := rows.Scan(&projectID); err != nil {
			continue
		}
		members, err := s.client.GetProjectMembers(projectID)
		if err != nil {
			slog.Warn("Failed to get members", "project_id", projectID, "error", err)
			continue
		}
		for _, m := range members {
			if seen[m.ExternalID] {
				continue
			}
			seen[m.ExternalID] = true
			_, err := db.Exec(`
				INSERT INTO members (external_id, name, login, data_source, synced_at)
				VALUES ($1, $2, $3, 'redmine', NOW())
				ON CONFLICT (external_id, data_source) DO UPDATE SET
					name = EXCLUDED.name,
					login = EXCLUDED.login,
					synced_at = NOW()
			`, m.ExternalID, m.Name, m.Login)
			if err != nil {
				slog.Warn("Failed to upsert member", "member_id", m.ExternalID, "error", err)
			}
		}
	}
	slog.Info("Synced members", "count", len(seen))
	return nil
}

func (s *Syncer) syncIssues(_ context.Context, db *sql.DB) error {
	issues, err := s.client.GetIssues(0) // all projects
	if err != nil {
		return err
	}
	count := 0
	for _, iss := range issues {
		_, err := db.Exec(`
			INSERT INTO issues (
				external_id, project_id, project_name, subject, description,
				status_name, status_id, priority_name, priority_id,
				assigned_to_name, assigned_to_id, category_name,
				start_date, due_date, estimated_hours, spent_hours,
				done_ratio, tracker_name, author_name, data_source, synced_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,'redmine',NOW())
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
				spent_hours = EXCLUDED.spent_hours,
				done_ratio = EXCLUDED.done_ratio,
				tracker_name = EXCLUDED.tracker_name,
				author_name = EXCLUDED.author_name,
				synced_at = NOW()
		`, iss.ExternalID, iss.ProjectID, iss.ProjectName, iss.Subject, iss.Description,
			iss.StatusName, iss.StatusID, iss.PriorityName, iss.PriorityID,
			iss.AssignedToName, iss.AssignedToID, iss.CategoryName,
			iss.StartDate, iss.DueDate, iss.EstimatedHours, iss.SpentHours,
			iss.DoneRatio, iss.TrackerName, iss.AuthorName)
		if err != nil {
			slog.Warn("Failed to upsert issue", "issue_id", iss.ExternalID, "error", err)
			continue
		}
		count++
	}
	slog.Info("Synced issues", "synced", count, "total", len(issues))
	return nil
}
