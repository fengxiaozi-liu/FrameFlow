package application

import (
	"context"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCancelledSubmissionDoesNotLeaveUnscheduledQueuedTask(t *testing.T) {
	worker := queue.NewWorker()
	for i := 0; i < 32; i++ {
		if err := worker.Enqueue(context.Background(), task.Task{}); err != nil {
			t.Fatal(err)
		}
	}
	store := queue.NewStore()
	service := TaskService{Store: store, Enqueuer: worker}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	router := gin.New()
	router.POST("/tasks", service.Create)
	out := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/tasks", strings.NewReader(`{"kind":"video","prompt":"test"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(out, req)
	if out.Code != 504 {
		t.Fatal(out.Code, out.Body.String())
	}
	items, err := store.List(context.Background())
	if err != nil || len(items) != 1 || items[0].Status != task.StatusCancelled {
		t.Fatal(items, err)
	}
}
