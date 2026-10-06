package db

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
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

// MemorySessionStore is a fallback when Redis is not available.
// Safe for concurrent use; expired entries are swept by a background goroutine.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]sessionEntry
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
}

type sessionEntry struct {
	userID  int
	expires time.Time
}

const memorySessionCleanupInterval = time.Minute

// newMemorySessionStore creates an in-memory store and starts the cleanup goroutine.
func newMemorySessionStore() *MemorySessionStore {
	s := &MemorySessionStore{
		sessions: make(map[string]sessionEntry),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

// cleanupLoop periodically removes expired sessions so abandoned ones don't leak memory.
func (s *MemorySessionStore) cleanupLoop() {
	defer close(s.done)
	ticker := time.NewTicker(memorySessionCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.removeExpired()
		case <-s.stop:
			return
		}
	}
}

// removeExpired deletes all expired entries under a write lock.
func (s *MemorySessionStore) removeExpired() {
	now := time.Now()
	s.mu.Lock()
	for id, entry := range s.sessions {
		if now.After(entry.expires) {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}

// NewSessionStore creates a Redis-backed session store, falls back to memory
func NewSessionStore(redisURL string) SessionStore {
	if redisURL == "" {
		slog.Warn("No Redis URL configured, using in-memory session store")
		return newMemorySessionStore()
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Warn("Invalid Redis URL, falling back to memory store", "error", err, "url", redisURL)
		return newMemorySessionStore()
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("Redis not available, falling back to memory store", "error", err)
		return newMemorySessionStore()
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
	s.mu.Lock()
	s.sessions[sessionID] = sessionEntry{
		userID:  userID,
		expires: time.Now().Add(expiry),
	}
	s.mu.Unlock()
	return nil
}

func (s *MemorySessionStore) Get(ctx context.Context, sessionID string) (int, error) {
	// Write lock: lazy delete of an expired entry mutates the map.
	s.mu.Lock()
	entry, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return 0, fmt.Errorf("session not found")
	}
	if time.Now().After(entry.expires) {
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return 0, fmt.Errorf("session expired")
	}
	s.mu.Unlock()
	return entry.userID, nil
}

func (s *MemorySessionStore) Delete(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	return nil
}

// Close stops the cleanup goroutine and waits for it to exit.
// Safe to call multiple times.
func (s *MemorySessionStore) Close() error {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
	})
	return nil
}
