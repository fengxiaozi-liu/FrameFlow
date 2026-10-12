package custom

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func TestDiscoverUsesConfiguredPathAndIdentityOnly(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/catalog" || r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer local-test-key" {
			t.Fatalf("unexpected request: %s %s auth=%q", r.Method, r.URL, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"items":[{"model_id":"doubao-seedance","title":"Seedance"},{"model_id":"image-model"}]}`))
	}))
	defer server.Close()
	connection := provider.Connection{
		ID: "custom", Name: "Custom", Vendor: "custom", BaseURL: server.URL,
		Discovery: &provider.DiscoveryProfile{
			Method: "GET", Path: "/catalog", ModelListPointer: "/items",
			ModelIDPointer: "/model_id", ModelNamePointer: "/title",
			Pagination: provider.DiscoveryPagination{Mode: provider.DiscoveryNone},
		},
	}
	items, err := (DiscoveryClient{HTTP: server.Client()}).Discover(context.Background(), connection, "local-test-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "doubao-seedance" || items[0].Name != "Seedance" || items[1].Name != "image-model" {
		t.Fatalf("unexpected identities: %#v", items)
	}
	if len(items[0].Capabilities) != 0 {
		t.Fatal("custom discovery must not infer capabilities")
	}
}

func TestCursorCycleIsRejectedWithoutPublishingPartialModels(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		cursor := "A"
		if calls == 2 {
			cursor = "B"
		}
		fmt.Fprintf(w, `{"data":[{"id":"model-%d"}],"more":true,"next":"%s"}`, calls, cursor)
	}))
	defer server.Close()
	p := provider.DefaultCustomDiscoveryProfile()
	p.Pagination = provider.DiscoveryPagination{Mode: provider.DiscoveryCursor, CursorParam: "after", HasMorePointer: "/more", NextCursorPointer: "/next"}
	c := provider.Connection{ID: "c", Name: "Custom", Vendor: "custom", BaseURL: server.URL, Discovery: &p}
	items, err := (DiscoveryClient{HTTP: server.Client()}).Discover(context.Background(), c, "local-placeholder")
	if err == nil || len(items) != 0 || calls != 3 {
		t.Fatal("cursor cycle was not stopped", calls, err)
	}
}

func TestDiscoveryCannotOverrideCustomAuthenticationHeader(t *testing.T) {
	p := provider.DefaultCustomDiscoveryProfile()
	p.Headers = map[string]string{"X-Api-Key": "not-a-credential"}
	c := provider.Connection{ID: "c", Name: "Custom", Vendor: "custom", BaseURL: "https://example.com", Discovery: &p, Auth: &provider.ConnectionAuth{Mode: "header", Header: "X-API-Key"}}
	if _, err := (DiscoveryClient{}).Discover(context.Background(), c, "local-placeholder"); err == nil {
		t.Fatal("authentication header override accepted")
	}
}

func TestDiscoverSupportsPostAndPagePagination(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Query().Get("page") == "" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
		if calls == 1 {
			_, _ = w.Write([]byte(`{"items":[{"id":"one"},{"id":"two"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"three"}]}`))
	}))
	defer server.Close()
	connection := provider.Connection{
		ID: "custom", Name: "Custom", Vendor: "custom", BaseURL: server.URL,
		Discovery: &provider.DiscoveryProfile{
			Method: "POST", Path: "/catalog", RequestJSON: `{"scope":"all"}`,
			ModelListPointer: "/items", ModelIDPointer: "/id",
			Pagination: provider.DiscoveryPagination{Mode: provider.DiscoveryPage, PageParam: "page", PageSize: 2},
		},
	}
	items, err := (DiscoveryClient{HTTP: server.Client()}).Discover(context.Background(), connection, "test")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(items) != 3 {
		t.Fatalf("unexpected pages/items: calls=%d items=%#v", calls, items)
	}
}
