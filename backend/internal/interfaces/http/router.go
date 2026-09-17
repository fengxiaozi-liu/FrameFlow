package http

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/application"
	"github.com/gin-gonic/gin"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func UseMiddleware(engine *gin.Engine, token string, rateLimit int) {
	engine.Use(audit(), requestContext(), limitJSONBody(), cors(), newLimiter(rateLimit).middleware(), auth(token))
	engine.HandleMethodNotAllowed = true
}

// RegisterRoutes registers API groups with the application service methods.
func RegisterRoutes(
	engine *gin.Engine,
	tasks application.TaskService,
	projects application.ProjectService,
	providers application.ProviderService,
	materials application.MaterialService,
	system application.SystemService,
) {
	system.RequestCount = requestCount.Load
	engine.GET("/health", system.Health)
	engine.GET("/metrics", system.Metrics)
	api := engine.Group("/api")
	api.GET("/overview", system.Overview)
	api.Group("/system").POST("/shutdown", system.Stop)

	taskRoutes := api.Group("/tasks")
	taskRoutes.GET("", tasks.List)
	taskRoutes.POST("", tasks.Create)
	taskRoutes.GET("/:id", tasks.Get)
	taskRoutes.DELETE("/:id", tasks.Delete)
	taskRoutes.POST("/:id/cancel", tasks.Cancel)
	taskRoutes.POST("/:id/retry", tasks.Retry)
	taskRoutes.GET("/:id/result", tasks.Result)

	projectRoutes := api.Group("/projects")
	projectRoutes.GET("", projects.List)
	projectRoutes.POST("", projects.Create)
	projectRoutes.GET("/:id", projects.Get)
	projectRoutes.POST("/:id/drafts", projects.SaveDraft)

	providerRoutes := api.Group("/providers")
	providerRoutes.GET("", providers.List)
	providerRoutes.POST("", providers.Save)
	providerRoutes.GET("/:id", providers.Get)
	providerRoutes.PUT("/:id", providers.Save)
	providerRoutes.DELETE("/:id", providers.Delete)
	providerRoutes.POST("/:id/test", providers.TestConnection)

	materialRoutes := api.Group("/materials")
	materialRoutes.GET("", materials.List)
	materialRoutes.POST("", materials.Save)
	materialRoutes.GET("/:id", materials.Get)
	materialRoutes.PUT("/:id", materials.Save)
	materialRoutes.DELETE("/:id", materials.Delete)
	materialRoutes.POST("/upload", materials.Upload)
}

func RegisterStatic(engine *gin.Engine, uploadDir string, files fs.FS) {
	if uploadDir != "" {
		engine.StaticFS("/media", http.Dir(uploadDir))
	}
	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || files == nil {
			c.Status(404)
			return
		}
		spa(files).ServeHTTP(c.Writer, c.Request)
	})
}
func spa(files fs.FS) http.Handler {
	assets := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" {
			if _, err := fs.Stat(files, name); err == nil {
				assets.ServeHTTP(w, r)
				return
			}
		}
		content, err := fs.ReadFile(files, "index.html")
		if err != nil {
			http.Error(w, "frontend unavailable", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	})
}
