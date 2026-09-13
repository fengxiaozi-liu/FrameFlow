package provider

import "errors"

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
	Capability    Capability `json:"capability"`
	Model         string     `json:"model"`
	BaseURL       string     `json:"base_url"`
	Enabled       bool       `json:"enabled"`
	Status        Status     `json:"status"`
	LastError     string     `json:"last_error,omitempty"`
	CredentialSet bool       `json:"credential_set"`
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
func (c *Config) Enable()             { c.Enabled = true; c.Status = Enabled; c.LastError = "" }
func (c *Config) Disable()            { c.Enabled = false; c.Status = Disabled }
func (c *Config) MarkHealthy()        { c.Enabled = true; c.Status = Healthy; c.LastError = "" }
func (c *Config) MarkError(err error) { c.Status = Disabled; c.LastError = err.Error() }
