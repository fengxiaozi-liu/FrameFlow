package provider

import (
	"context"
	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider/bailian"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type directoryStub string

func (d directoryStub) Discover(context.Context, domain.Connection, string) ([]domain.DiscoveredModel, error) {
	return []domain.DiscoveredModel{{ID: string(d)}}, nil
}

func TestRegisteredVendorsUseTheirOwnDirectoryEndpoints(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("incorrect model directory request")
		}
		switch r.URL.Path {
		case "/api/v1/models":
			if r.URL.Query().Get("page_no") != "1" {
				t.Error("missing native pagination")
			}
			io.WriteString(w, `{"output":{"total":1,"models":[{"model":"unfamiliar-video","capabilities":["VG"]}]}}`)
		case "/v1/models":
			io.WriteString(w, `{"data":[{"id":"unfamiliar-model"}]}`)
		default:
			t.Errorf("unexpected directory: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	r := NewDiscovery()
	native := r.adapters["bailian"].(bailian.Client)
	native.HTTP = server.Client()
	r.adapters["bailian"] = native
	compatible := r.adapters["custom"].(bailian.CompatibleDiscovery)
	compatible.HTTP = server.Client()
	r.adapters["custom"] = compatible
	connection := domain.Connection{ID: "test", Name: "Test", BaseURL: server.URL}
	for _, vendor := range []string{"bailian", "custom"} {
		connection.Vendor = vendor
		models, err := r.Discover(context.Background(), connection, "test-key")
		if err != nil || len(models) != 1 {
			t.Fatal(models, err)
		}
		if vendor == "bailian" && (len(models[0].Capabilities) != 1 || models[0].Capabilities[0] != domain.Video) {
			t.Fatal(models)
		}
		if vendor == "custom" && len(models[0].Capabilities) != 0 {
			t.Fatal("ID-only directory invented capabilities")
		}
	}
	connection.Vendor = "not-registered"
	if _, err := r.Discover(context.Background(), connection, "test-key"); err == nil {
		t.Fatal("unsupported vendor was not rejected")
	}
	if calls != 2 {
		t.Fatalf("unexpected probes or fallbacks: %d", calls)
	}
}

func TestDirectorySelectedByVendor(t *testing.T) {
	r := &DiscoveryRegistry{adapters: map[string]domain.Discovery{
		"bailian": directoryStub("native"), "custom": directoryStub("compatible"),
	}}
	for vendor, want := range map[string]string{"bailian": "native", "custom": "compatible"} {
		models, err := r.Discover(context.Background(), domain.Connection{Vendor: vendor, BaseURL: "https://same.example"}, "")
		if err != nil || models[0].ID != want {
			t.Fatal(models, err)
		}
	}
	if _, err := r.Discover(context.Background(), domain.Connection{Vendor: "unsupported"}, ""); err == nil {
		t.Fatal("unregistered vendor performed discovery")
	}
}
