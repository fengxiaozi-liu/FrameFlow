package bailian

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMultiImageAndSpeechWireRequests(t *testing.T) {
	calls := 0
	c := Client{Credentials: credential("test"), HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		input := body["input"].(map[string]any)
		out := `{"output":{"audio":{"url":"https://example.com/s.wav"}}}`
		if calls == 1 {
			messages := input["messages"].([]any)
			content := messages[0].(map[string]any)["content"].([]any)
			if len(content) != 3 || content[0].(map[string]any)["image"] != "https://example.com/a.png" || content[1].(map[string]any)["image"] != "https://example.com/b.png" {
				t.Fatal(content)
			}
			out = `{"output":{"choices":[{"message":{"content":[{"image":"https://example.com/out.png"}]}}]}}`
		} else if input["voice"] != "Ethan" || input["text"] != "Hello" {
			t.Fatal(input)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(out)), Header: http.Header{}}, nil
	})}}
	conn := provider.Connection{ID: "c", Name: "test", Vendor: "bailian", BaseURL: "https://example.com/compatible-mode/v1/chat/completions"}
	_, err := c.GenerateImage(context.Background(), conn, provider.Model{RemoteID: "qwen-image-2.0-pro", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Image}}, provider.ImageRequest{Prompt: "first person and second place", AspectRatio: "1:1", References: []provider.ImageReference{{URL: "https://example.com/a.png"}, {URL: "https://example.com/b.png"}}}, provider.RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m := provider.Model{RemoteID: "qwen3-tts-flash", Enabled: true, Supported: true, Capabilities: []provider.Capability{provider.Speech}}
	if _, err = c.GenerateSpeech(context.Background(), conn, m, provider.SpeechRequest{Text: "Hello", Voice: "Ethan", Language: "English"}, provider.RequestOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err = c.GenerateSpeech(context.Background(), conn, m, provider.SpeechRequest{Text: "Hello", Voice: "unknown", Language: "English"}, provider.RequestOptions{}); err == nil {
		t.Fatal("unsupported voice was sent")
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}
func TestCompatibleDiscovery(t *testing.T) {
	c := Client{HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/compatible-mode/v1/models" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"qwen3-tts-flash"},{"id":"qwen-image-2.0-pro"}]}`))}, nil
	})}}
	models, err := c.Discover(context.Background(), provider.Connection{ID: "c", Name: "test", Vendor: "bailian", BaseURL: "https://example.com/compatible-mode/v1/chat/completions"}, "test")
	if err != nil || len(models) != 2 || models[0].Capabilities[0] != provider.Speech {
		t.Fatal(models, err)
	}
}
