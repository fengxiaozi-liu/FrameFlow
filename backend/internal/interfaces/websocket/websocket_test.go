package websocket

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func testHub(t *testing.T, timeout time.Duration) (*Hub, func() *websocket.Conn) {
	t.Helper()
	h := NewHub()
	h.timeout = timeout
	router := gin.New()
	router.GET("/ws", h.Handler)
	server := httptest.NewServer(router)
	t.Cleanup(func() { h.CloseAll(); server.Close() })
	return h, func() *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		return conn
	}
}

func sessionIDs(h *Hub) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

func TestSessionSendAndBroadcast(t *testing.T) {
	h, dial := testHub(t, 5*time.Second)
	first, second := dial(), dial()
	ids := sessionIDs(h)
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatalf("expected distinct server-generated IDs: %v", ids)
	}
	if err := h.Send(context.Background(), ids[0], map[string]string{"message": "private"}); err != nil {
		t.Fatal(err)
	}
	// Both connections receive the broadcast, regardless of which received the directed message.
	h.Broadcast(context.Background(), map[string]string{"message": "all"})
	for _, conn := range []*websocket.Conn{first, second} {
		var message map[string]string
		if err := conn.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		if message["message"] == "private" {
			if err := conn.ReadJSON(&message); err != nil {
				t.Fatal(err)
			}
		}
		if message["message"] != "all" {
			t.Fatal(message)
		}
	}
	var writers sync.WaitGroup
	for i := 0; i < 20; i++ {
		writers.Add(1)
		go func(i int) {
			defer writers.Done()
			h.Broadcast(context.Background(), map[string]string{"message": fmt.Sprint(i)})
		}(i)
	}
	for _, conn := range []*websocket.Conn{first, second} {
		seen := make(map[string]bool)
		for i := 0; i < 20; i++ {
			var message map[string]string
			if err := conn.ReadJSON(&message); err != nil {
				t.Fatal(err)
			}
			seen[message["message"]] = true
		}
		if len(seen) != 20 {
			t.Fatal("lost concurrent broadcast")
		}
	}
	writers.Wait()
	h.CloseAll()
	if _, _, err := first.ReadMessage(); err == nil {
		t.Fatal("shutdown did not close connection")
	}
}

func TestHeartbeatExpiresAndKeepsLiveSession(t *testing.T) {
	h, dial := testHub(t, 120*time.Millisecond)
	stale, live := dial(), dial()
	for i := 0; i < 5; i++ {
		if err := live.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
			t.Fatal(err)
		}
		time.Sleep(40 * time.Millisecond)
	}
	if _, _, err := stale.ReadMessage(); err == nil {
		t.Fatal("stale session remained open")
	}
	if ids := sessionIDs(h); len(ids) != 1 {
		t.Fatalf("expected one live session: %v", ids)
	}
	h.Broadcast(context.Background(), "still live")
	var message string
	if err := live.ReadJSON(&message); err != nil || message != "still live" {
		t.Fatal(message, err)
	}
}
