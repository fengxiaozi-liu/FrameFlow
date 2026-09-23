package application

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/gin-gonic/gin"
)

func (s ProjectService) WriteBindings(g *gin.Context) {
	var input struct {
		ExpectedVersion *int64                 `json:"expected_version"`
		Bindings        []project.SceneBinding `json:"bindings"`
	}
	if err := g.ShouldBindJSON(&input); err != nil || input.ExpectedVersion == nil || *input.ExpectedVersion < 0 {
		writeError(g, Invalid("invalid_bindings", errors.New("expected_version and bindings are required")))
		return
	}
	if s.Materials == nil {
		writeError(g, errors.New("material repository is unavailable"))
		return
	}
	sceneID := g.Param("sceneId")
	seen := make(map[string]bool)
	one := make(map[string]bool)
	for i := range input.Bindings {
		binding := &input.Bindings[i]
		binding.SceneID = sceneID
		asset, err := s.Materials.Get(g.Request.Context(), binding.MaterialID)
		if err != nil {
			writeError(g, Invalid("invalid_bindings", fmt.Errorf("material %s: %s", binding.MaterialID, err)))
			return
		}
		if err := validateBinding(*binding, asset); err != nil {
			writeError(g, Invalid("invalid_bindings", fmt.Errorf("binding %d: %w", i+1, err)))
			return
		}
		key := binding.Usage + "/" + binding.MaterialID
		if seen[key] {
			writeError(g, Invalid("invalid_bindings", errors.New("duplicate material usage")))
			return
		}
		seen[key] = true
		if binding.Usage == "first_frame" || binding.Usage == "last_frame" || binding.Usage == "driving_audio" || binding.Usage == "voiceover" {
			if one[binding.Usage] {
				writeError(g, Invalid("invalid_bindings", fmt.Errorf("only one %s is allowed", binding.Usage)))
				return
			}
			one[binding.Usage] = true
		}
	}
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(g, errors.New("atomic project repository is required"))
		return
	}
	var result project.Draft
	_, err := atomic.Update(g.Request.Context(), g.Param("id"), func(p *project.Project) error {
		d, err := draftByID(p, g.Param("draftId"))
		if err != nil {
			return err
		}
		result = *d
		if d.Version != *input.ExpectedVersion {
			return project.ErrVersionConflict
		}
		if !d.HasScene(sceneID) {
			return errors.New("scene not found")
		}
		retained := make([]project.SceneBinding, 0, len(d.Bindings)+len(input.Bindings))
		for _, binding := range d.Bindings {
			if binding.SceneID != sceneID {
				retained = append(retained, binding)
			}
		}
		d.Bindings = append(retained, input.Bindings...)
		d.Version++
		d.UpdatedAt = time.Now().UTC()
		result = *d
		return nil
	})
	if errors.Is(err, project.ErrVersionConflict) {
		g.JSON(http.StatusConflict, gin.H{"code": "draft_version_conflict", "draft": result})
		return
	}
	if err != nil {
		writeError(g, Invalid("invalid_bindings", err))
		return
	}
	g.JSON(http.StatusOK, result)
}

func validateBinding(binding project.SceneBinding, asset material.Asset) error {
	switch binding.Usage {
	case "character_reference":
		if asset.Kind != material.Character {
			return errors.New("character image required")
		}
	case "scene_reference", "first_frame", "last_frame":
		if asset.Kind != material.Scene {
			return errors.New("scene image required")
		}
	case "prop_reference":
		if asset.Kind != material.Prop {
			return errors.New("prop image required")
		}
	case "driving_audio", "voiceover":
		if asset.Kind != material.Voice {
			return errors.New("voice audio required")
		}
	default:
		return errors.New("unsupported usage")
	}
	return nil
}
