package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/request"
	"github.com/fengxiaozi-liu/FrameFlow/internal/transport/response"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type ProviderService struct {
	Repo  provider.Repository
	Vault provider.CredentialVault
}

func (s ProviderService) Save(g *gin.Context) {
	ctx := g.Request.Context()
	var input struct {
		provider.Config
		APIKey string `json:"api_key"`
	}
	if !request.BindJSON(g, &input) {
		return
	}
	c := input.Config
	status := 201
	if code := g.Param("id"); code != "" {
		c.Code = code
		status = 200
	}
	if err := c.Validate(); err != nil {
		response.Set(g, 0, nil, Invalid("invalid_provider", err))
		return
	}
	if s.Vault != nil {
		if input.APIKey != "" {
			if err := s.Vault.Put(ctx, c.Code, input.APIKey); err != nil {
				response.Set(g, 0, nil, err)
				return
			}
			c.CredentialSet = true
		} else {
			var err error
			c.CredentialSet, err = s.Vault.Has(ctx, c.Code)
			if err != nil {
				response.Set(g, 0, nil, err)
				return
			}
		}
	}
	err := s.Repo.Save(ctx, c)
	response.Set(g, status, c, err)
}
func (s ProviderService) List(g *gin.Context) {
	items, err := s.Repo.List(g.Request.Context(), provider.Capability(g.Query("capability")))
	response.Set(g, 200, gin.H{"providers": items}, err)
}
func (s ProviderService) Get(g *gin.Context) {
	item, err := s.Repo.Get(g.Request.Context(), g.Param("id"))
	response.Set(g, 200, item, err)
}
func (s ProviderService) Delete(g *gin.Context) {
	ctx := g.Request.Context()
	id := g.Param("id")
	err := s.Repo.Delete(ctx, id)
	if err == nil && s.Vault != nil {
		err = s.Vault.Delete(ctx, id)
	}
	response.Set(g, 204, nil, err)
}
func (s ProviderService) SetEnabled(g *gin.Context) {
	ctx := g.Request.Context()
	code := g.Param("id")
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if !request.BindJSON(g, &input) {
		return
	}
	enabled := input.Enabled
	c, err := s.Repo.Get(ctx, code)
	if err != nil {
		response.Set(g, 200, c, err)
		return
	}
	if enabled {
		c.Enable()
	} else {
		c.Disable()
	}
	response.Set(g, 200, c, s.Repo.Save(ctx, c))
	return
}
func (s ProviderService) TestConnection(g *gin.Context) {
	ctx := g.Request.Context()
	code := g.Param("id")
	c, err := s.Repo.Get(ctx, code)
	if err != nil {
		response.Set(g, 200, c, err)
		return
	}
	if c.BaseURL == "" || c.Model == "" {
		err := errors.New("base_url and model are required")
		c.MarkError(err)
		_ = s.Repo.Save(ctx, c)
		response.Set(g, 200, c, &Error{Kind: "provider", Code: "provider_unavailable", Cause: err})
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.BaseURL, nil)
	if err != nil {
		response.Set(g, 200, c, &Error{Kind: "provider", Code: "provider_unavailable", Cause: err})
		return
	}
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		c.MarkError(err)
		_ = s.Repo.Save(ctx, c)
		response.Set(g, 200, c, &Error{Kind: "provider", Code: "provider_unavailable", Cause: err})
		return
	}
	_ = res.Body.Close()
	if res.StatusCode >= 500 {
		err = errors.New("provider endpoint unavailable")
		c.MarkError(err)
		_ = s.Repo.Save(ctx, c)
		response.Set(g, 200, c, &Error{Kind: "provider", Code: "provider_unavailable", Cause: err})
		return
	}
	c.MarkHealthy()
	response.Set(g, 200, c, s.Repo.Save(ctx, c))
	return
}
