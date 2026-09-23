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
	taskRoutes.GET("/:id/download", tasks.Download)

	projectRoutes := api.Group("/projects")
	projectRoutes.GET("", projects.List)
	projectRoutes.POST("", projects.Create)
	projectRoutes.GET("/:id", projects.Get)
	projectRoutes.POST("/:id/drafts", projects.SaveDraft)
	projectRoutes.PUT("/:id/drafts/:draftId", projects.SaveDraftRevision)
	projectRoutes.GET("/:id/drafts/:draftId", projects.GetDraft)
	projectRoutes.POST("/:id/drafts/:draftId/candidates", tasks.CreateCandidate)
	projectRoutes.GET("/:id/drafts/:draftId/candidates", projects.ListCandidates)
	projectRoutes.GET("/:id/drafts/:draftId/candidates/:candidateId", projects.GetCandidate)
	projectRoutes.POST("/:id/drafts/:draftId/candidates/:candidateId/apply", projects.ApplyCandidate)
	projectRoutes.POST("/:id/drafts/:draftId/restore", projects.RestoreDraft)
	projectRoutes.POST("/:id/drafts/:draftId/scenes", projects.WriteScene)
	projectRoutes.PATCH("/:id/drafts/:draftId/scenes/:sceneId", projects.WriteScene)
	projectRoutes.DELETE("/:id/drafts/:draftId/scenes/:sceneId", projects.DeleteScene)
	projectRoutes.PUT("/:id/drafts/:draftId/scenes/:sceneId/bindings", projects.WriteBindings)
	projectRoutes.POST("/:id/drafts/:draftId/scenes/:sceneId/video-validation", tasks.ValidateSceneVideo)
	projectRoutes.POST("/:id/drafts/:draftId/scenes/:sceneId/video-tasks", tasks.CreateSceneVideo)
	projectRoutes.POST("/:id/drafts/:draftId/video-tasks/batch", tasks.CreateBatchVideo)
	projectRoutes.GET("/:id/drafts/:draftId/scenes/:sceneId/versions", projects.ListSceneVersions)
	projectRoutes.PUT("/:id/drafts/:draftId/scenes/:sceneId/selected-version", projects.SelectSceneVersion)
	projectRoutes.POST("/:id/drafts/:draftId/compositions", tasks.CreateComposition)
	projectRoutes.GET("/:id/drafts/:draftId/compositions", projects.ListCompositions)
	projectRoutes.GET("/:id/drafts/:draftId/compositions/:compositionId/download", tasks.DownloadComposition)

	providerRoutes := api.Group("/providers")
	providerRoutes.GET("", providers.List)
	providerRoutes.POST("", providers.Save)
	providerRoutes.GET("/:id", providers.Get)
	providerRoutes.PUT("/:id", providers.Save)
	providerRoutes.DELETE("/:id", providers.Delete)
	providerRoutes.POST("/:id/test", providers.TestConnection)
	connectionRoutes := api.Group("/connections")
	connectionRoutes.GET("", providers.Catalog.ListConnections)
	connectionRoutes.POST("", providers.Catalog.SaveConnection)
	connectionRoutes.PUT("/:id", providers.Catalog.SaveConnection)
	connectionRoutes.DELETE("/:id", providers.Catalog.DeleteConnection)
	connectionRoutes.POST("/:id/test", providers.Catalog.TestConnection)
	connectionRoutes.GET("/:id/models", providers.Catalog.ListModels)
	connectionRoutes.POST("/:id/models", providers.Catalog.AddModel)
	connectionRoutes.POST("/:id/models/sync", providers.Catalog.SyncModels)
	connectionRoutes.PATCH("/:id/models/:model", providers.Catalog.UpdateModel)
	connectionRoutes.DELETE("/:id/models/:model", providers.Catalog.DeleteModel)
	api.GET("/models", providers.Catalog.ListModels)
	api.GET("/models/:model/video-capability", providers.Catalog.VideoCapability)

	materialRoutes := api.Group("/materials")
	materialRoutes.GET("", materials.List)
	materialRoutes.POST("", materials.Save)
	materialRoutes.GET("/:id", materials.Get)
	materialRoutes.GET("/:id/uses", materials.Usage)
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
		c.Status(http.StatusOK)
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
