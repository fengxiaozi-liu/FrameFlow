package story

import (
	"errors"
	"strings"
	"time"
)

type Scene struct {
	ID              string `json:"id"`
	Order           int    `json:"order"`
	Title           string `json:"title"`
	VisualPrompt    string `json:"visual_prompt"`
	Narration       string `json:"narration"`
	DurationSeconds int    `json:"duration_seconds"`
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
	s.Order = len(d.Scenes) + 1
	d.Scenes = append(d.Scenes, s)
	return nil
}
