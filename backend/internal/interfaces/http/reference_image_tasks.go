package http

import (
	"context"
	"errors"

	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
)

type referenceImageCommand interface {
	SubmitReferenceImage(context.Context, string, string, application.ReferenceImageRequest) (task.Task, error)
}

func createReferenceImage(service referenceImageCommand) gin.HandlerFunc {
	return func(g *gin.Context) {
		var in application.ReferenceImageRequest
		if err := g.ShouldBindJSON(&in); err != nil {
			g.JSON(400, gin.H{"error": gin.H{"code": "invalid_reference_image", "message": "图片生成请求格式无效"}})
			return
		}
		item, err := service.SubmitReferenceImage(g.Request.Context(), g.Param("id"), g.Param("draftId"), in)
		if err != nil {
			status, code, message := 500, "internal_error", "图片生成任务提交失败"
			var e *fault.Error
			switch {
			case errors.As(err, &e):
				code, message = e.Code, e.Error()
				switch e.Kind {
				case "invalid":
					status = 400
				case "conflict":
					status = 409
				case "unavailable":
					status = 503
				}
			case errors.Is(err, fault.ErrNotFound):
				status, code, message = 404, "not_found", "项目、模型或图片不存在"
			case errors.Is(err, context.DeadlineExceeded):
				status = 504
			case errors.Is(err, context.Canceled):
				status = 408
			}
			g.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
			return
		}
		g.JSON(202, item)
	}
}
