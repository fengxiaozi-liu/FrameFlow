package http

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/gin-gonic/gin"
)

func modelProtocols(g *gin.Context) { g.JSON(200, gin.H{"protocols": provider.Protocols()}) }
func configureModel(models provider.ModelRepository) gin.HandlerFunc {
	return func(g *gin.Context) {
		var in struct {
			Capabilities     []provider.Capability                             `json:"capabilities"`
			ExpectedRevision int64                                             `json:"expected_revision"`
			Operations       map[provider.Capability]provider.OperationBinding `json:"operations"`
		}
		if err := g.ShouldBindJSON(&in); err != nil || in.Operations == nil {
			g.JSON(422, gin.H{"error": gin.H{"code": "invalid_configuration", "message": "请提交操作协议与参数规格"}})
			return
		}
		repo, ok := models.(provider.OperationRepository)
		if !ok {
			g.JSON(503, gin.H{"error": gin.H{"message": "配置仓储不可用"}})
			return
		}
		m, err := (application.ModelConfiguration{Repository: repo}).Configure(g.Request.Context(), g.Param("model"), in.ExpectedRevision, in.Operations, in.Capabilities)
		if err != nil {
			status := 422
			if errors.Is(err, provider.ErrRevisionConflict) {
				status = 409
			}
			g.JSON(status, gin.H{"error": gin.H{"code": "configuration_error", "message": err.Error()}})
			return
		}
		g.JSON(200, m)
	}
}
