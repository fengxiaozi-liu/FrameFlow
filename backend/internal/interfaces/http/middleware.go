package http

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type visitor struct {
	window time.Time
	count  int
}
type limiter struct {
	mu    sync.Mutex
	limit int
	items map[string]visitor
}

func newLimiter(limit int) *limiter {
	if limit <= 0 {
		limit = 120
	}
	return &limiter{limit: limit, items: map[string]visitor{}}
}
func (l *limiter) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		now := time.Now()
		l.mu.Lock()
		v := l.items[host]
		if now.Sub(v.window) >= time.Minute {
			v = visitor{window: now}
		}
		v.count++
		l.items[host] = v
		allowed := v.count <= l.limit
		l.mu.Unlock()
		if !allowed {
			writeError(w, 429, "rate_limited", fmt.Errorf("request limit exceeded"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}
func (w *statusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

var requestCount atomic.Uint64

func audit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		requestCount.Add(1)
		log.Printf(`{"method":%q,"path":%q,"status":%d,"duration_ms":%d}`, r.Method, r.URL.Path, sw.status, time.Since(started).Milliseconds())
	})
}
func (s Server) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	counts := map[string]int{}
	for _, item := range s.TaskService.List() {
		counts[string(item.Status)]++
	}
	_, _ = fmt.Fprintf(w, "frameflow_up 1\nframeflow_http_requests_total %d\nframeflow_queue_depth %d\n", requestCount.Load(), s.Worker.Depth())
	for _, status := range []string{"queued", "running", "succeeded", "failed", "cancelled"} {
		_, _ = fmt.Fprintf(w, "frameflow_tasks{status=%q} %d\n", status, counts[status])
	}
}
