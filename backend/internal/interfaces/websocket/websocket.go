package websocket

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{HandshakeTimeout: 5 * time.Second, CheckOrigin: func(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && (u.Host == r.Host || u.Host == "127.0.0.1:5173" || u.Host == "localhost:5173")
}}

// Each session owns one live connection. Writes from processors are serialized.
type client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

var clientsMu sync.RWMutex
var clients = make(map[string]*client)

func remove(id string, current *client) {
	clientsMu.Lock()
	if clients[id] == current {
		delete(clients, id)
	}
	clientsMu.Unlock()
	_ = current.conn.Close()
}

// Send resolves the current connection, including after a reconnect.
// Offline sessions rely on HTTP catch-up; notification errors do not fail tasks.
func Send(ctx context.Context, sessionID string, event task.Event) error {
	clientsMu.RLock()
	current := clients[sessionID]
	clientsMu.RUnlock()
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
	if err := current.conn.WriteJSON(event); err != nil {
		remove(sessionID, current)
		return err
	}
	return nil
}

func CloseAll() {
	clientsMu.Lock()
	previous := clients
	clients = make(map[string]*client)
	clientsMu.Unlock()
	for _, current := range previous {
		_ = current.conn.Close()
	}
}

// Handler stays in the request goroutine, reading until the connection closes.
func Handler(c *gin.Context) {
	id := c.Query("session_id")
	if id == "" || len(id) > 128 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "session_id must contain 1 to 128 bytes"})
		return
	}
	// Register atomically with the handshake: Send cannot miss a newly opened session.
	clientsMu.Lock()
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		clientsMu.Unlock()
		return
	}
	current := &client{conn: conn}
	previous := clients[id]
	clients[id] = current
	clientsMu.Unlock()
	if previous != nil {
		_ = previous.conn.Close()
	}
	defer remove(id, current)
	conn.SetReadLimit(1024)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
