package provider

import (
	"errors"
	"strings"
	"time"
)

// ApplyDiscoveredIdentity never grants capabilities or replaces user settings.
func (m Model) ApplyDiscoveredIdentity(connectionID string, entry DiscoveredModel, seen time.Time) (Model, error) {
	if entry.ID == "" || strings.ContainsAny(entry.ID, "\r\n\x00") {
		return m, errors.New("模型 ID 无效")
	}
	if m.ID == "" {
		m = Model{ID: ModelID(connectionID, entry.ID), ConnectionID: connectionID, RemoteID: entry.ID, Source: "discovered", Capabilities: []Capability{}, Operations: map[Capability]OperationBinding{}, Revision: 1}
	}
	if m.ConnectionID != connectionID || m.RemoteID != entry.ID {
		return m, errors.New("模型身份不匹配")
	}
	if m.Source == "manual" {
		return m, nil
	}
	if !m.NameConfigured {
		m.Name = entry.Name
		if strings.TrimSpace(m.Name) == "" {
			m.Name = entry.ID
		}
	}
	m.LastSeen = seen
	return m, nil
}
