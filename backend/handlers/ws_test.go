package handlers

import (
	"testing"
	"time"
)

// TestWSHubSurvivesFullSendBuffer ensures a slow client (full send buffer)
// does not deadlock the hub: the old code sent to h.unregister from inside
// run()'s broadcast branch and blocked forever on the first bad client.
func TestWSHubSurvivesFullSendBuffer(t *testing.T) {
	h := NewWSHub()

	// Client with a buffer of 1 that is already full.
	slow := &wsClient{send: make(chan []byte, 1)}
	h.register <- slow
	slow.send <- []byte(`{"event":"fill"}`) // fill the buffer

	// These must both complete without blocking.
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

	// Hub must still accept new clients and deliver messages.
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

	// The slow client must have been dropped.
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

// TestWSClientCloseIdempotent — close() must be safe from multiple paths
// (dead-list cleanup and unregister).
func TestWSClientCloseIdempotent(t *testing.T) {
	c := &wsClient{send: make(chan []byte, 1)}
	c.close()
	c.close() // must not panic on double-close
}
