package application

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/request"
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/response"
	"github.com/gin-gonic/gin"
	"time"
)

type ProjectService struct{ Repo project.Repository }

func (s ProjectService) Create(g *gin.Context) {
	var input struct {
		Name string `json:"name"`
	}
	if !request.BindJSON(g, &input) {
		return
	}
	ctx := g.Request.Context()
	id := time.Now().UTC().Format("20060102150405.000000000")
	name := input.Name
	p, e := project.New(id, name, time.Now().UTC())
	if e != nil {
		response.Set(g, 201, p, Invalid("invalid_project", e))
		return
	}
	response.Set(g, 201, p, s.Repo.Save(ctx, p))
	return
}
func (s ProjectService) List(g *gin.Context) {
	items, err := s.Repo.List(g.Request.Context())
	response.Set(g, 200, gin.H{"projects": items}, err)
}
func (s ProjectService) Get(g *gin.Context) {
	item, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	response.Set(g, 200, item, err)
}
func (s ProjectService) SaveDraft(g *gin.Context) {
	ctx := g.Request.Context()
	projectID := g.Param("id")
	var d project.Draft
	if !request.BindJSON(g, &d) {
		return
	}
	if d.ID == "" {
		d.ID = time.Now().UTC().Format("20060102150405.000000000")
	}
	p, err := s.Repo.Get(ctx, projectID)
	if err != nil {
		response.Set(g, 200, p, err)
		return
	}
	found := false
	for i := range p.Drafts {
		if p.Drafts[i].ID == d.ID {
			d.ProjectID = p.ID
			d.UpdatedAt = time.Now().UTC()
			p.Drafts[i] = d
			found = true
			break
		}
	}
	if !found {
		if e := p.AddDraft(d); e != nil {
			response.Set(g, 200, p, Invalid("invalid_project", e))
			return
		}
	}
	response.Set(g, 200, p, s.Repo.Save(ctx, p))
	return
}
