package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/project"
	"time"
)

type ProjectService struct{ Repo project.Repository }

func (s ProjectService) Create(id, name string) (project.Project, error) {
	p, e := project.New(id, name, time.Now().UTC())
	if e != nil {
		return p, e
	}
	return p, s.Repo.Save(p)
}

func (s ProjectService) List() []project.Project {
	return s.Repo.List()
}

func (s ProjectService) Get(id string) (project.Project, bool) {
	return s.Repo.Get(id)
}

func (s ProjectService) SaveDraft(projectID string, d project.Draft) (project.Project, error) {
	p, ok := s.Repo.Get(projectID)
	if !ok {
		return p, errors.New("project not found")
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
			return p, e
		}
	}
	return p, s.Repo.Save(p)
}
