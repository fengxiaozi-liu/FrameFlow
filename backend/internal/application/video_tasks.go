package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
)

type videoTaskWrite struct {
	ExpectedVersion *int64 `json:"expected_version"`
	IdempotencyKey  string `json:"idempotency_key"`
	ModelID         string `json:"model_id"`
	Resolution      string `json:"resolution"`
}

func (s TaskService) CreateSceneVideo(g *gin.Context) {
	var input videoTaskWrite
	if err := g.ShouldBindJSON(&input); err != nil {
		writeError(g, Invalid("invalid_video_task", err))
		return
	}
	item, issues, err := s.submitSceneVideo(g, g.Param("id"), g.Param("draftId"), g.Param("sceneId"), input)
	if err != nil {
		writeError(g, err)
		return
	}
	if len(issues) > 0 {
		g.JSON(http.StatusUnprocessableEntity, gin.H{"code": "video_input_invalid", "issues": issues})
		return
	}
	g.JSON(http.StatusAccepted, item)
}

func (s TaskService) CreateBatchVideo(g *gin.Context) {
	var input videoTaskWrite
	if err := g.ShouldBindJSON(&input); err != nil {
		writeError(g, Invalid("invalid_video_task", err))
		return
	}
	p, err := s.Projects.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	d, err := draftByID(&p, g.Param("draftId"))
	if err != nil {
		writeError(g, Invalid("invalid_video_task", err))
		return
	}
	if input.ExpectedVersion == nil || d.Version != *input.ExpectedVersion {
		writeError(g, Conflict("draft_version_conflict", project.ErrVersionConflict))
		return
	}
	created := []task.Task{}
	failed := []VideoValidationIssue{}
	for _, scene := range d.Story.Scenes {
		item, issues, err := s.submitSceneVideo(g, p.ID, d.ID, scene.ID, input)
		if err != nil {
			failed = append(failed, VideoValidationIssue{SceneID: scene.ID, Field: "submission", Message: err.Error()})
			continue
		}
		if len(issues) > 0 {
			failed = append(failed, issues...)
			continue
		}
		created = append(created, item)
	}
	status := http.StatusAccepted
	if len(created) == 0 && len(failed) > 0 {
		status = http.StatusUnprocessableEntity
	}
	g.JSON(status, gin.H{"created": created, "failed": failed})
}

func (s TaskService) submitSceneVideo(g *gin.Context, projectID, draftID, sceneID string, input videoTaskWrite) (task.Task, []VideoValidationIssue, error) {
	if input.ExpectedVersion == nil || *input.ExpectedVersion < 0 || input.IdempotencyKey == "" || input.ModelID == "" {
		return task.Task{}, nil, Invalid("invalid_video_task", errors.New("expected_version, idempotency_key and model_id are required"))
	}
	identity := sha256.Sum256([]byte(projectID + "\x00" + draftID + "\x00" + sceneID + "\x00" + input.IdempotencyKey))
	taskID := "task-" + hex.EncodeToString(identity[:16])
	if existing, err := s.Store.Get(g.Request.Context(), taskID); err == nil {
		if existing.ModelID != input.ModelID || existing.InputVersion != *input.ExpectedVersion {
			return task.Task{}, nil, Conflict("idempotency_key_reused", errors.New("idempotency key was used for different video inputs"))
		}
		return existing, nil, nil
	}
	p, err := s.Projects.Get(g.Request.Context(), projectID)
	if err != nil {
		return task.Task{}, nil, err
	}
	d, err := draftByID(&p, draftID)
	if err != nil {
		return task.Task{}, nil, Invalid("draft_not_found", err)
	}
	if d.Version != *input.ExpectedVersion {
		return task.Task{}, nil, Conflict("draft_version_conflict", project.ErrVersionConflict)
	}
	issues := s.validateVideoScene(g, projectID, draftID, sceneID, input.ModelID, input.Resolution)
	if len(issues) > 0 {
		return task.Task{}, issues, nil
	}
	var sceneIndex int = -1
	for i := range d.Story.Scenes {
		if d.Story.Scenes[i].ID == sceneID {
			sceneIndex = i
			break
		}
	}
	if sceneIndex < 0 {
		return task.Task{}, nil, Invalid("scene_not_found", errors.New("scene not found"))
	}
	scene := d.Story.Scenes[sceneIndex]
	taskInput := task.Input{Prompt: scene.VisualPrompt, Duration: scene.DurationSeconds, Resolution: input.Resolution}
	if taskInput.Resolution == "" {
		taskInput.Resolution = "1080P"
	}
	for _, binding := range d.Bindings {
		if binding.SceneID != sceneID {
			continue
		}
		asset, err := s.Materials.Get(g.Request.Context(), binding.MaterialID)
		if err != nil {
			return task.Task{}, nil, err
		}
		switch binding.Usage {
		case "first_frame":
			taskInput.SourceImageURL = asset.URL
		case "last_frame":
			taskInput.LastFrameURL = asset.URL
		case "driving_audio":
			taskInput.DrivingAudioURL = asset.URL
		}
	}
	now := time.Now().UTC()
	created := task.New(taskID, task.KindVideo, taskInput, now)
	created.ProjectID, created.DraftID, created.SceneID = projectID, draftID, sceneID
	created.ModelID, created.InputVersion, created.IdempotencyKey = input.ModelID, d.Version, input.IdempotencyKey
	created.InputHash = videoFingerprint(input.ModelID, taskInput)
	if err := s.Store.Save(g.Request.Context(), created); err != nil {
		return task.Task{}, nil, err
	}
	s.broadcast(g.Request.Context(), created)
	if s.Enqueuer != nil {
		if err := s.Enqueuer.Enqueue(g.Request.Context(), created); err != nil {
			return task.Task{}, nil, s.rejectSubmission(g.Request.Context(), created, err)
		}
	}
	return created, nil, nil
}

func videoFingerprint(modelID string, input task.Input) string {
	payload, _ := json.Marshal(struct {
		ModelID string     `json:"model_id"`
		Input   task.Input `json:"input"`
	}{modelID, input})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
