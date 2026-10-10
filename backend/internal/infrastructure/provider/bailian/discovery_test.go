package bailian

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func discoveryClient(t *testing.T, responses ...string) (Client, *int) {
	t.Helper()
	calls := 0
	client := Client{HTTP: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if calls >= len(responses) {
			t.Fatalf("unexpected request: %s", req.URL)
		}
		if req.Method != "GET" || req.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("incorrect directory request")
		}
		body := responses[calls]
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	return client, &calls
}

func TestNativeDiscoveryPaginatesUnknownVideoAndSpeech(t *testing.T) {
	calls := 0
	client := Client{HTTP: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Path != "/api/v1/models" || req.URL.Query().Get("page_size") != "100" {
			t.Fatal(req.URL)
		}
		body := `{"success":true,"output":{"total":3,"models":[{"model":"unfamiliar-video","capabilities":["VG","VG"]},{"model":"unfamiliar-text","capabilities":["TG","Reasoning"]}]}}`
		if calls == 2 {
			if req.URL.Query().Get("page_no") != "2" {
				t.Fatal(req.URL)
			}
			body = `{"success":true,"output":{"total":3,"models":[{"model":"unfamiliar-voice","capabilities":["TTS"]}]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	items, err := client.Discover(context.Background(), directoryConnection(), "test-key")
	if err != nil || calls != 2 || len(items) != 3 {
		t.Fatal(items, calls, err)
	}
	for i, capability := range []provider.Capability{provider.Video, provider.Story, provider.Speech} {
		if len(items[i].Capabilities) != 1 || items[i].Capabilities[0] != capability || items[i].Name != items[i].ID {
			t.Fatal(items[i])
		}
	}
}

func directoryConnection() provider.Connection {
	return provider.Connection{ID: "c", Name: "Test", Vendor: "custom", BaseURL: "https://example.com"}
}

func TestNativeDiscoveryRejectsIncompleteOrWrongDirectory(t *testing.T) {
	for _, response := range []string{
		`{"success":false}`, `{"data":[]}`, `{"output":{"total":1,"models":[]}}`,
		`{"output":{"models":[]}}`, `{"output":{"total":1,"models":[{}]}}`,
	} {
		client, _ := discoveryClient(t, response)
		if _, err := client.Discover(context.Background(), directoryConnection(), "test-key"); err == nil {
			t.Fatal("accepted incomplete directory", response)
		}
	}
	page := `{"output":{"total":2,"models":[{"model":"same"}]}}`
	client, calls := discoveryClient(t, page, page)
	if _, err := client.Discover(context.Background(), directoryConnection(), "test-key"); err == nil || *calls != 2 {
		t.Fatal("repeated page accepted", err)
	}
	client, _ = discoveryClient(t, `{"output":{"total":0,"models":[]}}`)
	if items, err := client.Discover(context.Background(), directoryConnection(), "test-key"); err != nil || len(items) != 0 {
		t.Fatal(items, err)
	}
}

func TestCompatibleDirectoryDoesNotInferCapabilities(t *testing.T) {
	client, calls := discoveryClient(t, `{"data":[{"id":"qwen-plus"}],"has_more":true,"last_id":"qwen-plus"}`, `{"data":[{"id":"new-video"}]}`)
	items, err := (CompatibleDiscovery{HTTP: client.HTTP}).Discover(context.Background(), directoryConnection(), "test-key")
	if err != nil || *calls != 2 || len(items) != 2 || len(items[0].Capabilities) != 0 {
		t.Fatal(items, err)
	}
	for _, body := range []string{`{"output":{}}`, `{"data":[],"has_more":true}`} {
		client, _ = discoveryClient(t, body)
		if _, err := (CompatibleDiscovery{HTTP: client.HTTP}).Discover(context.Background(), directoryConnection(), "test-key"); err == nil {
			t.Fatal("accepted incompatible response")
		}
	}
}
