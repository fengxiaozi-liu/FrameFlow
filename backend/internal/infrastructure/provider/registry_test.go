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
