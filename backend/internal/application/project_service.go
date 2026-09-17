package application

import (
	"net/http"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/gin-gonic/gin"
)

type ProjectService struct {
	Repo project.Repository
}

func (s ProjectService) Create(ctx *gin.Context) {
	var input struct {
		Name string `json:"name"`
	}
	if err := ctx.Bind(&input); err != nil {
		return
	}
	now := time.Now().UTC()
	p, err := project.New(now.Format("20060102150405.000000000"), input.Name, now)
	if err != nil {
		writeError(ctx, Invalid("invalid_project", err))
		return
	}
	if err := s.Repo.Save(ctx.Request.Context(), p); err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, p)
}

func (s ProjectService) List(ctx *gin.Context) {
	items, err := s.Repo.List(ctx.Request.Context())
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"projects": items,
	})
}

func (s ProjectService) Get(ctx *gin.Context) {
	item, err := s.Repo.Get(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, item)
}

func (s ProjectService) SaveDraft(ctx *gin.Context) {
	var d project.Draft
	if err := ctx.Bind(&d); err != nil {
		return
	}
	if d.ID == "" {
		d.ID = time.Now().UTC().Format("20060102150405.000000000")
	}
	p, err := s.Repo.Get(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		writeError(ctx, err)
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
		if err := p.AddDraft(d); err != nil {
			writeError(ctx, Invalid("invalid_project", err))
			return
		}
	}
	if err := s.Repo.Save(ctx.Request.Context(), p); err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}
