package http

import (
	"errors"
	"net/http"

	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/gin-gonic/gin"
)

func deleteGenerationRun(useCase application.GenerationRunDeletion) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if err := useCase.Delete(ctx.Request.Context(), ctx.Param("id"), ctx.Param("draftId"), ctx.Param("sceneId"), ctx.Param("runId")); err != nil {
			status, code, message := http.StatusInternalServerError, "internal_error", "任务删除失败，请重试"
			var failure *fault.Error
			if errors.Is(err, fault.ErrNotFound) {
				status, code, message = http.StatusNotFound, "not_found", "任务不存在或已删除"
			} else if errors.As(err, &failure) {
				code, message = failure.Code, failure.Error()
				if failure.Kind == "conflict" {
					status = http.StatusConflict
				} else if failure.Kind == "unavailable" {
					status = http.StatusServiceUnavailable
				}
			}
			ctx.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
			return
		}
		ctx.Status(http.StatusNoContent)
	}
}
