package http

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"net/http"
	"strconv"
	"strings"
)

func TaskEvents(events task.EventRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, 405, "method_not_allowed", errors.New("method not allowed"))
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
		id := strings.TrimSuffix(path, "/event-log")
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		writeJSON(w, 200, map[string]any{"events": events.ListEvents(id, after)})
	}
}
