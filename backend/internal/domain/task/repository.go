package task

import (
	"context"
	"time"
)

type Repository interface {
	Save(context.Context, Task) error
	Get(context.Context, string) (Task, error)
	List(context.Context) ([]Task, error)
	Delete(context.Context, string) error
}

type Scope struct {
	ProjectID string
	DraftID   string
}

type ScopedRepository interface {
	Repository
	ListByScope(context.Context, Scope) ([]Task, error)
}

type Event struct {
	TaskID    string    `json:"task_id"`
	ProjectID string    `json:"project_id,omitempty"`
	DraftID   string    `json:"draft_id,omitempty"`
	Status    Status    `json:"status"`
	Progress  int       `json:"progress"`
	Stage     Stage     `json:"stage"`
	At        time.Time `json:"at"`
}
