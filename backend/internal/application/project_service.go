package application

import (
	"errors"
	"net/http"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
)

type ProjectService struct {
	Repo      project.Repository
	Materials material.Repository
	Tasks     task.Repository
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

func (s ProjectService) SaveDraftRevision(ctx *gin.Context) {
	var input struct {
		ExpectedVersion *int64  `json:"expected_version"`
		Body            string  `json:"body"`
		Name            *string `json:"name"`
	}
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeError(ctx, Invalid("invalid_draft_revision", err))
		return
	}
	if input.ExpectedVersion == nil || *input.ExpectedVersion < 0 {
		writeError(ctx, Invalid("expected_version_required", errors.New("expected_version must be non-negative")))
		return
	}
	projectID, draftID := ctx.Param("id"), ctx.Param("draftId")
	var saved project.Draft
	change := func(p *project.Project) error {
		var err error
		saved, err = p.UpdateDraftBody(draftID, *input.ExpectedVersion, input.Body, time.Now().UTC())
		if err == nil && input.Name != nil {
			for i := range p.Drafts {
				if p.Drafts[i].ID == draftID {
					p.Drafts[i].Name = *input.Name
					saved.Name = *input.Name
					break
				}
			}
		}
		return err
	}
	var err error
	if atomic, ok := s.Repo.(project.AtomicRepository); ok {
		_, err = atomic.Update(ctx.Request.Context(), projectID, change)
	} else {
		err = errors.New("atomic project repository is required for versioned saves")
	}
	if errors.Is(err, project.ErrVersionConflict) {
		ctx.JSON(http.StatusConflict, gin.H{"code": "draft_version_conflict", "draft": saved})
		return
	}
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, saved)
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
			// Legacy draft saves do not know about new server-owned history.
			d.Candidates = p.Drafts[i].Candidates
			d.StoryboardSnapshots = p.Drafts[i].StoryboardSnapshots
			d.BodySnapshots = p.Drafts[i].BodySnapshots
			d.Bindings = p.Drafts[i].Bindings
			d.VideoVersions = p.Drafts[i].VideoVersions
			d.SelectedVersions = p.Drafts[i].SelectedVersions
			d.Compositions = p.Drafts[i].Compositions
			d.Version = p.Drafts[i].Version + 1
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
