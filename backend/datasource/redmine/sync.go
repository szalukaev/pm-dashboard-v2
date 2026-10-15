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

	"pm-dashboard/datasource"
)

type Syncer struct {
	client *Client

	// Projects возвращает проекты, задачи которых синхронизируются; необязательно.
	Projects func() ([]int, error)

	// FullSyncTime возвращает время суток ("HH:MM", время сервера), после
	// которого проекты раз в день перечитываются целиком; необязательно.
	FullSyncTime func() string

	// Одновременно идёт только одна синхронизация задач.
	mu sync.Mutex
}

// Статусы, с которыми синхронизация записывается в collection_log.
const (
	StatusSuccess = "success"
	// StatusPartial: синхронизация завершилась, но часть проектов прочитать
	// не удалось. В error_text лежит "<не прочитано>/<всего>".
	StatusPartial = "partial"
	StatusError   = "error"
	// StatusStopped: синхронизацию остановил администратор.
	StatusStopped = "stopped"
)

type fullSyncKey struct{}

// WithFullSync помечает синхронизацию, которая должна перечитать каждый
// проект целиком независимо от расписания.
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

	// Запись о начале синхронизации
	var logID int
	if err := (*db).QueryRow(`INSERT INTO collection_log (started_at, status, data_source)
		VALUES ($1, 'running', 'redmine') RETURNING id`, startTime).Scan(&logID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("Database query failed", "at", "SyncAll", "error", err)
	}

	var stats syncStats
	var syncErr error

	// Запросы этой синхронизации обрываются, когда её останавливают
	original := s.client
	s.client = original.withContext(ctx)
	defer func() { s.client = original }()

	// Статусы
	if err := s.syncStatuses(ctx, db); err != nil {
		syncErr = fmt.Errorf("sync statuses: %w", err)
	}

	// Приоритеты
	if syncErr == nil {
		if err := s.syncPriorities(ctx, db); err != nil {
			syncErr = fmt.Errorf("sync priorities: %w", err)
		}
	}

	// Проекты
	if syncErr == nil {
		if err := s.syncProjects(ctx, db); err != nil {
			syncErr = fmt.Errorf("sync projects: %w", err)
		}
	}

	// Сотрудники (по всем проектам)
	if syncErr == nil {
		if err := s.syncMembers(ctx, db); err != nil {
			syncErr = fmt.Errorf("sync members: %w", err)
		}
	}

	// Задачи (по всем проектам)
	if syncErr == nil {
		var err error
		if stats, err = s.syncIssues(ctx, db); err != nil {
			syncErr = fmt.Errorf("sync issues: %w", err)
		}
	}

	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	// Итог — в журнал синхронизации
	if ctx.Err() != nil {
		if _, err := (*db).Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, status=$3 WHERE id=$4`,
			time.Now(), durationMs, StatusStopped, logID); err != nil {
			slog.Warn("Database write failed", "at", "SyncAll", "error", err)
		}
		slog.Info("Sync stopped", "duration", duration)
		return ctx.Err()
	}
	if syncErr != nil {
		if _, err := (*db).Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, status=$3, error_text=$4 WHERE id=$5`,
			time.Now(), durationMs, StatusError, syncErr.Error(), logID); err != nil {
			slog.Warn("Database write failed", "at", "SyncAll", "error", err)
		}
		slog.Error("Sync failed", "duration", duration, "error", syncErr)
		return syncErr
	}

	// issues_collected — что принесла эта синхронизация: записанные ею задачи.
	status, note := StatusSuccess, ""
	if stats.failed > 0 {
		status, note = StatusPartial, fmt.Sprintf("%d/%d", stats.failed, stats.projects)
	}
	if _, err := (*db).Exec(`UPDATE collection_log SET finished_at=$1, duration_ms=$2, issues_collected=$3, status=$4, error_text=NULLIF($5, '') WHERE id=$6`,
		time.Now(), durationMs, stats.issues, status, note, logID); err != nil {
		slog.Warn("Database write failed", "at", "SyncAll", "error", err)
	}

	slog.Info("Sync completed", "duration", duration, "issues", stats.issues, "status", status)
	return nil
}

func (s *Syncer) syncStatuses(_ context.Context, db **sql.DB) error {
	statuses, err := s.client.GetStatuses()
	if err != nil {
		return err
	}
	for _, st := range statuses {
		// group_name — настройка пользователя (Настройки → Данные: Открытые/
		// Тестирование/Закрытые). Redmine знает только is_closed: перезапись
		// group_name при каждой синхронизации стирала заданную раскладку (а
		// группу «testing» — целиком), и фильтры вкладки «Задачи» пустели.
		// Группа по умолчанию из is_closed ставится только при первой вставке.
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
		// sort_order — настройка пользователя (администратор меняет порядок
		// приоритетов в настройках): берётся из порядка Redmine только при
		// первой вставке и после этого не перезаписывается.
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
	// Все активные пользователи одним запросом, без обхода всех проектов
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

	// Очистка: удаляются сотрудники, которых нет среди активных пользователей
	// (остатки прежней синхронизации по участию в проектах)
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
	// Что синхронизировать: проекты, показанные пользователям в пределах их
	// прав (Projects), либо — если права доступа не подключены — объединение
	// проектов, выбранных всеми пользователями.
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
		_ = rows.Close()
	}

	if len(projectIDs) == 0 {
		slog.Info("No projects selected in user_settings, skipping issue sync")
		return syncStats{}, nil
	}

	// Выбранные проекты включают всех своих потомков
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
