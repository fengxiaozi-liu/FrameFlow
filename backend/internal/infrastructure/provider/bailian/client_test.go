package bailian

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

type credential string

func (c credential) Get(context.Context, string) (string, error) { return string(c), nil }

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestWan27FirstLastAudioRequest(t *testing.T) {
	client := Client{Credentials: credential("secret"), HTTP: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		var payload struct {
			Input struct {
				Media []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"media"`
			} `json:"input"`
			Parameters struct {
				Resolution string `json:"resolution"`
				Duration   int    `json:"duration"`
			} `json:"parameters"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Input.Media) != 3 || payload.Input.Media[0].Type != "first_frame" || payload.Input.Media[1].Type != "last_frame" || payload.Input.Media[2].Type != "driving_audio" || payload.Parameters.Resolution != "720P" || payload.Parameters.Duration != 10 {
			t.Fatalf("wrong Wan 2.7 request: %+v", payload)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"output":{"task_id":"remote"}}`)), Header: make(http.Header)}, nil
	})}}
	model := provider.Model{RemoteID: "wan2.7-i2v-2026-04-25", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}}
	_, err := client.GenerateVideo(context.Background(), provider.Connection{ID: "c", Name: "Test", Vendor: "bailian", BaseURL: "https://example.com/api/v1"}, model, provider.VideoRequest{Prompt: "motion", SourceImageURL: "https://example.com/first.png", LastFrameURL: "https://example.com/last.png", DrivingAudioURL: "https://example.com/voice.mp3", Resolution: "720P", Duration: 10}, provider.RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDiscoveryAndClientProtocolSelection(t *testing.T) {
	var paths []string
	client := Client{Credentials: credential("secret"), HTTP: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		if req.URL.Host != "custom.example.com" || req.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("wrong endpoint or credentials: %s", req.URL)
		}
		var body string
		switch req.URL.Path {
		case "/api/v1/models":
			body = `{"output":{"total":5,"models":[{"model":"qwen-plus","name":"Qwen Plus","capabilities":["Reasoning","TG"]},{"model":"qwen-image","capabilities":["IG"]},{"model":"wan2.7-i2v","capabilities":["VG"]},{"model":"wan2.7-t2v","capabilities":["VG"]},{"model":"other","capabilities":{"unknown":true}}]}}`
		case "/compatible-mode/v1/chat/completions":
			body = `{"choices":[{"message":{"content":"draft"}}]}`
		case "/api/v1/services/aigc/multimodal-generation/generation":
			var payload struct {
				Parameters struct {
					Size string `json:"size"`
				} `json:"parameters"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil || payload.Parameters.Size != "2688*1536" {
				t.Fatalf("wrong image protocol: %+v %v", payload, err)
			}
			body = `{"output":{"choices":[{"message":{"content":[{"image":"https://example.com/image.png"}]}}]}}`
		case "/api/v1/services/aigc/video-generation/video-synthesis":
			if req.Header.Get("X-DashScope-Async") != "enable" {
				t.Fatal("missing async header")
			}
			var payload struct {
				Input struct {
					Media []struct {
						Type string `json:"type"`
					} `json:"media"`
				} `json:"input"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil || len(payload.Input.Media) != 1 || payload.Input.Media[0].Type != "first_frame" {
				t.Fatalf("wrong video protocol: %+v %v", payload, err)
			}
			body = `{"output":{"task_status":"PENDING","task_id":"remote-1"}}`
		case "/api/v1/tasks/remote-1":
			body = `{"output":{"task_status":"SUCCEEDED","video_url":"https://example.com/video.mp4"}}`
		default:
			t.Fatalf("unexpected request %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	connection := provider.Connection{ID: "custom", Name: "Custom", Vendor: "bailian", BaseURL: "https://custom.example.com/"}
	models, err := client.Discover(context.Background(), connection, "secret")
	if err != nil || len(models) != 5 || len(models[0].Capabilities) != 1 || models[0].Capabilities[0] != provider.Story || len(models[1].Capabilities) != 1 || models[1].Capabilities[0] != provider.Image || len(models[2].Capabilities) != 1 || models[2].Capabilities[0] != provider.Video || len(models[3].Capabilities) != 0 || len(models[4].Capabilities) != 0 {
		t.Fatal(models, err)
	}
	model := func(id string, cap provider.Capability) provider.Model {
		return provider.Model{RemoteID: id, Enabled: true, Supported: true, Capabilities: []provider.Capability{cap}}
	}
	text, err := client.GenerateText(context.Background(), connection, model("qwen-plus", provider.Story), provider.StoryRequest{Prompt: "hello"}, provider.RequestOptions{})
	if err != nil || text.Document != "draft" {
		t.Fatal(text, err)
	}
	img, err := client.GenerateImage(context.Background(), connection, model("qwen-image-2.0", provider.Image), provider.ImageRequest{Prompt: "scene", AspectRatio: "16:9"}, provider.RequestOptions{})
	if err != nil || img.URL == "" {
		t.Fatal(img, err)
	}
	video := model("wan2.7-i2v", provider.Video)
	if _, err := client.GenerateText(context.Background(), connection, video, provider.StoryRequest{}, provider.RequestOptions{}); err != provider.ErrUnsupported {
		t.Fatal(err)
	}
	submitted, err := client.GenerateVideo(context.Background(), connection, video, provider.VideoRequest{Prompt: "motion", SourceImageURL: "https://example.com/first.png"}, provider.RequestOptions{})
	if err != nil || submitted.JobReference != "remote-1" {
		t.Fatal(submitted, err)
	}
	finished, done, err := client.PollVideo(context.Background(), connection, video, submitted.JobReference)
	if err != nil || !done || finished.URL == "" {
		t.Fatal(finished, done, err)
	}
	if len(paths) != 5 {
		t.Fatal(paths)
	}
	if _, err := client.GenerateImage(context.Background(), connection, model("unknown", provider.Image), provider.ImageRequest{}, provider.RequestOptions{}); err != provider.ErrUnsupported {
		t.Fatal(err)
	}
}

func TestConnectionUsesHTTPSBaseURL(t *testing.T) {
	for _, origin := range []string{"", "http://example.com", "https://user:pass@example.com", "https://example.com/?token=key"} {
		connection := provider.Connection{ID: "test", Name: "Test", Vendor: "bailian", BaseURL: origin}
		if _, err := baseURL(connection); err == nil {
			t.Fatalf("accepted invalid origin %q", origin)
		}
	}
	connection := provider.Connection{ID: "test", Name: "Test", Vendor: "bailian", BaseURL: "https://example.com/compatible-mode/v1/chat/completions"}
	if got, err := baseURL(connection); err != nil || got != connection.BaseURL {
		t.Fatalf("changed complete endpoint: %q %v", got, err)
	}
	client := Client{HTTP: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/v1/models" || req.URL.Query().Get("page_no") != "1" {
			t.Fatalf("wrong discovery endpoint: %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"output":{"total":0,"models":[]}}`)), Header: make(http.Header)}, nil
	})}}
	if _, err := client.Discover(context.Background(), connection, "test-key"); err != nil {
		t.Fatal(err)
	}
	if got, err := endpoint(connection, "/compatible-mode/v1/chat/completions"); err != nil || got != connection.BaseURL {
		t.Fatalf("complete chat endpoint changed: %q %v", got, err)
	}
	if got, err := endpoint(connection, "/api/v1/services/aigc/multimodal-generation/generation"); err != nil || got != "https://example.com/api/v1/services/aigc/multimodal-generation/generation" {
		t.Fatalf("wrong image endpoint: %q %v", got, err)
	}
}
