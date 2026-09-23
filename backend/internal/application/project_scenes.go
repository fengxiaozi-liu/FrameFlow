package application

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/gin-gonic/gin"
)

func (s ProjectService) WriteScene(ctx *gin.Context) {
	var input struct {
		ExpectedVersion *int64 `json:"expected_version"`
		Action          string `json:"action"`
		Title           string `json:"title"`
		VisualPrompt    string `json:"visual_prompt"`
		Narration       string `json:"narration"`
		DurationSeconds int    `json:"duration_seconds"`
		Order           int    `json:"order"`
	}
	if err := ctx.ShouldBindJSON(&input); err != nil || input.ExpectedVersion == nil || *input.ExpectedVersion < 0 {
		writeError(ctx, Invalid("invalid_scene", errors.New("expected_version is required")))
		return
	}
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(ctx, errors.New("atomic project repository is required"))
		return
	}
	var result project.Draft
	created := false
	_, err := atomic.Update(ctx.Request.Context(), ctx.Param("id"), func(p *project.Project) error {
		d, findErr := draftByID(p, ctx.Param("draftId"))
		if findErr != nil {
			return findErr
		}
		result = *d
		if d.Version != *input.ExpectedVersion {
			return project.ErrVersionConflict
		}
		sceneID := ctx.Param("sceneId")
		changes := story.Scene{Title: input.Title, VisualPrompt: input.VisualPrompt, Narration: input.Narration, DurationSeconds: input.DurationSeconds}
		switch input.Action {
		case "create":
			if sceneID != "" {
				return errors.New("create must not have scene id")
			}
			if err := d.Story.AddScene(changes); err != nil {
				return err
			}
			created = true
		case "update":
			if _, err := d.Story.UpdateScene(sceneID, changes); err != nil {
				return err
			}
		case "copy":
			if _, err := d.CopyScene(sceneID); err != nil {
				return err
			}
		case "move":
			if err := d.Story.MoveScene(sceneID, input.Order); err != nil {
				return err
			}
		default:
			return errors.New("action must be create, update, copy or move")
		}
		d.Version++
		d.UpdatedAt = time.Now().UTC()
		result = *d
		return nil
	})
	if errors.Is(err, project.ErrVersionConflict) {
		ctx.JSON(http.StatusConflict, gin.H{"code": "draft_version_conflict", "draft": result})
		return
	}
	if err != nil {
		writeError(ctx, Invalid("invalid_scene", err))
		return
	}
	if created {
		ctx.JSON(http.StatusCreated, result)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (s ProjectService) DeleteScene(ctx *gin.Context) {
	expected, err := strconv.ParseInt(ctx.Query("expected_version"), 10, 64)
	if err != nil || expected < 0 {
		writeError(ctx, Invalid("expected_version_required", errors.New("expected_version is required")))
		return
	}
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(ctx, errors.New("atomic project repository is required"))
		return
	}
	var result project.Draft
	_, err = atomic.Update(ctx.Request.Context(), ctx.Param("id"), func(p *project.Project) error {
		d, findErr := draftByID(p, ctx.Param("draftId"))
		if findErr != nil {
			return findErr
		}
		result = *d
		if d.Version != expected {
			return project.ErrVersionConflict
		}
		id := ctx.Param("sceneId")
		if !d.HasScene(id) {
			return errors.New("scene not found")
		}
		now := time.Now().UTC()
		if err := d.CaptureStoryboardSnapshot("delete-"+id+"-"+strconv.FormatInt(d.Version, 10), now); err != nil {
			return err
		}
		if _, err := d.Story.DeleteScene(id); err != nil {
			return err
		}
		bindings := d.Bindings[:0]
		for _, binding := range d.Bindings {
			if binding.SceneID != id {
				bindings = append(bindings, binding)
			}
		}
		d.Bindings = bindings
		versions := d.VideoVersions[:0]
		for _, version := range d.VideoVersions {
			if version.SceneID != id {
				versions = append(versions, version)
			}
		}
		d.VideoVersions = versions
		delete(d.SelectedVersions, id)
		d.Version++
		d.UpdatedAt = now
		result = *d
		return nil
	})
	if errors.Is(err, project.ErrVersionConflict) {
		ctx.JSON(http.StatusConflict, gin.H{"code": "draft_version_conflict", "draft": result})
		return
	}
	if err != nil {
		writeError(ctx, Invalid("invalid_scene", err))
		return
	}
	ctx.JSON(http.StatusOK, result)
}
