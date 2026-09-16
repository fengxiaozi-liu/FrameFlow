package task

import (
	"context"
	"log"
	"time"
)

type Repository interface {
	Save(context.Context, Task) error
	Get(context.Context, string) (Task, error)
	List(context.Context) ([]Task, error)
	Delete(context.Context, string) error
}

type Event struct {
	Sequence int64     `json:"sequence"`
	TaskID   string    `json:"task_id"`
	Status   Status    `json:"status"`
	Progress int       `json:"progress"`
	Stage    string    `json:"stage"`
	At       time.Time `json:"at"`
}
type EventRepository interface {
	AppendEvent(context.Context, *Event) error
	ListEvents(context.Context, string, int64) ([]Event, error)
}

// Sender delivers a persisted event to a session's current connection.
type Sender func(context.Context, string, Event) error

// Publish persists independently of best-effort live delivery.
func Publish(ctx context.Context, repo EventRepository, send Sender, sessionID string, event Event) error {
	if repo != nil {
		if err := repo.AppendEvent(ctx, &event); err != nil {
			return err
		}
	}
	if send != nil && sessionID != "" {
		if err := send(ctx, sessionID, event); err != nil {
			log.Printf("task %s notification: %v", event.TaskID, err)
		}
	}
	return nil
}
