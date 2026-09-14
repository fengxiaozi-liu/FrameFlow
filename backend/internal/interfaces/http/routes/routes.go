package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Health, Metrics, Tasks, Task, Projects, Project, Overview, Providers, Provider, Materials, Material http.Handler
	Media, SPA                                                                                          http.Handler
}

func Register(engine *gin.Engine, deps Dependencies) {
	registerSystem(engine, deps)
	registerTasks(engine, deps)
	registerProjects(engine, deps)
	registerProviders(engine, deps)
	registerMaterials(engine, deps)
	registerStatic(engine, deps)
}

func wrap(handler http.Handler) gin.HandlerFunc {
	return gin.WrapH(handler)
}
