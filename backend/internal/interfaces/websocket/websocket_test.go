package websocket

import (
	"context"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSessionConnections(t *testing.T) {
	router := gin.New()
	router.GET("/ws", Handler)
	server := httptest.NewServer(router)
	defer server.Close()
	defer CloseAll()
	dial := func(id string) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws?session_id="+id, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		return conn
	}
	first, second := dial("one"), dial("two")
	send := func(id, taskID string) {
		t.Helper()
		if err := Send(context.Background(), id, task.Event{TaskID: taskID, Status: task.StatusSucceeded}); err != nil {
			t.Fatal(err)
		}
	}
	read := func(conn *websocket.Conn, taskID string) {
		t.Helper()
		var event task.Event
		if err := conn.ReadJSON(&event); err != nil || event.TaskID != taskID {
			t.Fatalf("event=%+v err=%v", event, err)
		}
	}
	send("one", "first")
	send("two", "second")
	read(first, "first")
	read(second, "second")
	send("one", "another-task")
	read(first, "another-task")
	clientsMu.RLock()
	previous := clients["one"]
	clientsMu.RUnlock()
	replacement := dial("one")
	remove("one", previous) // Delayed cleanup from the old handler must preserve the replacement.
	send("one", "reconnected")
	read(replacement, "reconnected")
	if _, _, err := first.ReadMessage(); err == nil {
		t.Fatal("old connection still open")
	}
	var writers sync.WaitGroup
	for i := 0; i < 20; i++ {
		writers.Add(1)
		go func(i int) {
			defer writers.Done()
			if err := Send(context.Background(), "one", task.Event{TaskID: fmt.Sprint(i)}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		var event task.Event
		if err := replacement.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		seen[event.TaskID] = true
	}
	writers.Wait()
	if len(seen) != 20 {
		t.Fatal("lost concurrent events")
	}
	replacement.Close()
	deadline := time.Now().Add(time.Second)
	for {
		clientsMu.RLock()
		_, ok := clients["one"]
		clientsMu.RUnlock()
		if !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disconnected session retained")
		}
		time.Sleep(time.Millisecond)
	}
	send("one", "offline")
	CloseAll()
	if _, _, err := second.ReadMessage(); err == nil {
		t.Fatal("shutdown did not close connection")
	}
}

func TestHandlerRequiresSession(t *testing.T) {
	router := gin.New()
	router.GET("/ws", Handler)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/ws", nil))
	if response.Code != 400 {
		t.Fatal(response.Code)
	}
}
