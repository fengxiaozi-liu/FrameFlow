package task

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
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
	KindStory       Kind = "story"
	KindImage       Kind = "image"
	KindVideo       Kind = "video"
	KindComposition Kind = "composition"
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
	case KindStory, KindImage, KindVideo, KindComposition:
		return Kind(value), nil
	default:
		return "", errors.New("unsupported task kind")
	}
}

type Input struct {
	Prompt          string `json:"prompt"`
	SourceBody      string `json:"source_body,omitempty"`
	Mode            string `json:"mode,omitempty"`
	AspectRatio     string `json:"aspect_ratio,omitempty"`
	SourceImageURL  string `json:"source_image_url,omitempty"`
	LastFrameURL    string `json:"last_frame_url,omitempty"`
	DrivingAudioURL string `json:"driving_audio_url,omitempty"`
	Duration        int    `json:"duration,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
}

type CompositionClip struct {
	SceneID         string  `json:"scene_id"`
	VersionID       string  `json:"version_id"`
	MediaPath       string  `json:"media_path"`
	VoicePath       string  `json:"voice_path,omitempty"`
	DurationSeconds float64 `json:"duration_seconds"`
}

type CompositionInput struct {
	Clips           []CompositionClip `json:"clips"`
	MusicMaterialID string            `json:"music_material_id,omitempty"`
	MusicPath       string            `json:"music_path,omitempty"`
	MusicVolume     float64           `json:"music_volume"`
	SourceVolume    float64           `json:"source_volume"`
	VoiceVolume     float64           `json:"voice_volume"`
	AspectRatio     string            `json:"aspect_ratio"`
	Resolution      string            `json:"resolution"`
}

type MediaProbeResult struct {
	Duration float64
	Width    int
	Height   int
	HasAudio bool
}

func (i Input) Validate() error {
	if strings.TrimSpace(i.Prompt) == "" {
		return errors.New("task prompt is required")
	}
	return nil
}

type Task struct {
	ID                    string            `json:"id"`
	ProjectID             string            `json:"project_id,omitempty"`
	DraftID               string            `json:"draft_id,omitempty"`
	SceneID               string            `json:"scene_id,omitempty"`
	Target                string            `json:"target,omitempty"`
	InputVersion          int64             `json:"input_version,omitempty"`
	InputHash             string            `json:"input_hash,omitempty"`
	IdempotencyKey        string            `json:"idempotency_key,omitempty"`
	CapabilityVersion     string            `json:"capability_version,omitempty"`
	Kind                  Kind              `json:"kind"`
	Status                Status            `json:"status"`
	Progress              int               `json:"progress"`
	Stage                 Stage             `json:"stage"`
	Error                 string            `json:"error,omitempty"`
	ProviderCode          string            `json:"provider_code,omitempty"`
	ModelID               string            `json:"model_id,omitempty"`
	RemoteTaskID          string            `json:"remote_task_id,omitempty"`
	ResultText            string            `json:"result_text,omitempty"`
	ResultCandidate       *story.Candidate  `json:"result_candidate,omitempty"`
	Composition           *CompositionInput `json:"composition,omitempty"`
	Input                 Input             `json:"input"`
	ResultURL             string            `json:"result_url,omitempty"`
	ResultDurationSeconds float64           `json:"result_duration_seconds,omitempty"`
	RetryCount            int               `json:"retry_count"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}

func (t Task) ValidateScope() error {
	if strings.TrimSpace(t.ProjectID) == "" || strings.TrimSpace(t.DraftID) == "" {
		return errors.New("task project_id and draft_id are required")
	}
	return nil
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
