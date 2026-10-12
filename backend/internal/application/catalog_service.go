package application

import (
	"errors"
	"fmt"
	"net/http"
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
	status := model.Resolve(provider.Video)
	if !status.Available {
		g.JSON(http.StatusOK, gin.H{"supported": false, "reason": status.Reason})
		return
	}
	capabilities := map[string]provider.VideoCapability{}
	for _, mode := range status.Binding.VideoInputModes() {
		capability, err := model.VideoCapabilityFor(mode)
		if err == nil {
			capabilities[mode] = capability
		}
	}
	response := gin.H{"supported": true, "capabilities": capabilities}
	if capability, ok := model.VideoCapability(); ok {
		response["capability"] = capability
	}
	g.JSON(http.StatusOK, response)
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
		c.ID = id
	}
	configuration := ConnectionConfiguration{Repository: s.Connections, Vault: s.Vault}
	c, err := configuration.Save(ctx, c, input.APIKey, g.Param("id") == "")
	if err != nil {
		if errors.Is(err, provider.ErrRevisionConflict) {
			writeError(g, Conflict("configuration_revision_conflict", err))
		} else {
			writeError(g, Invalid("invalid_connection", err))
		}
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
		writeError(g, err)
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
	for i := range items {
		caps, _ := items[i].InputCapability()
		items[i].Availability = caps.Operations
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
	if input.ModelID == "" || strings.ContainsAny(input.ModelID, "\r\n\x00") {
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
	m = provider.MigrateLegacyModel(m)
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
	if c.Vendor == "custom" {
		writer, ok := s.Models.(ModelIdentityWriter)
		if !ok {
			writeError(g, unavailable("discovery_unavailable", "identity repository is unavailable"))
			return
		}
		count, err := writer.SyncModelIdentities(ctx, c, discovered, time.Now().UTC())
		if err != nil {
			writeError(g, err)
			return
		}
		g.JSON(http.StatusOK, gin.H{"synced": count})
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
		// Custom directory contracts are identity-only by design. Capabilities are
		// configured on the model's protocol panel and must never be inferred from
		// an arbitrary vendor response. Built-in vendors retain their fixed,
		// documented capability metadata for compatibility.
		acceptDiscovery := c.Vendor != "custom" && (!m.CapabilitiesConfigured && len(m.Capabilities) == 0)
		if acceptDiscovery {
			m.Capabilities, err = validCapabilities(entry.Capabilities)
			if err != nil {
				writeError(g, Invalid("invalid_capabilities", err))
				return
			}
		}
		m.Supported = len(m.Capabilities) > 0
		if acceptDiscovery && (!exists || !wasSupported) {
			m.Enabled = m.Supported
		}
		m.LastSeen = now
		m = provider.MigrateLegacyModel(m)
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
		m, err = m.SelectCapabilities(*input.Capabilities)
		if err != nil {
			writeError(g, Invalid("invalid_capabilities", err))
			return
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
	m = provider.MigrateLegacyModel(m)
	if err := s.Models.SaveModel(ctx, m); err != nil {
		writeError(g, err)
		return
	}
	g.JSON(http.StatusOK, m)
}

func validCapabilities(items []provider.Capability) ([]provider.Capability, error) {
	result := make([]provider.Capability, 0, len(items))
	for _, cap := range items {
		if cap != provider.Story && cap != provider.Image && cap != provider.Video && cap != provider.Speech {
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
