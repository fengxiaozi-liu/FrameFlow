package websocket

import (
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TaskEvents is the transport boundary for task event streaming. The concrete
// upgrader is intentionally isolated here so the domain never imports WebSocket code.
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == r.Host || u.Host == "127.0.0.1:5173" || u.Host == "localhost:5173"
}}

type eventStore interface {
	queue.Store
	task.EventRepository
}

func TaskEvents(store eventStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
		id = strings.TrimSuffix(id, "/events")
		_, ok := store.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		var after int64
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				for _, event := range store.ListEvents(id, after) {
					b, _ := json.Marshal(event)
					if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
						return
					}
					after = event.Sequence
					if event.Status == task.StatusSucceeded || event.Status == task.StatusFailed || event.Status == task.StatusCancelled {
						return
					}
				}
			}
		}
	}
}
