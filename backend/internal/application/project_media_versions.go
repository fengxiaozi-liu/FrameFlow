package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/gin-gonic/gin"
)

func (s ProjectService) ListMediaTakes(g *gin.Context) {
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
	out := []project.MediaTake{}
	for _, v := range d.MediaTakes {
		if v.SceneID == g.Param("sceneId") {
			out = append(out, v)
		}
	}
	g.JSON(200, gin.H{"takes": out, "selected": d.SelectedMedia})
}
func (s ProjectService) SelectMediaTake(g *gin.Context) {
	var in struct {
		Expected    *int64 `json:"expected_version"`
		TakeID      string `json:"take_id"`
		UtteranceID string `json:"utterance_id"`
	}
	if err := g.ShouldBindJSON(&in); err != nil || in.Expected == nil {
		writeError(g, Invalid("invalid_selection", errors.New("expected_version required")))
		return
	}
	var result project.Draft
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(g, errors.New("atomic repository required"))
		return
	}
	_, err := atomic.Update(g.Request.Context(), g.Param("id"), func(p *project.Project) error {
		d, err := draftByID(p, g.Param("draftId"))
		if err != nil {
			return err
		}
		result = *d
		if d.Version != *in.Expected {
			return project.ErrVersionConflict
		}
		sceneID := g.Param("sceneId")
		if !d.HasScene(sceneID) {
			return errors.New("scene not found")
		}
		var take *project.MediaTake
		for i := range d.MediaTakes {
			if d.MediaTakes[i].ID == in.TakeID && d.MediaTakes[i].SceneID == sceneID {
				take = &d.MediaTakes[i]
			}
		}
		if take == nil || take.Stale {
			return errors.New("take missing or stale; regenerate before selecting")
		}
		slot := "image"
		if take.Kind == "speech" {
			slot = "speech:" + take.UtteranceID
		}
		if in.UtteranceID != take.UtteranceID {
			return errors.New("selection slot mismatch")
		}
		if d.SelectedMedia == nil {
			d.SelectedMedia = map[string]string{}
		}
		d.SelectedMedia[sceneID+":"+slot] = take.ID
		if take.Kind == "image" {
			next := []project.SceneBinding{}
			for _, b := range d.Bindings {
				if b.SceneID != sceneID || b.Usage != "first_frame" {
					next = append(next, b)
				}
			}
			d.Bindings = append(next, project.SceneBinding{SceneID: sceneID, MaterialID: take.MaterialID, Usage: "first_frame"})
		}
		d.Version++
		result = *d
		return nil
	})
	if errors.Is(err, project.ErrVersionConflict) {
		g.JSON(409, gin.H{"code": "draft_version_conflict", "draft": result})
		return
	}
	if err != nil {
		writeError(g, Invalid("invalid_selection", err))
		return
	}
	g.JSON(200, result)
}
