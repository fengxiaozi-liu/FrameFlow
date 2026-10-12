package custom

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func TestPlainDemoRendersActualValuesAndValidatesResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "unknown-id" || body["input"] != "actual prompt" || body["temperature"] != 0.5 {
			t.Error(body)
		}
		_, _ = w.Write([]byte(`{"answer":"real result","extra":true}`))
	}))
	defer server.Close()
	b, err := (domain.OperationBinding{Protocol: domain.CustomProtocol, Custom: &domain.CustomInterface{Path: "/generate", AuthMode: "none"}, Contract: &domain.OperationContract{Execution: domain.SyncJSON, Request: domain.JSONContract{Example: `{"model":"demo","input":"demo","temperature":0.5}`, Bindings: map[string]string{"model": "/model", "prompt": "/input"}}, Response: domain.JSONContract{Example: `{"answer":"demo"}`, Bindings: map[string]string{"result": "/answer"}}}}).Compile(domain.Story)
	if err != nil {
		t.Fatal(err)
	}
	m := domain.Model{ID: "m", RemoteID: "unknown-id", Enabled: true, Capabilities: []domain.Capability{domain.Story}, Operations: map[domain.Capability]domain.OperationBinding{domain.Story: b}}
	result, err := (Client{HTTP: server.Client()}).GenerateText(context.Background(), connection(server), m, domain.StoryRequest{Prompt: "actual prompt"}, domain.RequestOptions{})
	if err != nil || result.Document != "real result" {
		t.Fatal(result, err)
	}
}

func TestVideoContractBindsNestedReferenceImageArray(t *testing.T) {
	model := fixture(domain.Video)
	binding := model.Operations[domain.Video]
	binding.Custom.AuthMode = "none"
	binding.Contract = &domain.OperationContract{
		Execution: domain.SyncJSON,
		Request: domain.JSONContract{
			Example:  `{"model":"demo","prompt":"demo","input":{"references":[]}}`,
			Bindings: map[string]string{"model": "/model", "prompt": "/prompt", "images": "/input/references"},
		},
		Response: domain.JSONContract{Example: `{"url":"https://media/demo.mp4"}`, Bindings: map[string]string{"result": "/url"}},
	}
	compiled, err := binding.Compile(domain.Video)
	if err != nil {
		t.Fatal(err)
	}
	model.Operations[domain.Video] = compiled
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input struct {
				References []string `json:"references"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		images := body.Input.References
		if len(images) != 2 || images[0] != "https://media/a.png" || images[1] != "https://media/b.png" {
			t.Errorf("reference array lost images or order: %v", images)
		}
		_, _ = w.Write([]byte(`{"url":"https://media/video.mp4"}`))
	}))
	defer server.Close()
	result, err := (Client{HTTP: server.Client()}).GenerateVideo(context.Background(), connection(server), model, domain.VideoRequest{
		InputMode: "reference_images", Prompt: "move", AspectRatio: "16:9", Duration: 4, Resolution: "720P",
		References: []domain.ImageReference{{URL: "https://media/a.png"}, {URL: "https://media/b.png"}},
	}, domain.RequestOptions{})
	if err != nil || result.URL != "https://media/video.mp4" {
		t.Fatal(result, err)
	}
}

func TestStreamFramingCompletionAndCancellationBoundaries(t *testing.T) {
	for _, mode := range []domain.ExecutionMode{domain.SSEStream, domain.NDJSONStream} {
		data := "{\"delta\":\"Hello \"}\n{\"delta\":\"world\"}\n[DONE]\n"
		if mode == domain.SSEStream {
			data = ": comment\n\ndata: {\"delta\":\"Hello \"}\n\ndata: {\"delta\":\"world\"}\n\ndata: [DONE]\n\n"
		}
		config := domain.CustomInterface{StreamMode: mode, StreamAppend: true, StreamDoneMarker: "[DONE]", ResultPointer: "/delta"}
		result, err := decodeStream(strings.NewReader(data), config)
		if err != nil {
			t.Fatal(err)
		}
		text, _ := extract(result, "/delta")
		if text != "Hello world" {
			t.Fatal(text)
		}
		if _, err := decodeStream(strings.NewReader(strings.ReplaceAll(data, "[DONE]", "")), config); err == nil {
			t.Fatal("truncated stream accepted")
		}
	}
}

func TestUnknownAcceptanceHasNoAutomaticRetry(t *testing.T) {
	c := Client{HTTP: &http.Client{Transport: contractFailureTransport{}}}
	_, err := c.request(context.Background(), domain.Connection{ID: "c", Name: "C", Vendor: "custom", BaseURL: "https://example.com"}, domain.CustomInterface{AuthMode: "none"}, "POST", "/generate", map[string]string{"prompt": "x"})
	if !errors.Is(err, domain.ErrRemoteAcceptanceUnknown) {
		t.Fatal(err)
	}
}

type contractFailureTransport struct{}

func (contractFailureTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("synthetic network failure")
}

func TestPlainDemoAsyncLifecycleUsesSavedSchema(t *testing.T) {
	state := "queued"
	badTaskID := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if badTaskID {
				_, _ = w.Write([]byte(`{"job":{"id":17}}`))
				return
			}
			_, _ = w.Write([]byte(`{"job":{"id":"remote-one"}}`))
			return
		}
		if r.URL.Path != "/jobs/remote-one" {
			t.Error("incorrect polling path")
		}
		if state == "queued" {
			_, _ = w.Write([]byte(`{"state":"queued"}`))
			return
		}
		if state == "failed" {
			_, _ = w.Write([]byte(`{"state":"failed"}`))
			return
		}
		_, _ = w.Write([]byte(`{"state":"done","output":{"url":"https://cdn.example.com/video.mp4"}}`))
	}))
	defer server.Close()
	m := fixture(domain.Video)
	b := m.Operations[domain.Video]
	b.Custom.AuthMode = "none"
	b.Custom.Polling = &domain.CustomPolling{Path: "/jobs/{task_id}", Pending: []string{"queued"}, Succeeded: []string{"done"}, Failed: []string{"failed"}}
	b.Contract = &domain.OperationContract{
		Execution:       domain.AsyncPolling,
		Request:         domain.JSONContract{Example: `{"model":"demo","prompt":"demo","images":[]}`, Bindings: map[string]string{"model": "/model", "prompt": "/prompt", "images": "/images"}},
		Response:        domain.JSONContract{Example: `{"job":{"id":"demo"}}`, Bindings: map[string]string{"task_id": "/job/id"}},
		PollingResponse: &domain.JSONContract{Example: `{"state":"done","output":{"url":"https://cdn.example.com/demo.mp4"}}`, Bindings: map[string]string{"status": "/state", "result": "/output/url"}},
	}
	compiled, err := b.Compile(domain.Video)
	if err != nil {
		t.Fatal(err)
	}
	m.Operations[domain.Video] = compiled
	c := Client{HTTP: server.Client()}
	ctx := context.Background()
	input := domain.VideoRequest{Prompt: "move", AspectRatio: "16:9", References: []domain.ImageReference{{URL: "https://cdn.example.com/frame.png"}}, Duration: 4, Resolution: "720P"}
	accepted, err := c.GenerateVideo(ctx, connection(server), m, input, domain.RequestOptions{})
	if err != nil || accepted.JobReference != "remote-one" {
		t.Fatal(accepted, err)
	}
	if _, done, err := c.PollVideo(ctx, connection(server), m, accepted.JobReference); done || err != nil {
		t.Fatal(done, err)
	}
	state = "done"
	if result, done, err := c.PollVideo(ctx, connection(server), m, accepted.JobReference); !done || err != nil || result.URL == "" {
		t.Fatal(result, done, err)
	}
	state = "failed"
	if _, _, err := c.PollVideo(ctx, connection(server), m, accepted.JobReference); err == nil {
		t.Fatal("remote failure accepted")
	}
	badTaskID = true
	if _, err := c.GenerateVideo(ctx, connection(server), m, input, domain.RequestOptions{}); !errors.Is(err, domain.ErrRemoteAcceptanceUnknown) {
		t.Fatal("invalid acceptance must prevent blind retry", err)
	}
}
