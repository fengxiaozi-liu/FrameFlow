package provider

import (
	"context"
	"errors"
	"time"
)

type Capability string

const (
	Story Capability = "story"
	Image Capability = "image"
	Video Capability = "video"
)

type Status string

const (
	Disabled Status = "disabled"
	Enabled  Status = "enabled"
	Healthy  Status = "healthy"
)

type Config struct {
	Code          string     `json:"code"`
	Name          string     `json:"name"`
	Vendor        string     `json:"vendor,omitempty"`
	Capability    Capability `json:"capability"`
	Model         string     `json:"model"`
	BaseURL       string     `json:"base_url"`
	Enabled       bool       `json:"enabled"`
	Status        Status     `json:"status"`
	LastError     string     `json:"last_error,omitempty"`
	CredentialSet bool       `json:"credential_set"`
}

type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

type RequestOptions struct {
	Timeout time.Duration
	Retry   RetryPolicy
}

type StoryRequest struct{ Prompt, Model, SystemPrompt string }
type StoryResult struct{ Document string }
type ImageRequest struct{ Prompt, Model, AspectRatio string }
type ImageResult struct{ URL string }
type VideoRequest struct {
	Prompt, Model, AspectRatio, SourceImageURL string
	LastFrameURL, DrivingAudioURL, Resolution  string
	Duration                                   int
}
type VideoResult struct{ JobReference, URL string }

// WithTimeout 是所有 provider 共用的取消和超时处理方式。
func WithTimeout(parent context.Context, options RequestOptions) (context.Context, context.CancelFunc) {
	if options.Timeout <= 0 {
		options.Timeout = 60 * time.Second
	}
	return context.WithTimeout(parent, options.Timeout)
}

func (c Config) Validate() error {
	if c.Code == "" || c.Name == "" {
		return errors.New("provider code and name are required")
	}
	if c.Capability != Story && c.Capability != Image && c.Capability != Video {
		return errors.New("invalid provider capability")
	}
	if c.Enabled && (c.BaseURL == "" || c.Model == "") {
		return errors.New("enabled provider requires base_url and model")
	}
	return nil
}

func (c *Config) Enable() {
	c.Enabled = true
	c.Status = Enabled
	c.LastError = ""
}

func (c *Config) Disable() {
	c.Enabled = false
	c.Status = Disabled
}

func (c *Config) MarkHealthy() {
	c.Enabled = true
	c.Status = Healthy
	c.LastError = ""
}

func (c *Config) MarkError(err error) {
	c.Status = Disabled
	c.LastError = err.Error()
}
