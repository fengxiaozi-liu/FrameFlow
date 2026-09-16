package response

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/gin-gonic/gin"
	"net/http"
)

const responseKey = "frameflow.response"

type response struct {
	data   any
	err    error
	status int
}

func Set(c *gin.Context, status int, data any, err error) {
	c.Set(responseKey, response{status: status, data: data, err: err})
}
func Error(c *gin.Context, status int, code string, err error) {
	Set(c, status, gin.H{"error": gin.H{"code": code, "message": err.Error()}}, nil)
}

// AfterWrite runs only after a successful response has been flushed.
func AfterWrite(c *gin.Context, fn func()) { c.Set("frameflow.after_response", fn) }

// JSONResponse is the only JSON writer for ordinary API responses.
// Streaming and file handlers bypass it by not setting a response.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.Request.Context().Err(); err != nil {
			Set(c, 0, nil, err)
			c.Abort()
		} else {
			c.Next()
		}
		value, ok := c.Get(responseKey)
		if !ok || c.Writer.Written() {
			return
		}
		result := value.(response)
		if err := c.Request.Context().Err(); err != nil {
			result.err = err
		}
		if result.err != nil {
			code, message := "internal_error", "internal server error"
			result.status = http.StatusInternalServerError
			var appErr *fault.Error
			switch {
			case errors.Is(result.err, context.Canceled):
				result.status = 408
				code = "request_cancelled"
				message = "request cancelled"
			case errors.Is(result.err, context.DeadlineExceeded):
				result.status = 504
				code = "deadline_exceeded"
				message = "request deadline exceeded"
			case errors.Is(result.err, fault.ErrNotFound):
				result.status = 404
				code = "not_found"
				message = result.err.Error()
			case errors.As(result.err, &appErr):
				code, message = appErr.Code, appErr.Error()
				switch appErr.Kind {
				case "too_large":
					result.status = 413
				case "unsupported_media":
					result.status = 415
				case "invalid":
					result.status = 400
				case "conflict":
					result.status = 409
				case "unavailable":
					result.status = 501
				case "provider":
					result.status = 422
				}
			}
			c.JSON(result.status, gin.H{"error": gin.H{"code": code, "message": message}})
			return
		}
		if result.status == http.StatusNoContent {
			c.Status(result.status)
			return
		}
		c.JSON(result.status, result.data)
		if fn, ok := c.Get("frameflow.after_response"); ok && result.status >= 200 && result.status < 300 {
			c.Writer.Flush()
			fn.(func())()
		}
	}
}
