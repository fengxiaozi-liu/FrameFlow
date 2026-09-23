package application

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/fault"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/gin-gonic/gin"
)

type CatalogService struct {
	Connections provider.ConnectionRepository
	Models      provider.ModelRepository
	Tasks       task.Repository
	Vault       provider.CredentialVault
	Discovery   provider.Discovery
}

func (s CatalogService) VideoCapability(g *gin.Context) {
	model, err := s.Models.GetModel(g.Request.Context(), g.Param("model"))
	if err != nil {
		writeError(g, err)
		return
	}
	capability, known := provider.VideoCapabilityFor(model.RemoteID)
	if !known || !model.Supports(provider.Video) {
		g.JSON(http.StatusOK, gin.H{"supported": false, "reason": "该模型的视频输入能力尚未确认"})
		return
	}
	g.JSON(http.StatusOK, gin.H{"supported": true, "capability": capability})
}

func (s CatalogService) ListConnections(g *gin.Context) {
	items, err := s.Connections.ListConnections(g.Request.Context())
	if err != nil {
		writeError(g, err)
		return
	}
	g.JSON(http.StatusOK, gin.H{"connections": items})
}

func (s CatalogService) SaveConnection(g *gin.Context) {
	ctx := g.Request.Context()
	var input struct {
		provider.Connection
		APIKey string `json:"api_key"`
	}
	if err := g.ShouldBindJSON(&input); err != nil {
		writeError(g, Invalid("invalid_connection", err))
		return
	}
	c := input.Connection
	if id := g.Param("id"); id != "" {
		current, err := s.Connections.GetConnection(ctx, id)
		if err != nil {
			writeError(g, err)
			return
		}
		if current.Vendor != c.Vendor {
			writeError(g, Invalid("immutable_connection_scope", errors.New("vendor cannot change; create a new connection")))
			return
		}
		c.ID = id
	}
	if err := c.Validate(); err != nil {
		writeError(g, Invalid("invalid_connection", err))
		return
	}
	u, _ := url.Parse(strings.TrimSpace(c.BaseURL))
	if u.Path == "" || u.Path == "/" {
		writeError(g, Invalid("invalid_connection", errors.New("base_url must be a complete API endpoint, not just a host")))
		return
	}
	if s.Vault != nil {
		if input.APIKey != "" {
			if err := s.Vault.Put(ctx, "connection:"+c.ID, input.APIKey); err != nil {
				writeError(g, err)
				return
			}
		}
		var err error
		c.CredentialSet, err = s.Vault.Has(ctx, "connection:"+c.ID)
		if err != nil {
			writeError(g, err)
			return
		}
	}
	if err := s.Connections.SaveConnection(ctx, c); err != nil {
		writeError(g, err)
		return
	}
	status := http.StatusCreated
	if g.Param("id") != "" {
		status = http.StatusOK
	}
	g.JSON(status, c)
}

func (s CatalogService) DeleteConnection(g *gin.Context) {
	ctx := g.Request.Context()
	id := g.Param("id")
	if err := s.Connections.DeleteConnection(ctx, id); err != nil {
		writeError(g, Conflict("connection_has_models", err))
		return
	}
	if s.Vault != nil {
		if err := s.Vault.Delete(ctx, "connection:"+id); err != nil {
			writeError(g, err)
			return
		}
	}
	g.Status(http.StatusNoContent)
}

func (s CatalogService) TestConnection(g *gin.Context) {
	ctx := g.Request.Context()
	c, err := s.Connections.GetConnection(ctx, g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	if s.Discovery == nil || s.Vault == nil {
		writeError(g, unavailable("discovery_unavailable", "model discovery is not configured"))
		return
	}
	reader, ok := s.Vault.(provider.CredentialReader)
	if !ok {
		writeError(g, unavailable("discovery_unavailable", "credential reader is not configured"))
		return
	}
	key, err := reader.Get(ctx, "connection:"+c.ID)
	if err != nil {
		writeError(g, Invalid("missing_api_key", err))
		return
	}
	models, err := s.Discovery.Discover(ctx, c, key)
	if err != nil {
		writeError(g, &Error{Kind: "provider", Code: "connection_failed", Cause: err})
		return
	}
	g.JSON(http.StatusOK, gin.H{"reachable": true, "model_count": len(models)})
}

func (s CatalogService) ListModels(g *gin.Context) {
	ctx := g.Request.Context()
	connectionID := g.Query("connection_id")
	items, err := s.Models.ListModels(ctx, connectionID)
	if err != nil {
		writeError(g, err)
		return
	}
	if cap := provider.Capability(g.Query("capability")); cap != "" {
		filtered := make([]provider.Model, 0)
		for _, item := range items {
			for _, c := range item.Capabilities {
				if c == cap {
					filtered = append(filtered, item)
					break
				}
			}
		}
		items = filtered
	}
	g.JSON(http.StatusOK, gin.H{"models": items})
}

func (s CatalogService) AddModel(g *gin.Context) {
	ctx := g.Request.Context()
	id := g.Param("id")
	_, err := s.Connections.GetConnection(ctx, id)
	if err != nil {
		writeError(g, err)
		return
	}
	var input struct {
		ModelID      string                `json:"model_id"`
		Name         string                `json:"name"`
		Capabilities []provider.Capability `json:"capabilities"`
	}
	if err := g.ShouldBindJSON(&input); err != nil {
		writeError(g, Invalid("invalid_model", err))
		return
	}
	input.ModelID = strings.TrimSpace(input.ModelID)
	if input.ModelID == "" || strings.ContainsAny(input.ModelID, "/\\?# ") {
		writeError(g, Invalid("invalid_model", errors.New("invalid model id")))
		return
	}
	modelID := provider.ModelID(id, input.ModelID)
	if _, err = s.Models.GetModel(ctx, modelID); err == nil {
		writeError(g, Conflict("model_exists", errors.New("model already exists")))
		return
	} else if !errors.Is(err, fault.ErrNotFound) {
		writeError(g, err)
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = input.ModelID
	}
	capabilities, err := validCapabilities(input.Capabilities)
	if err != nil {
		writeError(g, Invalid("invalid_capabilities", err))
		return
	}
	m := provider.Model{ID: modelID, ConnectionID: id, RemoteID: input.ModelID, Name: name, Source: "manual", Capabilities: capabilities, Supported: len(capabilities) > 0, Enabled: len(capabilities) > 0}
	if err := s.Models.SaveModel(ctx, m); err != nil {
		writeError(g, err)
		return
	}
	g.JSON(http.StatusCreated, m)
}

func (s CatalogService) SyncModels(g *gin.Context) {
	ctx := g.Request.Context()
	c, err := s.Connections.GetConnection(ctx, g.Param("id"))
	if err != nil {
		writeError(g, err)
		return
	}
	if s.Discovery == nil || s.Vault == nil {
		writeError(g, unavailable("discovery_unavailable", "model discovery is not configured"))
		return
	}
	reader, ok := s.Vault.(provider.CredentialReader)
	if !ok {
		writeError(g, unavailable("discovery_unavailable", "credential reader is not configured"))
		return
	}
	key, err := reader.Get(ctx, "connection:"+c.ID)
	if err != nil {
		writeError(g, Invalid("missing_api_key", err))
		return
	}
	discovered, err := s.Discovery.Discover(ctx, c, key)
	if err != nil {
		writeError(g, &Error{Kind: "provider", Code: "discovery_failed", Cause: err})
		return
	}
	current, err := s.Models.ListModels(ctx, c.ID)
	if err != nil {
		writeError(g, err)
		return
	}
	byID := make(map[string]provider.Model, len(current))
	for _, m := range current {
		byID[m.ID] = m
	}
	now := time.Now().UTC()
	count := 0
	for _, entry := range discovered {
		if entry.ID == "" {
			continue
		}
		id := provider.ModelID(c.ID, entry.ID)
		m, exists := byID[id]
		if !exists {
			if entry.Name == "" {
				entry.Name = entry.ID
			}
			m = provider.Model{ID: id, ConnectionID: c.ID, RemoteID: entry.ID, Name: entry.Name, Source: "discovered"}
		}
		if m.Source == "manual" {
			continue
		}
		wasSupported := m.Supported
		m.Name = entry.Name
		if !exists || len(m.Capabilities) == 0 {
			m.Capabilities, err = validCapabilities(entry.Capabilities)
			if err != nil {
				writeError(g, Invalid("invalid_capabilities", err))
				return
			}
		}
		m.Supported = len(m.Capabilities) > 0
		if !exists || !wasSupported {
			m.Enabled = m.Supported
		}
		m.LastSeen = now
		if err := s.Models.SaveModel(ctx, m); err != nil {
			writeError(g, fmt.Errorf("save discovered model: %w", err))
			return
		}
		count++
	}
	g.JSON(http.StatusOK, gin.H{"synced": count})
}

func (s CatalogService) UpdateModel(g *gin.Context) {
	ctx := g.Request.Context()
	m, err := s.Models.GetModel(ctx, g.Param("model"))
	if err != nil {
		writeError(g, err)
		return
	}
	if m.ConnectionID != g.Param("id") {
		writeError(g, fault.ErrNotFound)
		return
	}
	var input struct {
		Enabled      *bool                  `json:"enabled"`
		Default      *bool                  `json:"default"`
		Capabilities *[]provider.Capability `json:"capabilities"`
	}
	if err := g.ShouldBindJSON(&input); err != nil {
		writeError(g, Invalid("invalid_model", err))
		return
	}
	if input.Capabilities != nil {
		m.Capabilities, err = validCapabilities(*input.Capabilities)
		if err != nil {
			writeError(g, Invalid("invalid_capabilities", err))
			return
		}
		m.Supported = len(m.Capabilities) > 0
		if !m.Supported {
			m.Enabled = false
			m.Default = false
		}
	}
	if input.Enabled != nil {
		if *input.Enabled && !m.Supported {
			writeError(g, Invalid("unsupported_model", provider.ErrUnsupported))
			return
		}
		m.Enabled = *input.Enabled
	}
	if input.Default != nil && *input.Default {
		if !m.Enabled || len(m.Capabilities) == 0 {
			writeError(g, Invalid("unsupported_model", provider.ErrUnsupported))
			return
		}
		all, err := s.Models.ListModels(ctx, "")
		if err != nil {
			writeError(g, err)
			return
		}
		for _, other := range all {
			if other.ID == m.ID || !other.Default {
				continue
			}
			for _, cap := range m.Capabilities {
				for _, otherCap := range other.Capabilities {
					if cap == otherCap {
						other.Default = false
						if err := s.Models.SaveModel(ctx, other); err != nil {
							writeError(g, err)
							return
						}
					}
				}
			}
		}
	}
	if input.Default != nil {
		m.Default = *input.Default
	}
	if !m.Enabled {
		m.Default = false
	}
	if err := s.Models.SaveModel(ctx, m); err != nil {
		writeError(g, err)
		return
	}
	g.JSON(http.StatusOK, m)
}

func validCapabilities(items []provider.Capability) ([]provider.Capability, error) {
	result := make([]provider.Capability, 0, len(items))
	for _, cap := range items {
		if cap != provider.Story && cap != provider.Image && cap != provider.Video {
			return nil, fmt.Errorf("unknown capability %q", cap)
		}
		found := false
		for _, existing := range result {
			if existing == cap {
				found = true
				break
			}
		}
		if !found {
			result = append(result, cap)
		}
	}
	return result, nil
}

func (s CatalogService) DeleteModel(g *gin.Context) {
	ctx := g.Request.Context()
	m, err := s.Models.GetModel(ctx, g.Param("model"))
	if err != nil {
		writeError(g, err)
		return
	}
	if m.ConnectionID != g.Param("id") {
		writeError(g, fault.ErrNotFound)
		return
	}
	if s.Tasks != nil {
		tasks, err := s.Tasks.List(ctx)
		if err != nil {
			writeError(g, err)
			return
		}
		for _, item := range tasks {
			if item.ModelID == m.ID {
				writeError(g, Conflict("model_in_use", errors.New("model is referenced by tasks; disable it instead")))
				return
			}
		}
	}
	if err := s.Models.DeleteModel(ctx, m.ID); err != nil {
		writeError(g, err)
		return
	}
	g.Status(http.StatusNoContent)
}
