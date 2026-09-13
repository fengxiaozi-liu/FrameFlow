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
)

type Asset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      Kind      `json:"kind"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
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
