// Package jobs выполняет фоновую работу дашборда помимо синхронизации с
// источником данных: ежедневную очистку истории. Он же хранит состояние
// воркеров для экрана, на котором их видит администратор.
package jobs

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Состояния запуска воркера.
const (
	StateIdle    = "idle"    // последний запуск завершён
	StateRunning = "running" // запуск идёт
	StateError   = "error"   // последний запуск не удался
)

// Status — что известно о последнем запуске воркера.
type Status struct {
	State      string     `json:"state"`
	LastStart  *time.Time `json:"last_start"`
	DurationMs int64      `json:"duration_ms"`
	// Processed: сколько сделал запуск, в единицах воркера (строки,
	// удалённые очисткой).
	Processed int64  `json:"processed"`
	Error     string `json:"error"`
}

// Store хранит состояния воркеров.
type Store interface {
	Load(key string) (Status, bool)
	Save(key string, status Status)
}

// NewStore возвращает хранилище в Redis либо в памяти, если Redis не
// настроен или не отвечает: тогда состояния живут до перезапуска.
func NewStore(redisURL string) Store {
	if redisURL == "" {
		return newMemoryStore()
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Warn("Invalid Redis URL, worker statuses are kept in memory", "error", err)
		return newMemoryStore()
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("Redis is not available, worker statuses are kept in memory", "error", err)
		return newMemoryStore()
	}
	return &redisStore{client: client}
}

const redisKeyPrefix = "pm:worker:"

type redisStore struct {
	client *redis.Client
}

func (s *redisStore) Load(key string) (Status, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	data, err := s.client.Get(ctx, redisKeyPrefix+key).Bytes()
	if err != nil {
		return Status{}, false
	}
	var status Status
	if json.Unmarshal(data, &status) != nil {
		return Status{}, false
	}
	return status, true
}

func (s *redisStore) Save(key string, status Status) {
	data, _ := json.Marshal(status)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.client.Set(ctx, redisKeyPrefix+key, data, 0).Err(); err != nil {
		slog.Warn("Could not save the status of a worker", "worker", key, "error", err)
	}
}

type memoryStore struct {
	mu       sync.Mutex
	statuses map[string]Status
}

func newMemoryStore() *memoryStore {
	return &memoryStore{statuses: make(map[string]Status)}
}

func (s *memoryStore) Load(key string) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.statuses[key]
	return status, ok
}

func (s *memoryStore) Save(key string, status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses[key] = status
}
