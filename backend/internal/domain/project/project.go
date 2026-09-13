package project

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/story"
	"strings"
	"time"
)

type Draft struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	Name      string         `json:"name"`
	Story     story.Document `json:"story"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Drafts    []Draft   `json:"drafts"`
	CreatedAt time.Time `json:"created_at"`
}

func New(id, name string, now time.Time) (Project, error) {
	if id == "" || strings.TrimSpace(name) == "" {
		return Project{}, errors.New("project name is required")
	}
	return Project{ID: id, Name: name, CreatedAt: now}, nil
}
func (p *Project) AddDraft(d Draft) error {
	if d.ID == "" {
		return errors.New("draft id is required")
	}
	d.ProjectID = p.ID
	d.UpdatedAt = p.CreatedAt
	p.Drafts = append(p.Drafts, d)
	return nil
}
