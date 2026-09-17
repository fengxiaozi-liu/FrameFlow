package application

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/gin-gonic/gin"
)

type Error = fault.Error

func Invalid(code string, err error) error {
	return &Error{
		Kind:  "invalid",
		Code:  code,
		Cause: err,
	}
}

func Conflict(code string, err error) error {
	return &Error{
		Kind:  "conflict",
		Code:  code,
		Cause: err,
	}
}

func unavailable(code, message string) error {
	return &Error{
		Kind:  "unavailable",
		Code:  code,
		Cause: fmt.Errorf("%s", message),
	}
}

func writeError(ctx *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "internal server error"
	var appErr *fault.Error
	switch {
	case errors.Is(err, context.Canceled):
		status, code, message = http.StatusRequestTimeout, "request_cancelled", "request cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		status, code, message = http.StatusGatewayTimeout, "deadline_exceeded", "request deadline exceeded"
	case errors.Is(err, fault.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", err.Error()
	case errors.As(err, &appErr):
		code, message = appErr.Code, appErr.Error()
		switch appErr.Kind {
		case "too_large":
			status = http.StatusRequestEntityTooLarge
		case "unsupported_media":
			status = http.StatusUnsupportedMediaType
		case "invalid":
			status = http.StatusBadRequest
		case "conflict":
			status = http.StatusConflict
		case "unavailable":
			status = http.StatusNotImplemented
		case "provider":
			status = http.StatusUnprocessableEntity
		}
	}
	ctx.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
