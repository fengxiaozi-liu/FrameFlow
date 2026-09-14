package routes

import "github.com/gin-gonic/gin"

func registerProjects(engine *gin.Engine, deps Dependencies) {
	engine.Any("/api/projects", wrap(deps.Projects))
	engine.Any("/api/projects/:id", wrap(deps.Project))
	engine.Any("/api/projects/:id/*action", wrap(deps.Project))
}
