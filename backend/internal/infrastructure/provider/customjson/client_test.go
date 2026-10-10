package customjson

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

type credential string

func (k credential) Get(context.Context, string) (string, error) { return string(k), nil }
func fixture(op domain.Capability) domain.Model {
	template := `{"engine":"{{model}}","question":"{{prompt}}"}`
	if op == domain.Speech {
		template = `{"engine":"{{model}}","say":"{{text}}","speaker":"{{voice}}"}`
	}
	if op == domain.Video {
		template = `{"engine":"{{model}}","question":"{{prompt}}","frames":"{{images}}","seconds":"{{duration}}","quality":"{{resolution}}"}`
	}
	b := domain.OperationBinding{Protocol: domain.CustomProtocol, Custom: &domain.CustomInterface{Path: "/generate", BodyTemplate: template, AuthMode: "bearer", Mode: "sync", ResultPointer: "/payload/results/0/value"}, Ratios: []string{"16:9"}, ImageSizes: map[string]string{"16:9": "1024*576"}, InputMode: "reference_images", MaxReferences: 0, MaxImageBytes: 1024, Durations: []int{4}, Resolutions: []string{"720P"}, MaxTextLength: 100, Voices: []domain.Voice{{ID: "v", Languages: []string{"Chinese"}}}}
	if op == domain.Video {
		b.MaxReferences = 2
	}
	return domain.Model{ID: "c:m", RemoteID: "unknown-model-987", Enabled: true, Capabilities: []domain.Capability{op}, Operations: map[domain.Capability]domain.OperationBinding{op: b}}
}
func connection(server *httptest.Server) domain.Connection {
	return domain.Connection{ID: "test", Name: "test", Vendor: "custom", BaseURL: server.URL}
}
func TestIndependentFormatsAcrossFourOperations(t *testing.T) {
	for _, op := range []domain.Capability{domain.Story, domain.Image, domain.Video, domain.Speech} {
		t.Run(string(op), func(t *testing.T) {
			calls := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/generate" || r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("X-DashScope-Async") != "" {
					t.Errorf("incorrect request: %s %s", r.Method, r.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["engine"] != "unknown-model-987" || body["model"] != nil {
					t.Error("builtin format leaked", body)
				}
				if op == domain.Video {
					if body["seconds"] != float64(4) || len(body["frames"].([]any)) != 1 {
						t.Error("lost typed variables", body)
					}
				}
				if op == domain.Story && body["question"] != "hello \"world\"\n{{untrusted}}" {
					t.Error("prompt corrupted", body)
				}
				value := "https://media.example/output"
				if op == domain.Story {
					value = "written story"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"payload": map[string]any{"results": []any{map[string]string{"value": value}}}})
			}))
			defer server.Close()
			c := Client{HTTP: server.Client(), Credentials: credential("test-key")}
			m := fixture(op)
			ctx := context.Background()
			var err error
			switch op {
			case domain.Story:
				var result domain.StoryResult
				result, err = c.GenerateText(ctx, connection(server), m, domain.StoryRequest{Prompt: "hello \"world\"\n{{untrusted}}"}, domain.RequestOptions{})
				if result.Document != "written story" {
					t.Fatal(result)
				}
			case domain.Image:
				_, err = c.GenerateImage(ctx, connection(server), m, domain.ImageRequest{Prompt: "paint", AspectRatio: "16:9"}, domain.RequestOptions{})
			case domain.Video:
				_, err = c.GenerateVideo(ctx, connection(server), m, domain.VideoRequest{Prompt: "move", AspectRatio: "16:9", References: []domain.ImageReference{{URL: "https://media.example/frame"}}, Duration: 4, Resolution: "720P"}, domain.RequestOptions{})
			case domain.Speech:
				_, err = c.GenerateSpeech(ctx, connection(server), m, domain.SpeechRequest{Text: "你好", Voice: "v", Language: "Chinese"}, domain.RequestOptions{})
			}
			if err != nil || calls != 1 {
				t.Fatal(err, calls)
			}
		})
	}
}
func TestAsyncLifecycleAndAuthentication(t *testing.T) {
	m := fixture(domain.Video)
	config := m.Operations[domain.Video].Custom
	config.Mode = "async"
	config.AuthMode = "header"
	config.AuthHeader = "X-Credential"
	config.AuthPrefix = "Token "
	config.Polling = &domain.CustomPolling{Path: "/jobs/{task_id}", TaskIDPointer: "/job", StatusPointer: "/state", Pending: []string{"waiting"}, Succeeded: []string{"finished"}, Failed: []string{"failed", "cancelled"}}
	state := "waiting"
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Credential") != "Token secret" || r.Header.Get("Authorization") != "" {
			t.Error("incorrect auth")
		}
		if r.Method == "POST" {
			posts++
			fmt.Fprint(w, `{"job":"a/b ?"}`)
			return
		}
		if r.URL.EscapedPath() != "/jobs/a%2Fb%20%3F" {
			t.Errorf("unsafe job path %s", r.URL.EscapedPath())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"state": state, "payload": map[string]any{"results": []any{map[string]string{"value": "https://media.example/video"}}}})
	}))
	defer server.Close()
	c := Client{HTTP: server.Client(), Credentials: credential("secret")}
	ctx := context.Background()
	accepted, err := c.GenerateVideo(ctx, connection(server), m, domain.VideoRequest{Prompt: "move", AspectRatio: "16:9", References: []domain.ImageReference{{URL: "https://media.example/frame"}}, Duration: 4, Resolution: "720P"}, domain.RequestOptions{})
	if err != nil || accepted.JobReference != "a/b ?" {
		t.Fatal(accepted, err)
	}
	for _, s := range []string{"waiting", "finished", "failed", "cancelled", "unmapped"} {
		state = s
		result, done, err := c.PollVideo(ctx, connection(server), m, accepted.JobReference)
		if s == "waiting" && (err != nil || done) {
			t.Fatal(result, done, err)
		}
		if s == "finished" && (err != nil || !done || result.URL == "") {
			t.Fatal(result, done, err)
		}
		if (s == "failed" || s == "cancelled" || s == "unmapped") && err == nil {
			t.Fatal("failure swallowed")
		}
	}
	if posts != 1 {
		t.Fatal("polling resubmitted generation")
	}
}
func TestFailuresAreBoundedAndDoNotLeakBodies(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		body string
	}{
		{"unauthorized", 401, `{"secret":"private-key"}`},
		{"rate limited", 429, `{"secret":"private-key"}`},
		{"redirect", 302, `private-key`},
		{"not json", 200, `private-key`},
		{"missing field", 200, `{"other":"private-key"}`},
		{"wrong type", 200, `{"payload":{"results":[{"value":123}]}}`},
		{"too large", 200, strings.Repeat("x", (4<<20)+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "https://other.example")
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c := Client{HTTP: server.Client()}
			m := fixture(domain.Story)
			m.Operations[domain.Story].Custom.AuthMode = "none"
			_, err := c.GenerateText(context.Background(), connection(server), m, domain.StoryRequest{Prompt: "x"}, domain.RequestOptions{})
			if err == nil || strings.Contains(err.Error(), "private-key") || calls != 1 {
				t.Fatal(err, calls)
			}
		})
	}
}
func TestCancellationAndValidationBeforeNetwork(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(250 * time.Millisecond):
		}
	}))
	defer server.Close()
	c := Client{HTTP: server.Client()}
	m := fixture(domain.Story)
	m.Operations[domain.Story].Custom.AuthMode = "none"
	_, err := c.GenerateText(context.Background(), connection(server), m, domain.StoryRequest{Prompt: "x"}, domain.RequestOptions{Timeout: 20 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.GenerateText(cancelled, connection(server), m, domain.StoryRequest{Prompt: "x"}, domain.RequestOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	m.Operations[domain.Story].Custom.BodyTemplate = `{"wrong":"{{api_key}}"}`
	_, err = c.GenerateText(context.Background(), connection(server), m, domain.StoryRequest{}, domain.RequestOptions{})
	if err == nil {
		t.Fatal("invalid template executed")
	}
}
func TestJSONPointersAndMediaValidation(t *testing.T) {
	value := map[string]any{"a/b": map[string]any{"~key": []any{"ok"}}}
	text, err := extract(value, "/a~1b/~0key/0")
	if err != nil || text != "ok" {
		t.Fatal(text, err)
	}
	for _, pointer := range []string{"/a~1b/~0key/-1", "/a~1b/~0key/01", "/missing"} {
		if _, err := extract(value, pointer); err == nil {
			t.Fatal(pointer)
		}
	}
	for _, url := range []string{"file:///data/secret", "data:image/png;base64,xxx", "https://user:secret@host/a"} {
		if _, err := mediaURL(url); err == nil {
			t.Fatal(url)
		}
	}
}
