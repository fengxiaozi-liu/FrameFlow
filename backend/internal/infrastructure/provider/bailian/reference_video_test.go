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
	model := provider.Model{RemoteID: "wan2.7-r2v", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Video}}
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
