package application

import (
	"errors"
	"net/http"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
)

func (s ProjectService) ListSceneVersions(g *gin.Context) {
	p, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	d, err := draftByID(&p, g.Param("draftId"))
	if err != nil {
		writeError(g, err)
		return
	}
	sceneID := g.Param("sceneId")
	if !d.HasScene(sceneID) {
		writeError(g, Invalid("scene_not_found", errors.New("scene not found")))
		return
	}
	type versionView struct {
		project.VideoVersion
		BasedOnOldSettings bool `json:"based_on_old_settings"`
	}
	versions := []versionView{}
	for _, version := range d.VideoVersions {
		if version.SceneID != sceneID {
			continue
		}
		view := versionView{VideoVersion: version}
		if s.Tasks != nil && s.Materials != nil {
			original, err := s.Tasks.Get(g.Request.Context(), version.TaskID)
			if err == nil {
				current := task.Input{Prompt: original.Input.Prompt, Duration: original.Input.Duration, Resolution: original.Input.Resolution, AspectRatio: original.Input.AspectRatio}
				for _, scene := range d.Story.Scenes {
					if scene.ID == sceneID {
						current.Prompt = scene.VisualPrompt
						current.Duration = scene.DurationSeconds
						break
					}
				}
				for _, binding := range d.Bindings {
					if binding.SceneID != sceneID {
						continue
					}
					asset, err := s.Materials.Get(g.Request.Context(), binding.MaterialID)
					if err != nil {
						view.BasedOnOldSettings = true
						continue
					}
					switch binding.Usage {
					case "first_frame":
						current.SourceImageURL = asset.URL
					case "last_frame":
						current.LastFrameURL = asset.URL
					case "driving_audio":
						current.DrivingAudioURL = asset.URL
					}
				}
				if videoFingerprint(original.ModelID, current) != version.InputFingerprint {
					view.BasedOnOldSettings = true
				}
			}
		}
		versions = append(versions, view)
	}
	g.JSON(http.StatusOK, gin.H{"versions": versions, "selected_version_id": d.SelectedVersions[sceneID]})
}

func (s ProjectService) SelectSceneVersion(g *gin.Context) {
	var input struct {
		ExpectedVersion *int64 `json:"expected_version"`
		VersionID       string `json:"version_id"`
	}
	if err := g.ShouldBindJSON(&input); err != nil || input.ExpectedVersion == nil || *input.ExpectedVersion < 0 {
		writeError(g, Invalid("invalid_video_version", errors.New("expected_version is required")))
		return
	}
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(g, errors.New("atomic repository required"))
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
		if !d.HasScene(g.Param("sceneId")) {
			return errors.New("scene not found")
		}
		if input.VersionID == "" {
			delete(d.SelectedVersions, g.Param("sceneId"))
		} else if err := d.SelectVideoVersion(g.Param("sceneId"), input.VersionID); err != nil {
			return err
		}
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
		writeError(g, Invalid("invalid_video_version", err))
		return
	}
	g.JSON(http.StatusOK, result)
}
