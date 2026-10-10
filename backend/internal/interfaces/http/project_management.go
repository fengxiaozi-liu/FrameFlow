package http

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/gin-gonic/gin"
	"net/http"
)

func deleteProject(useCase application.ProjectDeletion) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var input struct {
			Name           *string          `json:"expected_name"`
			CatalogVersion *int64           `json:"expected_catalog_version"`
			DraftVersions  map[string]int64 `json:"expected_draft_versions"`
		}
		if err := ctx.ShouldBindJSON(&input); err != nil || input.Name == nil || input.CatalogVersion == nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "expected_project_required", "message": "请提供项目及所有草稿的当前版本"}})
			return
		}
		err := useCase.Delete(ctx.Request.Context(), ctx.Param("id"), project.DeletionExpectation{Name: *input.Name, CatalogVersion: *input.CatalogVersion, DraftVersions: input.DraftVersions})
		if err != nil {
			status, code, message := http.StatusInternalServerError, "internal_error", "项目删除失败，请重试"
			var failure *fault.Error
			if errors.Is(err, fault.ErrNotFound) {
				status, code, message = http.StatusNotFound, "not_found", "项目不存在或已删除"
			} else if errors.As(err, &failure) {
				code, message = failure.Code, failure.Error()
				switch failure.Kind {
				case "invalid":
					status = http.StatusBadRequest
				case "conflict":
					status = http.StatusConflict
				case "unavailable":
					status = http.StatusServiceUnavailable
				}
			}
			ctx.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
			return
		}
		ctx.Status(http.StatusNoContent)
	}
}
