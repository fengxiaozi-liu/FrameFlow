package application

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
)

func (s TaskService) CreateCandidate(g *gin.Context) {
	var input struct {
		ExpectedVersion *int64 `json:"expected_version"`
		IdempotencyKey  string `json:"idempotency_key"`
		Target          string `json:"target"`
		Mode            string `json:"mode"`
		Instruction     string `json:"instruction"`
		ModelID         string `json:"model_id"`
	}
	if err := g.ShouldBindJSON(&input); err != nil {
		writeError(g, Invalid("invalid_candidate", err))
		return
	}
	if input.ExpectedVersion == nil || *input.ExpectedVersion < 0 || strings.TrimSpace(input.IdempotencyKey) == "" {
		writeError(g, Invalid("invalid_candidate", errors.New("expected_version and idempotency_key are required")))
		return
	}
	target := story.CandidateTarget(input.Target)
	if target != story.CandidateStory && target != story.CandidateStoryboard {
		writeError(g, Invalid("invalid_candidate", errors.New("target must be story or storyboard")))
		return
	}
	projectID, draftID := g.Param("id"), g.Param("draftId")
	p, err := s.Projects.Get(g.Request.Context(), projectID)
	if err != nil {
		writeError(g, err)
		return
	}
	var sourceBody string
	found := false
	for _, draft := range p.Drafts {
		if draft.ID == draftID {
			found = true
			if draft.Version != *input.ExpectedVersion {
				writeError(g, Conflict("draft_version_conflict", errors.New("save draft changes before generation")))
				return
			}
			if target == story.CandidateStoryboard || input.Mode == "revise" {
				sourceBody = draft.Story.Body
			}
			break
		}
	}
	if !found {
		writeError(g, Invalid("draft_not_found", errors.New("draft does not belong to project")))
		return
	}
	if _, _, _, err = story.BuildGenerationPrompt(target, input.Instruction, sourceBody); err != nil {
		writeError(g, Invalid("invalid_candidate", err))
		return
	}
	if scoped, ok := s.Store.(task.ScopedRepository); ok {
		existing, listErr := scoped.ListByScope(g.Request.Context(), task.Scope{ProjectID: projectID, DraftID: draftID})
		if listErr != nil {
			writeError(g, listErr)
			return
		}
		for _, previous := range existing {
			if previous.IdempotencyKey == input.IdempotencyKey && previous.Target == input.Target {
				g.JSON(http.StatusAccepted, previous)
				return
			}
		}
	}
	if s.Providers.Catalog.Models == nil {
		writeError(g, Invalid("model_unavailable", errors.New("story model catalog is unavailable")))
		return
	}
	modelID := input.ModelID
	if modelID == "" {
		models, listErr := s.Providers.Catalog.Models.ListModels(g.Request.Context(), "")
		if listErr != nil {
			writeError(g, listErr)
			return
		}
		for _, model := range models {
			if model.Default && model.Supports(provider.Story) {
				modelID = model.ID
				break
			}
		}
	}
	model, err := s.Providers.Catalog.Models.GetModel(g.Request.Context(), modelID)
	if err != nil || !model.Supports(provider.Story) {
		writeError(g, Invalid("model_unavailable", errors.New("select an available story model")))
		return
	}
	prompt := strings.TrimSpace(input.Instruction)
	if prompt == "" {
		prompt = "按正文生成分镜"
	}
	now := time.Now().UTC()
	id, err := story.NewSceneID()
	if err != nil {
		writeError(g, err)
		return
	}
	created := task.New(strings.Replace(id, "scene-", "task-", 1), task.KindStory, task.Input{Prompt: prompt, SourceBody: sourceBody, Mode: input.Mode}, now)
	created.ProjectID, created.DraftID, created.Target = projectID, draftID, input.Target
	created.InputVersion, created.IdempotencyKey, created.ModelID = *input.ExpectedVersion, input.IdempotencyKey, modelID
	if err = s.Store.Save(g.Request.Context(), created); err != nil {
		writeError(g, err)
		return
	}
	if s.Enqueuer != nil {
		if err = s.Enqueuer.Enqueue(g.Request.Context(), created); err != nil {
			writeError(g, s.rejectSubmission(g.Request.Context(), created, err))
			return
		}
	}
	g.JSON(http.StatusAccepted, created)
}
