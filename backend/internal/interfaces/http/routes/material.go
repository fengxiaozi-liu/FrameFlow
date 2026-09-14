package routes

import "github.com/gin-gonic/gin"

func registerMaterials(engine *gin.Engine, deps Dependencies) {
	engine.Any("/api/materials", wrap(deps.Materials))
	engine.Any("/api/materials/:id", wrap(deps.Material))
	engine.Any("/api/materials/:id/*action", wrap(deps.Material))
}
