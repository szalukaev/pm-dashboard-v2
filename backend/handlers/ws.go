package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"pm-dashboard/middleware"

	"github.com/gorilla/websocket"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPingPeriod = 30 * time.Second
	// Pong ожидается в течение 10 с после ping; срок чтения покрывает весь
	// интервал между ping и этот запас.
	wsPongWait   = wsPingPeriod + 10*time.Second
	wsMaxMsgSize = 64 * 1024
	// Буфер исходящих сообщений клиента; при переполнении клиент отключается.
	wsSendBuffer = 64
)

// checkWSOrigin пропускает только известные адреса фронтенда (защита от
// CSWSH). Тот же адрес, что у сервера, разрешён всегда (браузер на
// http://176.12.69.5:82 и т. п.); кроме него — локальные порты разработки
// и FRONTEND_ORIGIN из окружения.
func checkWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	// Клиенты, не являющиеся браузером, заголовок Origin не присылают.
	if origin == "" {
		return true
	}
	// Тот же адрес: хост и порт в Origin совпадают с Host запроса.
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

// wsClient — одно соединение со своим буфером исходящих сообщений. В
// websocket.Conn пишет только writePump (gorilla допускает одного писателя).
type wsClient struct {
	// userID — чьё это соединение: по нему уходят личные события.
	userID    int
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

// close можно вызывать повторно: канал отправки и соединение закрываются один раз.
func (c *wsClient) close() {
	c.closeOnce.Do(func() {
		close(c.send)
		if c.conn != nil {
			_ = c.conn.Close()
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

// removeClient убирает клиента из набора и закрывает его. Вызывать можно повторно.
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
			// Отправка каждому клиенту без блокировки: медленный или
			// мёртвый клиент не должен останавливать хаб (прежний код
			// отправлял отсюда в h.unregister и зависал). Мёртвые клиенты
			// убираются после цикла.
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

// SendToUser отправляет событие только открытым страницам одного
// пользователя. Клиент, не успевающий читать, пропускает событие: оно
// останется в журнале и будет прочитано при следующей загрузке.
func (h *WSHub) SendToUser(userID int, event string, data interface{}) {
	jsonData, err := json.Marshal(WSMessage{Event: event, Data: data})
	if err != nil {
		slog.Warn("WS marshal failed", "event", event, "error", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		if client.userID != userID {
			continue
		}
		select {
		case client.send <- jsonData:
		default:
		}
	}
}

// DisconnectUser закрывает все открытые соединения пользователя: после
// блокировки, удаления или сброса пароля он не должен получать события.
// Возвращает число закрытых соединений.
func (h *WSHub) DisconnectUser(userID int) int {
	h.mu.Lock()
	var gone []*wsClient
	for client := range h.clients {
		if client.userID == userID {
			gone = append(gone, client)
			delete(h.clients, client)
		}
	}
	h.mu.Unlock()
	for _, client := range gone {
		client.close()
	}
	if len(gone) > 0 {
		slog.Info("WS user disconnected", "user", userID, "connections", len(gone))
	}
	return len(gone)
}

// writePump — единственный, кто пишет в соединение.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				// Хаб закрыл канал — клиента убирают.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump поддерживает соединение, следит за пределом чтения и сроками pong.
func (c *wsClient) readPump(h *WSHub) {
	defer func() { h.unregister <- c }()
	c.conn.SetReadLimit(wsMaxMsgSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
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
		userID: middleware.GetUserID(r),
		conn:   conn,
		send:   make(chan []byte, wsSendBuffer),
	}
	h.register <- client

	go client.writePump()
	go client.readPump(h)
}
