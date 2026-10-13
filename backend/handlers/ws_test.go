package handlers

import (
	"testing"
	"time"
)

// TestWSHubSurvivesFullSendBuffer проверяет, что медленный клиент (полный
// буфер отправки) не останавливает хаб: прежний код отправлял в
// h.unregister из ветки рассылки в run() и навсегда зависал на первом же
// плохом клиенте.
func TestWSHubSurvivesFullSendBuffer(t *testing.T) {
	h := NewWSHub()

	// Клиент с буфером на одно сообщение, который уже полон.
	slow := &wsClient{send: make(chan []byte, 1)}
	h.register <- slow
	slow.send <- []byte(`{"event":"fill"}`) // буфер заполнен

	// Обе отправки должны пройти без блокировки.
	done := make(chan struct{})
	go func() {
		h.BroadcastEvent("first", map[string]string{"k": "v"})
		h.BroadcastEvent("second", nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("BroadcastEvent blocked on a client with a full send buffer")
	}

	// Хаб по-прежнему принимает новых клиентов и доставляет сообщения.
	healthy := &wsClient{send: make(chan []byte, 4)}
	h.register <- healthy
	h.BroadcastEvent("third", nil)

	select {
	case msg := <-healthy.send:
		if len(msg) == 0 {
			t.Fatal("expected non-empty message")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("healthy client did not receive broadcast")
	}

	// Медленный клиент должен быть отключён.
	h.mu.Lock()
	_, stillThere := h.clients[slow]
	total := len(h.clients)
	h.mu.Unlock()
	if stillThere {
		t.Fatal("slow client should have been removed from the hub")
	}
	if total != 1 {
		t.Fatalf("expected 1 client left, got %d", total)
	}
}

// TestWSClientCloseIdempotent — close() должен быть безопасен при вызове с
// разных сторон (уборка мёртвых клиентов и unregister).
func TestWSClientCloseIdempotent(t *testing.T) {
	c := &wsClient{send: make(chan []byte, 1)}
	c.close()
	c.close() // повторное закрытие не должно вызывать панику
}
