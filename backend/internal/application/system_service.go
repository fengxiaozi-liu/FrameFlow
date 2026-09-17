package application

import (
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
	"net/http"
)

type SystemService struct {
	QueueDepth   func() int
	RequestCount func() uint64
	Tasks        TaskService
	Projects     ProjectService
	Providers    ProviderService
	Materials    MaterialService
	Shutdown     func()
}

func (s SystemService) Health(g *gin.Context) {
	g.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "frameflow",
	})
}
func (s SystemService) Overview(g *gin.Context) {
	ctx := g.Request.Context()
	projects, err := s.Projects.Repo.List(ctx)
	if err != nil {
		writeError(g, err)
		return
	}
	tasks, err := s.Tasks.Store.List(ctx)
	if err != nil {
		writeError(g, err)
		return
	}
	counts := map[string]int{
		"projects":      len(projects),
		"tasks":         len(tasks),
		"running_tasks": 0,
		"providers":     0,
		"materials":     0,
	}
	for _, item := range tasks {
		if item.Status == task.StatusQueued || item.Status == task.StatusRunning {
			counts["running_tasks"]++
		}
	}
	for _, kind := range []provider.Capability{provider.Story, provider.Image, provider.Video} {
		items, err := s.Providers.Repo.List(ctx, kind)
		if err != nil {
			writeError(g, err)
			return
		}
		counts["providers"] += len(items)
	}
	for _, kind := range []material.Kind{material.Visual, material.Frame, material.Character, material.Voice, material.Music} {
		items, err := s.Materials.Repo.List(ctx, kind)
		if err != nil {
			writeError(g, err)
			return
		}
		counts["materials"] += len(items)
	}
	g.JSON(http.StatusOK, counts)
}
func (s SystemService) Stop(g *gin.Context) {
	if s.Shutdown == nil {
		writeError(g, unavailable("shutdown_unavailable", "shutdown is unavailable"))
		return
	}
	g.JSON(http.StatusAccepted, gin.H{
		"status": "shutting_down",
	})
	g.Writer.Flush()
	s.Shutdown()
}
func (s SystemService) Metrics(g *gin.Context) {
	items, err := s.Tasks.Store.List(g.Request.Context())
	if err != nil {
		writeError(g, err)
		return
	}
	g.Header("Content-Type", "text/plain; version=0.0.4")
	counts := map[string]int{}
	for _, item := range items {
		counts[string(item.Status)]++
	}
	depth := 0
	if s.QueueDepth != nil {
		depth = s.QueueDepth()
	}
	var requests uint64
	if s.RequestCount != nil {
		requests = s.RequestCount()
	}
	_, _ = fmt.Fprintf(g.Writer, "frameflow_up 1\nframeflow_http_requests_total %d\nframeflow_queue_depth %d\n", requests, depth)
	for _, state := range []string{"queued", "running", "succeeded", "failed", "cancelled"} {
		_, _ = fmt.Fprintf(g.Writer, "frameflow_tasks{status=%q} %d\n", state, counts[state])
	}
}
