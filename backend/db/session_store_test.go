package db

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestMemorySessionStoreConcurrentAccess вызывает Set/Get/Delete из многих
// горутин сразу. Запускать с -race: прежнее хранилище на карте здесь падало.
func TestMemorySessionStoreConcurrentAccess(t *testing.T) {
	s := newMemorySessionStore()
	defer s.Close()

	ctx := context.Background()
	const workers = 100

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("sess-%d", n)
			if err := s.Set(ctx, id, n, time.Minute); err != nil {
				t.Errorf("Set(%s): %v", id, err)
				return
			}
			if _, err := s.Get(ctx, id); err != nil {
				t.Errorf("Get(%s): %v", id, err)
			}
			// Половина горутин ещё и удаляет — одновременно с чтением и записью.
			if n%2 == 0 {
				if err := s.Delete(ctx, id); err != nil {
					t.Errorf("Delete(%s): %v", id, err)
				}
			}
		}(i)
	}
	wg.Wait()
}

// TestMemorySessionStoreExpired проверяет истечение при Get и фоновую очистку.
func TestMemorySessionStoreDeleteUser(t *testing.T) {
	s := newMemorySessionStore()
	defer s.Close()
	ctx := context.Background()

	// Пользователь 7 вошёл с двух устройств, пользователь 8 — с одного
	s.Set(ctx, "phone", 7, time.Minute)
	s.Set(ctx, "laptop", 7, time.Minute)
	s.Set(ctx, "other", 8, time.Minute)

	deleted, err := s.DeleteUser(ctx, 7, "")
	if err != nil || deleted != 2 {
		t.Fatalf("DeleteUser = %d, %v; want 2 sessions", deleted, err)
	}
	for _, id := range []string{"phone", "laptop"} {
		if _, err := s.Get(ctx, id); err == nil {
			t.Errorf("session %q must be gone", id)
		}
	}
	if _, err := s.Get(ctx, "other"); err != nil {
		t.Errorf("a session of another user must stay: %v", err)
	}
}

// После смены пароля самим пользователем остаётся только текущая сессия
func TestMemorySessionStoreDeleteUserKeepsCurrent(t *testing.T) {
	s := newMemorySessionStore()
	defer s.Close()
	ctx := context.Background()
	s.Set(ctx, "phone", 7, time.Minute)
	s.Set(ctx, "laptop", 7, time.Minute)

	deleted, err := s.DeleteUser(ctx, 7, "laptop")
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteUser = %d, %v; want 1 session", deleted, err)
	}
	if _, err := s.Get(ctx, "phone"); err == nil {
		t.Error("другая сессия должна быть закрыта")
	}
	if _, err := s.Get(ctx, "laptop"); err != nil {
		t.Errorf("текущая сессия должна остаться: %v", err)
	}
}

func TestMemorySessionStoreExpired(t *testing.T) {
	s := newMemorySessionStore()
	defer s.Close()
	ctx := context.Background()

	if err := s.Set(ctx, "old", 1, -time.Second); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := s.Get(ctx, "old"); err == nil {
		t.Fatal("expected error for expired session")
	}

	// Очистка убирает истёкшие записи, к которым больше не обращались.
	if err := s.Set(ctx, "abandoned", 2, -time.Second); err != nil {
		t.Fatalf("Set: %v", err)
	}
	s.removeExpired()
	s.mu.RLock()
	_, ok := s.sessions["abandoned"]
	s.mu.RUnlock()
	if ok {
		t.Fatal("sweeper should have removed abandoned expired session")
	}
}

// TestMemorySessionStoreCloseStopsCleanup проверяет, что Close можно
// вызывать повторно и что он останавливает фоновую горутину.
func TestMemorySessionStoreCloseStopsCleanup(t *testing.T) {
	s := newMemorySessionStore()
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("cleanup goroutine did not exit after Close")
	}
}
