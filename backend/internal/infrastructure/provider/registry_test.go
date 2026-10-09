package provider

import (
	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"testing"
)

func TestRegistryUsesProtocolNotVendorOrModel(t *testing.T) {
	r := NewRegistry(nil)
	m := domain.MigrateLegacyModel(domain.Model{RemoteID: "qwen-plus", Enabled: true, Capabilities: []domain.Capability{domain.Story}})
	m.RemoteID = "custom-unknown"
	if _, err := r.client(m, domain.Story); err != nil {
		t.Fatal(err)
	}
	if _, err := r.client(m, domain.Image); err == nil {
		t.Fatal("unconfigured operation accepted")
	}
	r.clients = map[string]ProtocolAdapter{}
	if _, err := r.client(m, domain.Story); err == nil {
		t.Fatal("missing registration accepted")
	}
}

func TestCustomJSONIsIndependentlyRegistered(t *testing.T) {
	r := NewRegistry(nil)
	custom, ok := r.clients[domain.CustomProtocol]
	if !ok || custom.Text == nil || custom.Image == nil || custom.Video == nil || custom.Speech == nil {
		t.Fatal("custom adapter missing operation")
	}
	if custom.Audio != nil {
		t.Fatal("unimplemented upload advertised")
	}
	m := domain.Model{RemoteID: "unknown-custom-model", Enabled: true, Capabilities: []domain.Capability{domain.Story}, Operations: map[domain.Capability]domain.OperationBinding{domain.Story: {Protocol: domain.CustomProtocol, Custom: &domain.CustomInterface{Path: "/generate", Mode: "sync", AuthMode: "none", BodyTemplate: `{"model":"{{model}}","prompt":"{{prompt}}"}`, ResultPointer: "/text"}}}}
	if _, err := r.client(m, domain.Story); err != nil {
		t.Fatal(err)
	}
}
