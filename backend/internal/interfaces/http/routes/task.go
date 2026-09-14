package routes

import "github.com/gin-gonic/gin"

func registerTasks(engine *gin.Engine, deps Dependencies) {
	engine.Any("/api/tasks", wrap(deps.Tasks))
	engine.Any("/api/tasks/:id", wrap(deps.Task))
	engine.Any("/api/tasks/:id/*action", wrap(deps.Task))
}
