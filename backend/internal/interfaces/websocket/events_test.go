package websocket

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/sqlite"
	"github.com/gorilla/websocket"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskEventStream(t *testing.T) {
	store, e := sqlite.Open(filepath.Join(t.TempDir(), "events.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	v := task.New("t1", task.KindVideo, task.Input{Prompt: "测试视频"}, time.Now())
	_ = store.Save(v)
	_ = store.AppendEvent(task.Event{TaskID: v.ID, Status: v.Status, Stage: v.Stage, At: v.CreatedAt})
	server := httptest.NewServer(TaskEvents(store))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/tasks/t1/events"
	conn, _, e := websocket.DefaultDialer.Dial(url, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var event task.Event
	if e = conn.ReadJSON(&event); e != nil || event.TaskID != "t1" {
		t.Fatal(e, event)
	}
}
