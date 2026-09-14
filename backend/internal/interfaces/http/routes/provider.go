package routes

import "github.com/gin-gonic/gin"

func registerProviders(engine *gin.Engine, deps Dependencies) {
	engine.Any("/api/providers", wrap(deps.Providers))
	engine.Any("/api/providers/:id", wrap(deps.Provider))
	engine.Any("/api/providers/:id/*action", wrap(deps.Provider))
}
