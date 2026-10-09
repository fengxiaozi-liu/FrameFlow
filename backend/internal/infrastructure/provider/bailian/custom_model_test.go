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

func TestOpaqueModelIDsAcrossAllProtocols(t *testing.T) {
	for _, tt := range []struct {
		preset   string
		op       provider.Capability
		response string
	}{
		{"qwen-plus", provider.Story, `{"choices":[{"message":{"content":"ok"}}]}`},
		{"qwen-image-2.0-pro", provider.Image, `{"output":{"choices":[{"message":{"content":[{"image":"https://media/image"}]}}]}}`},
		{"wan2.7-i2v", provider.Video, `{"output":{"task_id":"job"}}`},
		{"qwen3-tts-flash", provider.Speech, `{"output":{"audio":{"url":"https://media/voice"}}}`},
	} {
		for _, mode := range []string{"", "custom", "default"} {
			t.Run(string(tt.op)+"/"+mode, func(t *testing.T) {
				model := provider.MigrateLegacyModel(provider.Model{RemoteID: tt.preset, Capabilities: []provider.Capability{tt.op}, Enabled: true, Supported: true})
				model.RemoteID = "custom/vendor-model-999"
				b := model.Operations[tt.op]
				b.Endpoint = "/custom/generate"
				b.EndpointMode = mode
				expectedPath := "/custom/generate"
				if mode == "default" {
					for _, p := range provider.Protocols() {
						if p.ID == b.Protocol {
							expectedPath = p.DefaultPath
						}
					}
				}
				model.Operations[tt.op] = b
				calls := 0
				c := Client{Credentials: credential("test"), HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
					calls++
					var body struct {
						Model string `json:"model"`
					}
					if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
						t.Fatal(e)
					}
					if body.Model != model.RemoteID || r.URL.Path != expectedPath {
						t.Fatal(body, r.URL)
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tt.response))}, nil
				})}}
				conn := provider.Connection{ID: "test", Name: "Custom provider", Vendor: "custom", BaseURL: "https://example.com/v1"}
				var err error
				switch tt.op {
				case provider.Story:
					_, err = c.GenerateText(context.Background(), conn, model, provider.StoryRequest{Prompt: "story"}, provider.RequestOptions{})
				case provider.Image:
					_, err = c.GenerateImage(context.Background(), conn, model, provider.ImageRequest{Prompt: "image", AspectRatio: "1:1"}, provider.RequestOptions{})
				case provider.Video:
					_, err = c.GenerateVideo(context.Background(), conn, model, provider.VideoRequest{SourceImageURL: "https://media/frame", Duration: 4, Resolution: "720P"}, provider.RequestOptions{})
				case provider.Speech:
					_, err = c.GenerateSpeech(context.Background(), conn, model, provider.SpeechRequest{Text: "hello", Voice: "Cherry", Language: "English"}, provider.RequestOptions{})
				}
				if err != nil || calls != 1 {
					t.Fatal(err, calls)
				}
			})
		}
	}
}
