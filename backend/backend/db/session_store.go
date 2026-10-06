package db

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionStore interface for session management
type SessionStore interface {
	Set(ctx context.Context, sessionID string, userID int, expiry time.Duration) error
	Get(ctx context.Context, sessionID string) (int, error)
	Delete(ctx context.Context, sessionID string) error
	Close() error
}

// RedisSessionStore uses Redis for session storage
type RedisSessionStore struct {
	client *redis.Client
}

// MemorySessionStore is a fallback when Redis is not available
type MemorySessionStore struct {
	sessions map[string]sessionEntry
}

type sessionEntry struct {
	userID  int
	expires time.Time
}

// NewSessionStore creates a Redis-backed session store, falls back to memory
func NewSessionStore(redisURL string) SessionStore {
	if redisURL == "" {
		slog.Warn("No Redis URL configured, using in-memory session store")
		return &MemorySessionStore{sessions: make(map[string]sessionEntry)}
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Warn("Invalid Redis URL, falling back to memory store", "error", err, "url", redisURL)
		return &MemorySessionStore{sessions: make(map[string]sessionEntry)}
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("Redis not available, falling back to memory store", "error", err)
		return &MemorySessionStore{sessions: make(map[string]sessionEntry)}
	}

	slog.Info("Redis session store connected", "url", redisURL)
	return &RedisSessionStore{client: client}
}

// ─── Redis implementation ───

func (s *RedisSessionStore) Set(ctx context.Context, sessionID string, userID int, expiry time.Duration) error {
	key := "session:" + sessionID
	return s.client.Set(ctx, key, userID, expiry).Err()
}

func (s *RedisSessionStore) Get(ctx context.Context, sessionID string) (int, error) {
	key := "session:" + sessionID
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, fmt.Errorf("session not found")
	}
	if err != nil {
		return 0, fmt.Errorf("redis error: %w", err)
	}
	userID, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid session data")
	}
	return userID, nil
}

func (s *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	key := "session:" + sessionID
	return s.client.Del(ctx, key).Err()
}

func (s *RedisSessionStore) Close() error {
	return s.client.Close()
}

// ─── Memory fallback implementation ───

func (s *MemorySessionStore) Set(ctx context.Context, sessionID string, userID int, expiry time.Duration) error {
	s.sessions[sessionID] = sessionEntry{
		userID:  userID,
		expires: time.Now().Add(expiry),
	}
	return nil
}

func (s *MemorySessionStore) Get(ctx context.Context, sessionID string) (int, error) {
	entry, ok := s.sessions[sessionID]
	if !ok {
		return 0, fmt.Errorf("session not found")
	}
	if time.Now().After(entry.expires) {
		delete(s.sessions, sessionID)
		return 0, fmt.Errorf("session expired")
	}
	return entry.userID, nil
}

func (s *MemorySessionStore) Delete(ctx context.Context, sessionID string) error {
	delete(s.sessions, sessionID)
	return nil
}

func (s *MemorySessionStore) Close() error {
	return nil
}
