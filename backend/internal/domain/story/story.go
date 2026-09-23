package story

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type CandidateTarget string

const (
	CandidateStory      CandidateTarget = "story"
	CandidateStoryboard CandidateTarget = "storyboard"
)

// Candidate remains separate from the editable document until explicitly applied.
type Candidate struct {
	ID            string          `json:"id"`
	TaskID        string          `json:"task_id"`
	Target        CandidateTarget `json:"target"`
	Mode          string          `json:"mode,omitempty"`
	Instruction   string          `json:"instruction,omitempty"`
	SourceVersion int64           `json:"source_version"`
	SourceBody    string          `json:"source_body,omitempty"`
	Body          string          `json:"body,omitempty"`
	Scenes        []Scene         `json:"scenes,omitempty"`
	Status        string          `json:"status"`
	PromptVersion string          `json:"prompt_version,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type Scene struct {
	ID              string `json:"id"`
	Order           int    `json:"order"`
	Title           string `json:"title"`
	VisualPrompt    string `json:"visual_prompt"`
	Narration       string `json:"narration"`
	DurationSeconds int    `json:"duration_seconds"`
}

func NewSceneID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "scene-" + hex.EncodeToString(raw[:]), nil
}

type Document struct {
	Summary   string    `json:"summary"`
	Body      string    `json:"body"`
	Scenes    []Scene   `json:"scenes"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (d *Document) UpdateBody(body string, now time.Time) error {
	if strings.TrimSpace(body) == "" {
		return errors.New("story body is required")
	}
	d.Body = body
	d.UpdatedAt = now
	return nil
}

func (d *Document) AddScene(s Scene) error {
	if strings.TrimSpace(s.Title) == "" {
		return errors.New("scene title is required")
	}
	if s.ID == "" {
		id, err := NewSceneID()
		if err != nil {
			return err
		}
		s.ID = id
	}
	for _, existing := range d.Scenes {
		if existing.ID == s.ID {
			return errors.New("scene id already exists")
		}
	}
	s.Order = len(d.Scenes) + 1
	d.Scenes = append(d.Scenes, s)
	return nil
}

func (d *Document) CopyScene(id string) (Scene, error) {
	for _, existing := range d.Scenes {
		if existing.ID == id {
			copy := existing
			copy.ID = ""
			if err := d.AddScene(copy); err != nil {
				return Scene{}, err
			}
			return d.Scenes[len(d.Scenes)-1], nil
		}
	}
	return Scene{}, errors.New("scene not found")
}

func (d *Document) UpdateScene(id string, changes Scene) (Scene, error) {
	if strings.TrimSpace(changes.Title) == "" || strings.TrimSpace(changes.VisualPrompt) == "" || changes.DurationSeconds <= 0 {
		return Scene{}, errors.New("scene title, visual prompt and positive duration are required")
	}
	for i := range d.Scenes {
		if d.Scenes[i].ID == id {
			changes.ID = id
			changes.Order = d.Scenes[i].Order
			d.Scenes[i] = changes
			return changes, nil
		}
	}
	return Scene{}, errors.New("scene not found")
}

func (d *Document) MoveScene(id string, newOrder int) error {
	if newOrder < 1 || newOrder > len(d.Scenes) {
		return errors.New("scene order out of range")
	}
	oldIndex := -1
	for i := range d.Scenes {
		if d.Scenes[i].ID == id {
			oldIndex = i
			break
		}
	}
	if oldIndex < 0 {
		return errors.New("scene not found")
	}
	scene := d.Scenes[oldIndex]
	d.Scenes = append(d.Scenes[:oldIndex], d.Scenes[oldIndex+1:]...)
	destination := newOrder - 1
	d.Scenes = append(d.Scenes, Scene{})
	copy(d.Scenes[destination+1:], d.Scenes[destination:])
	d.Scenes[destination] = scene
	d.normalizeOrders()
	return nil
}

func (d *Document) DeleteScene(id string) (Scene, error) {
	for i, scene := range d.Scenes {
		if scene.ID == id {
			d.Scenes = append(d.Scenes[:i], d.Scenes[i+1:]...)
			d.normalizeOrders()
			return scene, nil
		}
	}
	return Scene{}, errors.New("scene not found")
}

func (d *Document) normalizeOrders() {
	for i := range d.Scenes {
		d.Scenes[i].Order = i + 1
	}
}
