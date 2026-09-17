package http

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
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
	return &limiter{
		limit: limit,
		items: map[string]visitor{},
	}
}
func (l *limiter) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		now := time.Now()
		l.mu.Lock()
		v := l.items[host]
		if now.Sub(v.window) >= time.Minute {
			v = visitor{
				window: now,
			}
		}
		v.count++
		l.items[host] = v
		allowed := v.count <= l.limit
		l.mu.Unlock()
		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "rate_limited",
					"message": "request limit exceeded",
				},
			})
			return
		}
		c.Next()
	}
}

var requestCount atomic.Uint64

func audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		requestCount.Add(1)
		log.Printf(`{"method":%q,"path":%q,"status":%d,"duration_ms":%d}`, c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(started).Milliseconds())
	}
}
func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://127.0.0.1:5173")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
func auth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.GetHeader("Authorization") == "Bearer "+token || c.Query("token") == token
		if token != "" && c.Request.URL.Path != "/health" && !provided {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "unauthorized",
					"message": "valid bearer token required",
				},
			})
			return
		}
		c.Next()
	}
}

func requestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := c.Request.Context().Err()
		if err != nil {
			status, code, message := http.StatusRequestTimeout, "request_cancelled", "request cancelled"
			if errors.Is(err, context.DeadlineExceeded) {
				status, code, message = http.StatusGatewayTimeout, "deadline_exceeded", "request deadline exceeded"
			}
			c.AbortWithStatusJSON(status, gin.H{
				"error": gin.H{
					"code":    code,
					"message": message,
				},
			})
			return
		}
		c.Next()
	}
}

func limitJSONBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		}
		c.Next()
	}
}
