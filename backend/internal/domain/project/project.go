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
	Outputs   []DraftOutput  `json:"outputs,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type DraftOutput struct {
	TaskID    string    `json:"task_id"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text,omitempty"`
	URL       string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
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

func (p *Project) HasDraft(id string) bool {
	for i := range p.Drafts {
		if p.Drafts[i].ID == id {
			return true
		}
	}
	return false
}

// ApplyTaskResult updates exactly one draft and is idempotent by task ID.
func (p *Project) ApplyTaskResult(draftID string, output DraftOutput, now time.Time) error {
	if output.TaskID == "" || output.Kind == "" {
		return errors.New("task result identity is required")
	}
	for i := range p.Drafts {
		draft := &p.Drafts[i]
		if draft.ID != draftID {
			continue
		}
		for _, existing := range draft.Outputs {
			if existing.TaskID == output.TaskID {
				return nil
			}
		}
		if output.Kind == "story" {
			if err := draft.Story.UpdateBody(output.Text, now); err != nil {
				return err
			}
		} else if strings.TrimSpace(output.URL) == "" {
			return errors.New("media task result URL is required")
		}
		output.CreatedAt = now
		draft.Outputs = append(draft.Outputs, output)
		draft.UpdatedAt = now
		return nil
	}
	return errors.New("draft not found in project")
}
