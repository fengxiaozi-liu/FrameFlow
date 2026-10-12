package custom

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

type discoveryRoundTrip func(*http.Request) (*http.Response, error)

func (f discoveryRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func directoryConnection() provider.Connection {
	return provider.Connection{ID: "c", Name: "Test", Vendor: "custom", BaseURL: "https://example.com"}
}

func discoveryClient(t *testing.T, responses ...string) (DiscoveryClient, *int) {
	t.Helper()
	calls := 0
	client := DiscoveryClient{HTTP: &http.Client{Transport: discoveryRoundTrip(func(req *http.Request) (*http.Response, error) {
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

func TestCompatibleDirectoryDoesNotInferCapabilities(t *testing.T) {
	client, calls := discoveryClient(t, `{"data":[{"id":"qwen-plus"}],"has_more":true,"last_id":"qwen-plus"}`, `{"data":[{"id":"new-video"}]}`)
	items, err := (DiscoveryClient{HTTP: client.HTTP}).Discover(context.Background(), directoryConnection(), "test-key")
	if err != nil || *calls != 2 || len(items) != 2 || len(items[0].Capabilities) != 0 {
		t.Fatal(items, err)
	}
	for _, body := range []string{`{"output":{}}`, `{"data":[],"has_more":true}`} {
		client, _ = discoveryClient(t, body)
		if _, err := (DiscoveryClient{HTTP: client.HTTP}).Discover(context.Background(), directoryConnection(), "test-key"); err == nil {
			t.Fatal("accepted incompatible response")
		}
	}
}

func TestCompatibleDiscovery(t *testing.T) {
	c := DiscoveryClient{HTTP: &http.Client{Transport: discoveryRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/models" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"qwen3-tts-flash"},{"id":"qwen-image-2.0-pro"}]}`))}, nil
	})}}
	models, err := c.Discover(context.Background(), provider.Connection{ID: "c", Name: "test", Vendor: "bailian", BaseURL: "https://example.com"}, "test")
	if err != nil || len(models) != 2 || len(models[0].Capabilities) != 0 {
		t.Fatal(models, err)
	}
}
