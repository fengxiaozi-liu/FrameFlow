package bailian

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider/bailian/protocol"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReferenceVideoRequestAndPolling(t *testing.T) {
	refs := []provider.ImageReference{{URL: "data:image/png;base64,YQ==", EntityID: "scene"}, {URL: "https://example.com/person.png", EntityID: "person"}}
	calls := 0
	client := Client{Credentials: credential("secret"), HTTP: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		body := `{"output":{"task_status":"SUCCEEDED","video_url":"https://example.com/video.mp4"}}`
		if req.Method == http.MethodPost {
			var payload protocol.WanxiangVideoRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Input.Media) != 2 || payload.Parameters.Ratio != "9:16" || payload.Parameters.Duration != 6 || req.Header.Get("X-DashScope-Async") != "enable" {
				t.Fatalf("wrong payload: %+v", payload)
			}
			for i, ref := range refs {
				if payload.Input.Media[i].Type != "reference_image" || payload.Input.Media[i].URL != ref.URL {
					t.Fatalf("lost reference %d", i)
				}
			}
			body = `{"output":{"task_id":"r2v-job"}}`
		} else if req.URL.Path != "/api/v1/tasks/r2v-job" {
			t.Fatal(req.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	model := provider.MigrateLegacyModel(provider.Model{RemoteID: "wan2.7-r2v", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}})
	connection := provider.Connection{ID: "test", Name: "Test", Vendor: "bailian", BaseURL: "https://example.com"}
	input := provider.VideoRequest{Prompt: "motion", References: refs, Duration: 6, Resolution: "720P", AspectRatio: "9:16"}
	result, err := client.GenerateVideo(context.Background(), connection, model, input, provider.RequestOptions{})
	if err != nil || result.JobReference != "r2v-job" {
		t.Fatal(result, err)
	}
	result, done, err := client.PollVideo(context.Background(), connection, model, result.JobReference)
	if err != nil || !done || result.URL == "" {
		t.Fatal(result, done, err)
	}
	for _, count := range []int{0, 6} {
		input.References = make([]provider.ImageReference, count)
		for i := range input.References {
			input.References[i] = refs[0]
		}
		if _, err := client.GenerateVideo(context.Background(), connection, model, input, provider.RequestOptions{}); err == nil {
			t.Fatalf("accepted %d references", count)
		}
	}
	if calls != 2 {
		t.Fatalf("invalid input reached provider: %d calls", calls)
	}
}

func TestMultiModeModelUsesSelectedWireFormatAndCanPoll(t *testing.T) {
	model := provider.MigrateLegacyModel(provider.Model{RemoteID: "wan2.7-r2v", Enabled: true, Capabilities: []provider.Capability{provider.Video}})
	b := model.Operations[provider.Video]
	b.InputMode = ""
	b.InputModes = []string{"first_frame", "reference_images"}
	b.LastFrame = true
	model.Operations[provider.Video] = b
	model.RemoteID = "unknown-dual-mode"
	expectedMode := ""
	calls := 0
	client := Client{Credentials: credential("test-key"), HTTP: &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		body := `{"output":{"task_status":"SUCCEEDED","video_url":"https://example.com/video.mp4"}}`
		if req.Method == http.MethodPost {
			var payload protocol.WanxiangVideoRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Input.Media) != 2 {
				t.Fatal(payload)
			}
			if expectedMode == "first_frame" {
				if payload.Input.Media[0].Type != "first_frame" || payload.Input.Media[1].Type != "last_frame" {
					t.Fatal(payload)
				}
			} else if payload.Input.Media[0].Type != "reference_image" || payload.Input.Media[1].Type != "reference_image" {
				t.Fatal(payload)
			}
			body = `{"output":{"task_id":"dual-job"}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	conn := provider.Connection{ID: "c", Name: "c", Vendor: "bailian", BaseURL: "https://example.com"}
	for _, mode := range []string{"first_frame", "reference_images"} {
		expectedMode = mode
		input := provider.VideoRequest{InputMode: mode, Prompt: "move", AspectRatio: "16:9", Duration: 4, Resolution: "720P"}
		if mode == "first_frame" {
			input.SourceImageURL = "https://media/a.png"
			input.LastFrameURL = "https://media/b.png"
		} else {
			input.References = []provider.ImageReference{{URL: "https://media/a.png"}, {URL: "https://media/b.png"}}
		}
		result, err := client.GenerateVideo(context.Background(), conn, model, input, provider.RequestOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if _, done, err := client.PollVideo(context.Background(), conn, model, result.JobReference); err != nil || !done {
			t.Fatal(done, err)
		}
	}
	if calls != 4 {
		t.Fatal(calls)
	}
}
