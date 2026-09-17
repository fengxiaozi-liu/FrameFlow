package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const heartbeatTimeout = 45 * time.Second

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && (u.Host == r.Host || u.Host == "127.0.0.1:5173" || u.Host == "localhost:5173")
	},
}

type client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]*client
	timeout time.Duration
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*client),
		timeout: heartbeatTimeout,
	}
}

var Connections = NewHub()

func (h *Hub) remove(id string, current *client) {
	h.mu.Lock()
	if h.clients[id] == current {
		delete(h.clients, id)
	}
	h.mu.Unlock()
	_ = current.conn.Close()
}

func (h *Hub) Send(ctx context.Context, sessionID string, data any) error {
	h.mu.RLock()
	current := h.clients[sessionID]
	h.mu.RUnlock()
	if current == nil {
		return nil
	}
	current.mu.Lock()
	defer current.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}
	_ = current.conn.SetWriteDeadline(deadline)
	if err := current.conn.WriteJSON(data); err != nil {
		h.remove(sessionID, current)
		return err
	}
	return nil
}

func Send(ctx context.Context, sessionID string, data any) error {
	return Connections.Send(ctx, sessionID, data)
}

func (h *Hub) Broadcast(ctx context.Context, data any) {
	h.mu.RLock()
	ids := make([]string, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	h.mu.RUnlock()
	for _, id := range ids {
		if err := h.Send(ctx, id, data); err != nil {
			log.Printf("websocket session %s: %v", id, err)
		}
	}
}

func (h *Hub) CloseAll() {
	h.mu.Lock()
	previous := h.clients
	h.clients = make(map[string]*client)
	h.mu.Unlock()
	for _, current := range previous {
		_ = current.conn.Close()
	}
}

func CloseAll() {
	Connections.CloseAll()
}

func (h *Hub) Handler(c *gin.Context) {
	var key [16]byte
	if _, err := rand.Read(key[:]); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	id := hex.EncodeToString(key[:])
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	current := &client{
		conn: conn,
	}
	h.mu.Lock()
	h.clients[id] = current
	h.mu.Unlock()
	defer h.remove(id, current)
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(h.timeout))
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType == websocket.TextMessage && string(message) == "ping" {
			_ = conn.SetReadDeadline(time.Now().Add(h.timeout))
		}
	}
}

func Handler(c *gin.Context) {
	Connections.Handler(c)
}
