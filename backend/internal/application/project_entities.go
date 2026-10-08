package application

import (
	"errors"
	"fmt"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (s ProjectService) EntityCatalog(g *gin.Context) {
	p, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	catalog := p.EntityCatalog
	if catalog.Entities == nil {
		catalog.Entities = []project.EntityVersion{}
	}
	if catalog.Voices == nil {
		catalog.Voices = []project.VoiceProfile{}
	}
	g.JSON(200, catalog)
}

func (s ProjectService) EntityUsage(g *gin.Context) {
	p, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	uses := []string{}
	id := g.Param("entityId")
	for _, d := range p.Drafts {
		for _, scene := range d.Story.Scenes {
			used := false
			for _, b := range scene.EntityBindings {
				if b.EntityID == id {
					used = true
				}
			}
			for _, u := range scene.Utterances {
				if u.SpeakerID == id {
					used = true
				}
			}
			if used {
				uses = append(uses, d.ID+"/"+scene.ID)
			}
		}
	}
	g.JSON(200, gin.H{"entity_id": id, "uses": uses})
}
func (s ProjectService) WriteEntity(g *gin.Context) {
	var in struct {
		Expected *int64                `json:"expected_catalog_version"`
		Entity   project.EntityVersion `json:"entity"`
	}
	if err := g.ShouldBindJSON(&in); err != nil || in.Expected == nil {
		writeError(g, Invalid("invalid_entity", errors.New("expected_catalog_version and entity required")))
		return
	}
	if id := g.Param("entityId"); id != "" && id != in.Entity.ID {
		writeError(g, Invalid("invalid_entity", errors.New("entity identity mismatch")))
		return
	}
	if in.Entity.VoiceMaterialID != "" {
		if s.Materials == nil {
			writeError(g, errors.New("material repository unavailable"))
			return
		}
		asset, err := s.Materials.Get(g.Request.Context(), in.Entity.VoiceMaterialID)
		if err != nil || asset.Kind != material.Voice {
			writeError(g, Invalid("voice_material_missing", errors.New("请选择素材库中可用的配音素材")))
			return
		}
	}
	for _, r := range in.Entity.References {
		if s.Materials == nil {
			writeError(g, errors.New("material repository unavailable"))
			return
		}
		asset, err := s.Materials.Get(g.Request.Context(), r.MaterialID)
		if err != nil || (in.Entity.Kind == "character" && asset.Kind != material.Character) || (in.Entity.Kind == "location" && asset.Kind != material.Scene) {
			writeError(g, Invalid("reference_missing", fmt.Errorf("invalid reference %s", r.MaterialID)))
			return
		}
	}
	atomic, ok := s.Repo.(project.AtomicRepository)
	if !ok {
		writeError(g, errors.New("atomic repository required"))
		return
	}
	p, err := atomic.Update(g.Request.Context(), g.Param("id"), func(p *project.Project) error {
		if g.Request.Method == http.MethodPost && g.Param("entityId") == "" {
			for _, e := range p.Entities {
				if e.ID == in.Entity.ID {
					return errors.New("entity already exists")
				}
			}
		}
		if g.Param("entityId") != "" {
			exists := false
			for _, e := range p.Entities {
				if e.ID == in.Entity.ID {
					exists = true
				}
			}
			if !exists {
				return errors.New("entity not found")
			}
		}
		if g.FullPath() == "/api/projects/:id/entities/:entityId/archive" {
			in.Entity.Archived = true
		}
		return p.AddEntity(in.Entity, *in.Expected)
	})
	if errors.Is(err, project.ErrCatalogConflict) {
		g.JSON(409, gin.H{"code": "entity_revision_conflict", "message": "项目设定已更新，请重新加载后合并", "catalog": p.EntityCatalog})
		return
	}
	if err != nil {
		writeError(g, Invalid("invalid_entity", err))
		return
	}
	g.JSON(200, p.EntityCatalog)
}
