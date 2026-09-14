package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHub struct {
	clients    map[*websocket.Conn]bool
	mu         sync.RWMutex
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

type WSMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

func NewWSHub() *WSHub {
	hub := &WSHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
	go hub.run()
	return hub
}

func (h *WSHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			slog.Info("WS client connected", "total", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
			slog.Info("WS client disconnected", "total", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					h.unregister <- client
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *WSHub) BroadcastEvent(event string, data interface{}) {
	msg := WSMessage{Event: event, Data: data}
	jsonData, err := json.Marshal(msg)
	if err != nil {
		slog.Warn("WS marshal failed", "event", event, "error", err)
		return
	}
	h.broadcast <- jsonData
}

func (h *WSHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("WS upgrade failed", "error", err)
		return
	}

	h.register <- conn

	// Read loop (keep connection alive, handle pong)
	go func() {
		defer func() { h.unregister <- conn }()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}
