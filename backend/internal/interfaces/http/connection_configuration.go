package http

import (
	"context"
	"errors"

	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/gin-gonic/gin"
)

type ConnectionConfigurationCommand interface {
	Save(context.Context, provider.Connection, string, bool) (provider.Connection, error)
}

type ModelDiscoveryCommand interface {
	Sync(context.Context, string) (int, error)
}

type CatalogCommands struct {
	Connections ConnectionConfigurationCommand
	Discovery   ModelDiscoveryCommand
}

// Connection commands live in the application layer; this adapter only maps
// HTTP input and errors. Existing catalog handlers remain for unrelated routes.
func saveConnection(service ConnectionConfigurationCommand) gin.HandlerFunc {
	return func(g *gin.Context) {
		var in struct {
			provider.Connection
			APIKey string `json:"api_key"`
		}
		if err := g.ShouldBindJSON(&in); err != nil {
			g.JSON(400, gin.H{"error": gin.H{"code": "invalid_connection", "message": "连接配置格式无效"}})
			return
		}
		if g.Param("id") != "" {
			in.ID = g.Param("id")
		}
		c, err := service.Save(g.Request.Context(), in.Connection, in.APIKey, g.Param("id") == "")
		if err != nil {
			status := 400
			if errors.Is(err, provider.ErrRevisionConflict) {
				status = 409
			}
			g.JSON(status, gin.H{"error": gin.H{"code": "invalid_connection", "message": err.Error()}})
			return
		}
		status := 201
		if g.Param("id") != "" {
			status = 200
		}
		g.JSON(status, c)
	}
}

func syncConnectionModels(catalog application.CatalogService, service ModelDiscoveryCommand) gin.HandlerFunc {
	return func(g *gin.Context) {
		c, err := catalog.Connections.GetConnection(g.Request.Context(), g.Param("id"))
		if err != nil {
			g.JSON(404, gin.H{"error": gin.H{"message": "连接不存在"}})
			return
		}
		if c.Vendor != "custom" {
			catalog.SyncModels(g)
			return
		}
		count, err := service.Sync(g.Request.Context(), c.ID)
		if err != nil {
			status := 422
			if errors.Is(err, provider.ErrRevisionConflict) {
				status = 409
			}
			g.JSON(status, gin.H{"error": gin.H{"code": "discovery_failed", "message": err.Error()}})
			return
		}
		g.JSON(200, gin.H{"synced": count})
	}
}
