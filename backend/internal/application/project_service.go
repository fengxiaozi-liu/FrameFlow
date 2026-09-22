package application

import (
	"errors"
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
	if d.ProjectID != "" && d.ProjectID != ctx.Param("id") {
		writeError(ctx, Invalid("invalid_project", errors.New("draft project_id does not match route project")))
		return
	}
	projectID := ctx.Param("id")
	change := func(p *project.Project) error {
		for i := range p.Drafts {
			if p.Drafts[i].ID != d.ID {
				continue
			}
			d.ProjectID = p.ID
			d.UpdatedAt = time.Now().UTC()
			d.Outputs = mergeDraftOutputs(p.Drafts[i].Outputs, d.Outputs)
			p.Drafts[i] = d
			return nil
		}
		return p.AddDraft(d)
	}
	var p project.Project
	var err error
	if atomic, ok := s.Repo.(project.AtomicRepository); ok {
		p, err = atomic.Update(ctx.Request.Context(), projectID, change)
	} else {
		p, err = s.Repo.Get(ctx.Request.Context(), projectID)
		if err == nil {
			err = change(&p)
		}
		if err == nil {
			err = s.Repo.Save(ctx.Request.Context(), p)
		}
	}
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, p)
}

func mergeDraftOutputs(stored, submitted []project.DraftOutput) []project.DraftOutput {
	merged := append([]project.DraftOutput(nil), submitted...)
	known := make(map[string]bool, len(merged))
	for _, output := range merged {
		known[output.TaskID] = true
	}
	for _, output := range stored {
		if !known[output.TaskID] {
			merged = append(merged, output)
		}
	}
	return merged
}
