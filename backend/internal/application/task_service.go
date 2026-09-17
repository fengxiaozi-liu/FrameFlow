package application

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/interfaces/websocket"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type TaskService struct {
	Store       task.Repository
	Connections *websocket.Hub
	Enqueuer    interface {
		Enqueue(context.Context, task.Task) error
	}
	Providers ProviderService
}

func (s TaskService) Create(g *gin.Context) {
	ctx := g.Request.Context()
	var inputRequest struct {
		Kind         string `json:"kind"`
		ProviderCode string `json:"provider_code"`
		ModelID      string `json:"model_id"`
		task.Input
	}
	if err := g.Bind(&inputRequest); err != nil {
		return
	}
	kind := inputRequest.Kind
	if kind == "" {
		kind = string(task.KindVideo)
	}
	providerCode := inputRequest.ProviderCode
	modelID := inputRequest.ModelID
	if providerCode != "" {
		writeError(g, Invalid("invalid_task", errors.New("provider_code is no longer supported; select a model_id")))
		return
	}
	input := inputRequest.Input
	if err := input.Validate(); err != nil {
		writeError(g, Invalid("invalid_task", err))
		return
	}
	kindValue, err := task.ParseKind(kind)
	if err != nil {
		writeError(g, Invalid("invalid_task", err))
		return
	}
	if s.Providers.Catalog.Models != nil {
		cap := map[task.Kind]provider.Capability{task.KindStory: provider.Story, task.KindImage: provider.Image, task.KindVideo: provider.Video}[kindValue]
		if modelID == "" && providerCode == "" {
			models, err := s.Providers.Catalog.Models.ListModels(ctx, "")
			if err != nil {
				writeError(g, err)
				return
			}
			for _, model := range models {
				if model.Default && model.Supports(cap) {
					modelID = model.ID
					break
				}
			}
			if modelID == "" {
				writeError(g, Invalid("missing_default_model", errors.New("no default model is configured for this operation")))
				return
			}
		}
		if modelID != "" {
			model, err := s.Providers.Catalog.Models.GetModel(ctx, modelID)
			if err != nil {
				writeError(g, err)
				return
			}
			if !model.Supports(cap) {
				writeError(g, Invalid("unsupported_model", provider.ErrUnsupported))
				return
			}
			if cap == provider.Video && input.SourceImageURL == "" {
				writeError(g, Invalid("invalid_task", errors.New("image-to-video requires a source image")))
				return
			}
		}
	}
	now := time.Now().UTC()
	t := task.New(now.Format("20060102150405.000000000"), kindValue, input, now)
	t.ProviderCode = providerCode
	t.ModelID = modelID
	if err := s.Store.Save(ctx, t); err != nil {
		writeError(g, err)
		return
	}
	s.broadcast(ctx, t)
	if s.Enqueuer != nil {
		if err := s.Enqueuer.Enqueue(ctx, t); err != nil {
			writeError(g, s.rejectSubmission(ctx, t, err))
			return
		}
	}
	g.JSON(http.StatusAccepted, t)
}
func (s TaskService) Cancel(g *gin.Context) {
	ctx := g.Request.Context()
	id := g.Param("id")
	t, err := s.Store.Get(ctx, id)
	if err != nil {
		writeError(g, err)
		return
	}
	if err := t.Cancel(time.Now().UTC()); err != nil {
		writeError(g, Conflict("task_not_cancellable", err))
		return
	}
	if err := s.Store.Save(ctx, t); err != nil {
		writeError(g, err)
		return
	}
	if canceller, ok := s.Enqueuer.(interface {
		CancelTask(context.Context, string) error
	}); ok {
		if err := canceller.CancelTask(ctx, id); err != nil {
			writeError(g, err)
			return
		}
	}
	s.broadcast(ctx, t)
	g.JSON(http.StatusOK, t)
}
func (s TaskService) Retry(g *gin.Context) {
	ctx := g.Request.Context()
	id := g.Param("id")
	t, err := s.Store.Get(ctx, id)
	if err != nil {
		writeError(g, err)
		return
	}
	if err := t.Retry(time.Now().UTC()); err != nil {
		writeError(g, Conflict("task_not_retryable", err))
		return
	}
	if err := s.Store.Save(ctx, t); err != nil {
		writeError(g, err)
		return
	}
	s.broadcast(ctx, t)
	if s.Enqueuer != nil {
		if err := s.Enqueuer.Enqueue(ctx, t); err != nil {
			writeError(g, s.rejectSubmission(ctx, t, err))
			return
		}
	}
	g.JSON(http.StatusAccepted, t)
}
func (s TaskService) Get(g *gin.Context) {
	item, err := s.Store.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	g.JSON(http.StatusOK, item)
}
func (s TaskService) List(g *gin.Context) {
	items, err := s.Store.List(g.Request.Context())
	if err != nil {
		writeError(g, err)
		return
	}
	limit, _ := strconv.Atoi(g.Query("limit"))
	offset, _ := strconv.Atoi(g.Query("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	g.JSON(http.StatusOK, gin.H{
		"tasks": items[offset:end],
		"total": len(items),
	})
}
func (s TaskService) Delete(g *gin.Context) {
	ctx := g.Request.Context()
	id := g.Param("id")
	_, err := s.Store.Get(ctx, id)
	if err == nil {
		err = s.Store.Delete(ctx, id)
	}
	if err != nil {
		writeError(g, err)
		return
	}
	g.Status(http.StatusNoContent)
}
func (s TaskService) Result(g *gin.Context) {
	id := g.Param("id")
	item, err := s.Store.Get(g.Request.Context(), id)
	if err == nil && item.Status != task.StatusSucceeded {
		err = Conflict("result_not_ready", errors.New("task result is not ready"))
	}
	if err != nil {
		writeError(g, err)
		return
	}
	g.Header("Content-Disposition", `attachment; filename="frameflow-result-`+id+`.json"`)
	g.JSON(http.StatusOK, item)
}
func (s TaskService) rejectSubmission(ctx context.Context, t task.Task, cause error) error {
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer stop()
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		_ = t.Cancel(time.Now().UTC())
	} else {
		t.Fail(cause.Error(), time.Now().UTC())
	}
	if err := s.Store.Save(cleanup, t); err != nil {
		return errors.Join(cause, err)
	}
	s.broadcast(cleanup, t)
	return cause
}
func (s TaskService) broadcast(ctx context.Context, t task.Task) {
	if s.Connections != nil {
		s.Connections.Broadcast(ctx, task.Event{
			TaskID:   t.ID,
			Status:   t.Status,
			Progress: t.Progress,
			Stage:    t.Stage,
			At:       t.UpdatedAt,
		})
	}
}
