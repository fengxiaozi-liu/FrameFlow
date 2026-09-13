package task

import "time"

type Repository interface {
	Save(Task) error
	Get(string) (Task, bool)
	List() []Task
	Delete(string) error
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
	AppendEvent(Event) error
	ListEvents(string, int64) []Event
}
