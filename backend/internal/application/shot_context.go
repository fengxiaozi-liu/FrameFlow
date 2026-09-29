package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"github.com/gin-gonic/gin"
	"time"
)

func (s ProjectService) WriteShotContext(g *gin.Context) {
	var in struct {
		Expected   *int64                `json:"expected_version"`
		Bindings   []story.EntityBinding `json:"entity_bindings"`
		Prompt     story.PromptDocument  `json:"prompt_document"`
		Utterances []story.Utterance     `json:"utterances"`
	}
	if err := g.ShouldBindJSON(&in); err != nil || in.Expected == nil {
		writeError(g, Invalid("invalid_context", errors.New("expected_version required")))
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
		if d.Version != *in.Expected {
			return project.ErrVersionConflict
		}
		for i := range d.Story.Scenes {
			scene := &d.Story.Scenes[i]
			if scene.ID != g.Param("sceneId") {
				continue
			}
			next := *scene
			next.EntityBindings = in.Bindings
			next.PromptDocument = &in.Prompt
			next.Utterances = in.Utterances
			r, err := ResolveBindings(p.EntityCatalog, next, false)
			if err != nil {
				return err
			}
			next.VisualPrompt = r.Prompt
			*scene = next
			d.Version++
			d.UpdatedAt = time.Now().UTC()
			for i := range d.MediaTakes {
				if d.MediaTakes[i].SceneID == scene.ID {
					path := "reference_image_then_video"
					if d.MediaTakes[i].Kind == "speech" {
						path = "speech"
					}
					d.MediaTakes[i].Stale = d.MediaTakes[i].DependencyHash != dependencyHash(p.EntityCatalog, next, path, d.MediaTakes[i].UtteranceID)
				}
			}
			result = *d
			return nil
		}
		return errors.New("scene not found")
	})
	if errors.Is(err, project.ErrVersionConflict) {
		g.JSON(409, gin.H{"code": "draft_version_conflict", "message": "草稿已改变，本地内容仍保留", "draft": result})
		return
	}
	if err != nil {
		writeError(g, Invalid("invalid_context", err))
		return
	}
	g.JSON(200, result)
}
