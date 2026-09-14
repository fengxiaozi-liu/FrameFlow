package routes

import "github.com/gin-gonic/gin"

func registerStatic(engine *gin.Engine, deps Dependencies) {
	engine.Any("/media/*path", wrap(deps.Media))
	engine.NoRoute(wrap(deps.SPA))
}
