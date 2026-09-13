package application

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"net/http"
	"time"
)

type ProviderService struct{ Repo provider.Repository }

func (s ProviderService) Save(c provider.Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	return s.Repo.Save(c)
}
func (s ProviderService) List(k provider.Capability) []provider.Config { return s.Repo.List(k) }
func (s ProviderService) Get(code string) (provider.Config, bool)      { return s.Repo.Get(code) }
func (s ProviderService) Delete(code string) error                     { return s.Repo.Delete(code) }
func (s ProviderService) SetEnabled(code string, enabled bool) (provider.Config, error) {
	c, ok := s.Repo.Get(code)
	if !ok {
		return c, errors.New("provider not found")
	}
	if enabled {
		c.Enable()
	} else {
		c.Disable()
	}
	return c, s.Repo.Save(c)
}
func (s ProviderService) TestConnection(ctx context.Context, code string) (provider.Config, error) {
	c, ok := s.Repo.Get(code)
	if !ok {
		return c, errors.New("provider not found")
	}
	if c.BaseURL == "" || c.Model == "" {
		err := errors.New("base_url and model are required")
		c.MarkError(err)
		_ = s.Repo.Save(c)
		return c, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.BaseURL, nil)
	if err != nil {
		return c, err
	}
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		c.MarkError(err)
		_ = s.Repo.Save(c)
		return c, err
	}
	_ = res.Body.Close()
	if res.StatusCode >= 500 {
		err = errors.New("provider endpoint unavailable")
		c.MarkError(err)
		_ = s.Repo.Save(c)
		return c, err
	}
	c.MarkHealthy()
	return c, s.Repo.Save(c)
}
