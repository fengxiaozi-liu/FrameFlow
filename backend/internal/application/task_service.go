package application

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/request"
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/response"
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

type TaskService struct {
	Store     task.Repository
	Events    task.EventRepository
	SendEvent task.Sender
	Enqueuer  interface {
		Enqueue(context.Context, task.Task) error
	}
	Providers ProviderService
}

func (s TaskService) Create(g *gin.Context) {
	ctx := g.Request.Context()
	var inputRequest struct {
		SessionID    string `json:"session_id"`
		Kind         string `json:"kind"`
		ProviderCode string `json:"provider_code"`
		task.Input
	}
	if !request.BindJSON(g, &inputRequest) {
		return
	}
	if len(inputRequest.SessionID) > 128 {
		response.Set(g, 0, nil, Invalid("invalid_session", errors.New("session_id exceeds 128 bytes")))
		return
	}
	kind := inputRequest.Kind
	if kind == "" {
		kind = string(task.KindVideo)
	}
	providerCode := inputRequest.ProviderCode
	input := inputRequest.Input
	if err := input.Validate(); err != nil {
		response.Set(g, 202, task.Task{}, Invalid("invalid_task", err))
		return
	}
	kindValue, err := task.ParseKind(kind)
	if err != nil {
		response.Set(g, 202, task.Task{}, Invalid("invalid_task", err))
		return
	}
	if providerCode != "" {
		expected := map[task.Kind]provider.Capability{
			task.KindStory: provider.Story,
			task.KindImage: provider.Image,
			task.KindVideo: provider.Video,
		}[kindValue]
		if expected == "" {
			response.Set(g, 202, task.Task{}, Invalid("invalid_task", errors.New("unsupported generation kind")))
			return
		}
		config, err := s.Providers.Repo.Get(ctx, providerCode)
		if err != nil {
			response.Set(g, 202, task.Task{}, err)
			return
		}
		if config.Capability != expected {
			response.Set(g, 202, task.Task{}, Invalid("invalid_task", errors.New("provider capability mismatch")))
			return
		}
		if !config.Enabled || config.Status != provider.Healthy {
			response.Set(g, 202, task.Task{}, Invalid("invalid_task", errors.New("provider is not ready")))
			return
		}

	}
	now := time.Now().UTC()
	t := task.New(now.Format("20060102150405.000000000"), kindValue, input, now)
	t.ProviderCode = providerCode
	t.SessionID = inputRequest.SessionID
	if err := s.Store.Save(ctx, t); err != nil {
		response.Set(g, 202, task.Task{}, err)
		return
	}
	if err := s.event(ctx, t); err != nil {
		response.Set(g, 202, t, s.rejectSubmission(ctx, t, err))
		return
	}
	if s.Enqueuer != nil {
		if err := s.Enqueuer.Enqueue(ctx, t); err != nil {
			response.Set(g, 202, t, s.rejectSubmission(ctx, t, err))
			return
		}
	}
	response.Set(g, 202, t, nil)
	return
}
func (s TaskService) Cancel(g *gin.Context) {
	ctx := g.Request.Context()
	id := g.Param("id")
	t, err := s.Store.Get(ctx, id)
	if err != nil {
		response.Set(g, 200, task.Task{}, err)
		return
	}
	if err := t.Cancel(time.Now().UTC()); err != nil {
		response.Set(g, 200, task.Task{}, Conflict("task_not_cancellable", err))
		return
	}
	if err := s.Store.Save(ctx, t); err != nil {
		response.Set(g, 200, t, err)
		return
	}
	if canceller, ok := s.Enqueuer.(interface {
		CancelTask(context.Context, string) error
	}); ok {
		if err := canceller.CancelTask(ctx, id); err != nil {
			response.Set(g, 200, t, err)
			return
		}
	}
	if err := s.event(ctx, t); err != nil {
		response.Set(g, 200, t, err)
		return
	}
	response.Set(g, 200, t, nil)
	return
}
func (s TaskService) Retry(g *gin.Context) {
	ctx := g.Request.Context()
	id := g.Param("id")
	t, err := s.Store.Get(ctx, id)
	if err != nil {
		response.Set(g, 202, task.Task{}, err)
		return
	}
	if sessionID := g.Query("session_id"); sessionID != "" {
		if len(sessionID) > 128 {
			response.Set(g, 0, nil, Invalid("invalid_session", errors.New("session_id exceeds 128 bytes")))
			return
		}
		t.SessionID = sessionID
	}
	if err := t.Retry(time.Now().UTC()); err != nil {
		response.Set(g, 202, task.Task{}, Conflict("task_not_retryable", err))
		return
	}
	if err := s.Store.Save(ctx, t); err != nil {
		response.Set(g, 202, task.Task{}, err)
		return
	}
	if err := s.event(ctx, t); err != nil {
		response.Set(g, 202, t, s.rejectSubmission(ctx, t, err))
		return
	}
	if s.Enqueuer != nil {
		if err := s.Enqueuer.Enqueue(ctx, t); err != nil {
			response.Set(g, 202, t, s.rejectSubmission(ctx, t, err))
			return
		}
	}
	response.Set(g, 202, t, nil)
	return
}
func (s TaskService) Get(g *gin.Context) {
	item, err := s.Store.Get(g.Request.Context(), g.Param("id"))
	response.Set(g, 200, item, err)
}
func (s TaskService) List(g *gin.Context) {
	items, err := s.Store.List(g.Request.Context())
	if err != nil {
		response.Set(g, 0, nil, err)
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
	response.Set(g, 200, gin.H{"tasks": items[offset:end], "total": len(items)}, nil)
}
func (s TaskService) Delete(g *gin.Context) {
	ctx := g.Request.Context()
	id := g.Param("id")
	_, err := s.Store.Get(ctx, id)
	if err == nil {
		err = s.Store.Delete(ctx, id)
	}
	response.Set(g, 204, nil, err)
}
func (s TaskService) Result(g *gin.Context) {
	id := g.Param("id")
	item, err := s.Store.Get(g.Request.Context(), id)
	if err == nil && item.Status != task.StatusSucceeded {
		err = Conflict("result_not_ready", errors.New("task result is not ready"))
	}
	if err == nil {
		g.Header("Content-Disposition", `attachment; filename="frameflow-result-`+id+`.json"`)
	}
	response.Set(g, 200, item, err)
}
func (s TaskService) EventLog(g *gin.Context) {
	events := s.Events
	if events == nil {
		events, _ = s.Store.(task.EventRepository)
	}
	if events == nil {
		response.Set(g, 0, nil, unavailable("events_unavailable", "event storage unavailable"))
		return
	}
	after, _ := strconv.ParseInt(g.Query("after"), 10, 64)
	items, err := events.ListEvents(g.Request.Context(), g.Param("id"), after)
	response.Set(g, 200, gin.H{"events": items}, err)
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
	return errors.Join(cause, s.event(cleanup, t))
}
func (s TaskService) event(ctx context.Context, t task.Task) error {
	events := s.Events
	if events == nil {
		events, _ = s.Store.(task.EventRepository)
	}
	return task.Publish(ctx, events, s.SendEvent, t.SessionID, task.Event{TaskID: t.ID, Status: t.Status, Progress: t.Progress, Stage: t.Stage, At: t.UpdatedAt})
}
