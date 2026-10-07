package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPingPeriod = 30 * time.Second
	// Pong is expected within 10s after a ping; read deadline covers the
	// full ping interval plus that grace.
	wsPongWait   = wsPingPeriod + 10*time.Second
	wsMaxMsgSize = 64 * 1024
	// Per-client outbound buffer; when full the client is dropped.
	wsSendBuffer = 64
)

// checkWSOrigin allows only known frontend origins (CSWSH protection).
// Same-origin is always allowed (browser on http://176.12.69.5:82 etc.);
// otherwise localhost dev ports and FRONTEND_ORIGIN from env.
func checkWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	// Non-browser clients send no Origin header.
	if origin == "" {
		return true
	}
	// Same-origin: Origin host:port matches the request Host.
	if u, err := url.Parse(origin); err == nil && u.Host != "" {
		if u.Host == r.Host {
			return true
		}
	}
	allowed := map[string]bool{
		"http://localhost:82":   true,
		"http://localhost:5173": true,
	}
	if fo := os.Getenv("FRONTEND_ORIGIN"); fo != "" {
		allowed[fo] = true
	}
	return allowed[origin]
}

var upgrader = websocket.Upgrader{
	CheckOrigin: checkWSOrigin,
}

// wsClient is one connection with a dedicated outbound buffer.
// Only writePump writes to the websocket.Conn (gorilla allows a single writer).
type wsClient struct {
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

// close is idempotent: closes the send channel and the connection once.
func (c *wsClient) close() {
	c.closeOnce.Do(func() {
		close(c.send)
		if c.conn != nil {
			c.conn.Close()
		}
	})
}

type WSHub struct {
	clients    map[*wsClient]bool
	mu         sync.Mutex
	broadcast  chan []byte
	register   chan *wsClient
	unregister chan *wsClient
}

type WSMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

func NewWSHub() *WSHub {
	hub := &WSHub{
		clients:    make(map[*wsClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
	}
	go hub.run()
	return hub
}

// removeClient drops the client from the set and closes it. Idempotent.
func (h *WSHub) removeClient(client *wsClient) {
	h.mu.Lock()
	_, ok := h.clients[client]
	if ok {
		delete(h.clients, client)
	}
	total := len(h.clients)
	h.mu.Unlock()
	if ok {
		client.close()
		slog.Info("WS client disconnected", "total", total)
	}
}

func (h *WSHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			total := len(h.clients)
			h.mu.Unlock()
			slog.Info("WS client connected", "total", total)

		case client := <-h.unregister:
			h.removeClient(client)

		case message := <-h.broadcast:
			// Non-blocking send per client: a slow/dead client must not
			// stall the hub (the old code sent to h.unregister from here
			// and deadlocked). Dead clients are removed after the loop.
			h.mu.Lock()
			var dead []*wsClient
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					dead = append(dead, client)
					delete(h.clients, client)
				}
			}
			total := len(h.clients)
			h.mu.Unlock()
			for _, client := range dead {
				client.close()
			}
			if len(dead) > 0 {
				slog.Warn("WS dropped slow clients", "dropped", len(dead), "total", total)
			}
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

// writePump is the only writer to the connection.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				// Hub closed the channel — client is being removed.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump keeps the connection alive, enforces read limits and pong deadlines.
func (c *wsClient) readPump(h *WSHub) {
	defer func() { h.unregister <- c }()
	c.conn.SetReadLimit(wsMaxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (h *WSHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("WS upgrade failed", "error", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, wsSendBuffer),
	}
	h.register <- client

	go client.writePump()
	go client.readPump(h)
}
