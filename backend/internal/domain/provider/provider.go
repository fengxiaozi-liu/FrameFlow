package provider

import (
	"context"
	"errors"
	"sync"
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

type StoryRequest struct{ Prompt, Model string }
type StoryResult struct{ Document string }
type ImageRequest struct{ Prompt, Model, AspectRatio string }
type ImageResult struct{ URL string }
type VideoRequest struct{ Prompt, Model, AspectRatio, SourceImageURL string }
type VideoResult struct{ JobReference string }

type StoryGenerator interface {
	Generate(context.Context, StoryRequest, RequestOptions) (StoryResult, error)
}
type ImageGenerator interface {
	Generate(context.Context, ImageRequest, RequestOptions) (ImageResult, error)
}
type VideoGenerator interface {
	Generate(context.Context, VideoRequest, RequestOptions) (VideoResult, error)
}

// Adapter 由厂商客户端实现。能力是可选的，厂商可以只实现自身支持的操作。
type Adapter interface{}
type Factory func(context.Context, Config) (Adapter, error)

// Registry 负责解析厂商客户端，并按配置懒加载每个客户端实例。
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
	instances map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{factories: map[string]Factory{}, instances: map[string]Adapter{}}
}

func (r *Registry) Register(vendor string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[vendor] = factory
}

func (r *Registry) Resolve(ctx context.Context, config Config) (Adapter, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	instance, ok := r.instances[config.Code]
	vendor := config.Vendor
	if vendor == "" {
		vendor = "mock"
	}
	factory := r.factories[vendor]
	r.mu.RUnlock()
	if ok {
		return instance, nil
	}
	if factory == nil {
		return nil, errors.New("provider vendor is not registered")
	}
	created, err := factory(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	if existing, exists := r.instances[config.Code]; exists {
		r.mu.Unlock()
		return existing, nil
	}
	r.instances[config.Code] = created
	r.mu.Unlock()
	return created, nil
}

// WithTimeout 是所有 provider 共用的取消和超时处理方式。
func WithTimeout(parent context.Context, options RequestOptions) (context.Context, context.CancelFunc) {
	if options.Timeout <= 0 {
		options.Timeout = 60 * time.Second
	}
	return context.WithTimeout(parent, options.Timeout)
}

func New(code, name string, capability Capability) (Config, error) {
	if code == "" || name == "" {
		return Config{}, errors.New("provider code and name are required")
	}
	return Config{Code: code, Name: name, Capability: capability, Status: Disabled}, nil
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
