package task

import (
	"errors"
	"strings"
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

type Kind string

const (
	KindStory Kind = "story"
	KindImage Kind = "image"
	KindVideo Kind = "video"
)

type Stage string

const (
	StageQueued          Stage = "queued"
	StageProcessing      Stage = "processing"
	StagePreparing       Stage = "preparing"
	StageGeneratingStory Stage = "generating_story"
	StageGeneratingImage Stage = "generating_image"
	StageRenderingVideo  Stage = "rendering_video"
	StageComposing       Stage = "composing"
	StageCompleted       Stage = "completed"
	StageFailed          Stage = "failed"
	StageCancelled       Stage = "cancelled"
)

func ParseKind(value string) (Kind, error) {
	switch Kind(value) {
	case KindStory, KindImage, KindVideo:
		return Kind(value), nil
	default:
		return "", errors.New("unsupported task kind")
	}
}

type Input struct {
	Prompt         string `json:"prompt"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	SourceImageURL string `json:"source_image_url,omitempty"`
}

func (i Input) Validate() error {
	if strings.TrimSpace(i.Prompt) == "" {
		return errors.New("task prompt is required")
	}
	return nil
}

type Task struct {
	ID           string    `json:"id"`
	Kind         Kind      `json:"kind"`
	Status       Status    `json:"status"`
	Progress     int       `json:"progress"`
	Stage        Stage     `json:"stage"`
	Error        string    `json:"error,omitempty"`
	ProviderCode string    `json:"provider_code,omitempty"`
	ModelID      string    `json:"model_id,omitempty"`
	RemoteTaskID string    `json:"remote_task_id,omitempty"`
	ResultText   string    `json:"result_text,omitempty"`
	Input        Input     `json:"input"`
	ResultURL    string    `json:"result_url,omitempty"`
	RetryCount   int       `json:"retry_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func New(id string, kind Kind, input Input, now time.Time) Task {
	return Task{
		ID:        id,
		Kind:      kind,
		Input:     input,
		Status:    StatusQueued,
		Stage:     StageQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (t *Task) Start(now time.Time) {
	t.Status = StatusRunning
	t.Stage = StageProcessing
	t.UpdatedAt = now
}

func (t *Task) Advance(progress int, stage Stage, now time.Time) {
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
	t.Stage = StageCompleted
	t.UpdatedAt = now
}

func (t *Task) Fail(message string, now time.Time) {
	t.Status = StatusFailed
	t.Error = message
	t.Stage = StageFailed
	t.UpdatedAt = now
}

func (t *Task) Cancel(now time.Time) error {
	if t.Status == StatusSucceeded || t.Status == StatusFailed {
		return errors.New("completed task cannot be cancelled")
	}
	t.Status = StatusCancelled
	t.Stage = StageCancelled
	t.UpdatedAt = now
	return nil
}

func (t *Task) Retry(now time.Time) error {
	if t.Status != StatusFailed && t.Status != StatusCancelled {
		return errors.New("only failed or cancelled task can be retried")
	}
	t.Status = StatusQueued
	t.Stage = StageQueued
	t.Progress = 0
	t.Error = ""
	t.RetryCount++
	t.UpdatedAt = now
	return nil
}
