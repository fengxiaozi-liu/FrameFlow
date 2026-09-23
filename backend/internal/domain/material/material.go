package material

import (
	"errors"
	"strings"
	"time"
)

type Kind string

const (
	Visual    Kind = "visual"
	Frame     Kind = "frame"
	Character Kind = "character"
	Voice     Kind = "voice"
	Music     Kind = "music"
	Scene     Kind = "scene"
	Prop      Kind = "prop"
)

type MediaInfo struct {
	Format          string  `json:"format,omitempty"`
	SizeBytes       int64   `json:"size_bytes,omitempty"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
}

type Asset struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       Kind      `json:"kind"`
	LegacyKind Kind      `json:"legacy_kind,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	Media      MediaInfo `json:"media,omitempty"`
	URL        string    `json:"url"`
	CreatedAt  time.Time `json:"created_at"`
}

func (a *Asset) NormalizeLegacyKind() bool {
	switch a.Kind {
	case Visual:
		a.LegacyKind = Visual
		a.Kind = Scene
		return true
	case Frame:
		a.LegacyKind = Frame
		a.Kind = Scene
		for _, tag := range a.Tags {
			if tag == "原首尾帧" {
				return true
			}
		}
		a.Tags = append(a.Tags, "原首尾帧")
		return true
	default:
		return false
	}
}

func New(id, name string, k Kind, now time.Time) (Asset, error) {
	if id == "" || strings.TrimSpace(name) == "" {
		return Asset{}, errors.New("material id and name are required")
	}
	return Asset{ID: id, Name: name, Kind: k, CreatedAt: now}, nil
}

func (a *Asset) UpdateURL(url string) error {
	if strings.TrimSpace(url) == "" {
		return errors.New("material url is required")
	}
	a.URL = url
	return nil
}
