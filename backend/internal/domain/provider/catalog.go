package provider

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

type Connection struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Vendor        string `json:"vendor"`
	Region        string `json:"region"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	BaseURL       string `json:"base_url"`
	CredentialSet bool   `json:"credential_set"`
}

type Model struct {
	ID           string       `json:"id"`
	ConnectionID string       `json:"connection_id"`
	RemoteID     string       `json:"remote_id"`
	Name         string       `json:"name"`
	Capabilities []Capability `json:"capabilities"`
	Source       string       `json:"source"`
	Supported    bool         `json:"supported"`
	Enabled      bool         `json:"enabled"`
	Default      bool         `json:"default"`
	LastSeen     time.Time    `json:"last_seen,omitempty"`
}

func (m Model) Supports(cap Capability) bool {
	if !m.Enabled || !m.Supported {
		return false
	}
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func ModelID(connectionID, remoteID string) string {
	return connectionID + ":" + remoteID
}

func (c Connection) Validate() error {
	if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Name) == "" || c.Vendor == "" {
		return errors.New("connection id, name and vendor are required")
	}
	for _, value := range []string{c.ID, c.WorkspaceID} {
		for _, char := range value {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-') {
				return errors.New("connection and workspace IDs must contain only letters, digits or hyphens")
			}
		}
	}
	if c.Vendor != "bailian" {
		return errors.New("unsupported vendor")
	}
	if c.Vendor == "bailian" {
		u, err := url.Parse(c.BaseURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return errors.New("bailian base_url must be an HTTPS origin")
		}
	}
	return nil
}

type ConnectionRepository interface {
	SaveConnection(context.Context, Connection) error
	GetConnection(context.Context, string) (Connection, error)
	ListConnections(context.Context) ([]Connection, error)
	DeleteConnection(context.Context, string) error
}

type ModelRepository interface {
	SaveModel(context.Context, Model) error
	GetModel(context.Context, string) (Model, error)
	ListModels(context.Context, string) ([]Model, error)
	DeleteModel(context.Context, string) error
}

type DiscoveredModel struct {
	ID           string
	Name         string
	Capabilities []Capability
}

type Discovery interface {
	Discover(context.Context, Connection, string) ([]DiscoveredModel, error)
}

type CredentialReader interface {
	Get(context.Context, string) (string, error)
}

var ErrUnsupported = errors.New("model does not support this operation")

// Provider is the application-facing contract. Vendor-specific wire formats stay in infrastructure.
type Provider interface {
	GenerateText(context.Context, StoryRequest, RequestOptions) (StoryResult, error)
	GenerateImage(context.Context, ImageRequest, RequestOptions) (ImageResult, error)
	GenerateVideo(context.Context, VideoRequest, RequestOptions) (VideoResult, error)
}

type ModelClient interface {
	GenerateText(context.Context, Connection, Model, StoryRequest, RequestOptions) (StoryResult, error)
	GenerateImage(context.Context, Connection, Model, ImageRequest, RequestOptions) (ImageResult, error)
	GenerateVideo(context.Context, Connection, Model, VideoRequest, RequestOptions) (VideoResult, error)
	PollVideo(context.Context, Connection, Model, string) (VideoResult, bool, error)
}

type VideoPoller interface {
	PollVideo(context.Context, string) (VideoResult, bool, error)
}
