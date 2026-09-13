package task

import (
	"errors"
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type Task struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Status       Status    `json:"status"`
	Progress     int       `json:"progress"`
	Stage        string    `json:"stage"`
	Error        string    `json:"error,omitempty"`
	ProviderCode string    `json:"provider_code,omitempty"`
	ResultURL    string    `json:"result_url,omitempty"`
	RetryCount   int       `json:"retry_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func New(id, kind string, now time.Time) Task {
	return Task{ID: id, Kind: kind, Status: StatusQueued, Stage: "queued", CreatedAt: now, UpdatedAt: now}
}
func (t *Task) Start(now time.Time) {
	t.Status = StatusRunning
	t.Stage = "processing"
	t.UpdatedAt = now
}
func (t *Task) Advance(progress int, stage string, now time.Time) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	t.Progress = progress
	t.Stage = stage
	t.UpdatedAt = now
}
func (t *Task) Succeed(now time.Time) {
	t.Status = StatusSucceeded
	t.Progress = 100
	t.Stage = "completed"
	t.UpdatedAt = now
}
func (t *Task) Fail(message string, now time.Time) {
	t.Status = StatusFailed
	t.Error = message
	t.Stage = "failed"
	t.UpdatedAt = now
}
func (t *Task) Cancel(now time.Time) error {
	if t.Status == StatusSucceeded || t.Status == StatusFailed {
		return errors.New("completed task cannot be cancelled")
	}
	t.Status = StatusCancelled
	t.Stage = "cancelled"
	t.UpdatedAt = now
	return nil
}
func (t *Task) Retry(now time.Time) error {
	if t.Status != StatusFailed && t.Status != StatusCancelled {
		return errors.New("only failed or cancelled task can be retried")
	}
	t.Status = StatusQueued
	t.Stage = "queued"
	t.Progress = 0
	t.Error = ""
	t.RetryCount++
	t.UpdatedAt = now
	return nil
}
