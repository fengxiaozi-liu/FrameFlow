package routes

import "github.com/gin-gonic/gin"

func registerSystem(engine *gin.Engine, deps Dependencies) {
	engine.Any("/health", wrap(deps.Health))
	engine.Any("/metrics", wrap(deps.Metrics))
	engine.Any("/api/overview", wrap(deps.Overview))
	engine.POST("/api/system/shutdown", wrap(deps.Shutdown))
}
