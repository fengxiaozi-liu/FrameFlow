package bailian

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider/bailian/protocol"
)

type Client struct {
	Credentials provider.CredentialReader
	HTTP        *http.Client
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func baseURL(connection provider.Connection) (string, error) {
	if err := connection.Validate(); err != nil {
		return "", err
	}
	return strings.TrimSpace(connection.BaseURL), nil
}

// endpoint keeps a complete configured URL intact when it already targets the
// operation; other Bailian operations use the same workspace host.
func endpoint(connection provider.Connection, path string) (string, error) {
	base, err := baseURL(connection)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	operation, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	if u.Path == operation.Path && operation.RawQuery == "" {
		return base, nil
	}
	u.Path = operation.Path
	u.RawPath = ""
	u.RawQuery = operation.RawQuery
	return u.String(), nil
}

func (c Client) request(ctx context.Context, connection provider.Connection, key, method, path string, body any, out any, async bool) error {
	target, err := endpoint(connection, path)
	if err != nil {
		return err
	}
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	if async {
		req.Header.Set("X-DashScope-Async", "enable")
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var failure struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&failure)
		return fmt.Errorf("bailian %s (%d): %s", failure.Code, resp.StatusCode, failure.Message)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
}

func (c Client) key(ctx context.Context, connection provider.Connection) (string, error) {
	if c.Credentials == nil {
		return "", errors.New("credential reader unavailable")
	}
	return c.Credentials.Get(ctx, "connection:"+connection.ID)
}

func (c Client) Discover(ctx context.Context, connection provider.Connection, key string) ([]provider.DiscoveredModel, error) {
	items := make([]provider.DiscoveredModel, 0)
	for page := 1; page <= 100; page++ {
		var response struct {
			Output struct {
				Total  int `json:"total"`
				Models []struct {
					Model        string          `json:"model"`
					Name         string          `json:"name"`
					Capabilities json.RawMessage `json:"capabilities"`
				} `json:"models"`
			} `json:"output"`
		}
		path := fmt.Sprintf("/api/v1/models?page_no=%d&page_size=100", page)
		if err := c.request(ctx, connection, key, http.MethodGet, path, nil, &response, false); err != nil {
			return nil, err
		}
		for _, m := range response.Output.Models {
			if m.Model != "" {
				entry := provider.DiscoveredModel{ID: m.Model, Name: m.Name}
				var capabilities []provider.Capability
				if json.Unmarshal(m.Capabilities, &capabilities) == nil {
					for _, capability := range capabilities {
						switch {
						case (capability == "TG" || capability == provider.Story) && chatProtocol(m.Model):
							entry.Capabilities = append(entry.Capabilities, provider.Story)
						case (capability == "IG" || capability == provider.Image) && imageProtocol(m.Model):
							entry.Capabilities = append(entry.Capabilities, provider.Image)
						case (capability == "VG" || capability == provider.Video) && strings.HasPrefix(m.Model, "wan2.7-i2v"):
							entry.Capabilities = append(entry.Capabilities, provider.Video)
						}
					}
				}
				items = append(items, entry)
			}
		}
		if len(response.Output.Models) == 0 || len(items) >= response.Output.Total {
			return items, nil
		}
	}
	return nil, errors.New("model list exceeds pagination limit")
}

func (c Client) GenerateText(ctx context.Context, connection provider.Connection, model provider.Model, input provider.StoryRequest, options provider.RequestOptions) (provider.StoryResult, error) {
	if !model.Supports(provider.Story) || !chatProtocol(model.RemoteID) {
		return provider.StoryResult{}, provider.ErrUnsupported
	}
	ctx, cancel := provider.WithTimeout(ctx, options)
	defer cancel()
	key, err := c.key(ctx, connection)
	if err != nil {
		return provider.StoryResult{}, err
	}
	req := protocol.QianwenChatRequest{Model: model.RemoteID}
	req.Messages = append(req.Messages, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "user", Content: input.Prompt})
	var response protocol.QianwenChatResponse
	if err := c.request(ctx, connection, key, http.MethodPost, "/compatible-mode/v1/chat/completions", req, &response, false); err != nil {
		return provider.StoryResult{}, err
	}
	if len(response.Choices) == 0 {
		return provider.StoryResult{}, errors.New("text response has no choices")
	}
	return provider.StoryResult{Document: response.Choices[0].Message.Content}, nil
}

func chatProtocol(id string) bool {
	return strings.HasPrefix(id, "qwen-") && !strings.HasPrefix(id, "qwen-image") && !strings.Contains(id, "audio") && !strings.Contains(id, "vl") && !strings.Contains(id, "omni") ||
		strings.HasPrefix(id, "deepseek-") || strings.HasPrefix(id, "kimi-") || strings.HasPrefix(id, "glm-")
}

func imageProtocol(id string) bool {
	return strings.HasPrefix(id, "qwen-image-2.0") || strings.HasPrefix(id, "qwen-image-max") || strings.HasPrefix(id, "qwen-image-plus") || id == "qwen-image"
}

func (c Client) GenerateImage(ctx context.Context, connection provider.Connection, model provider.Model, input provider.ImageRequest, options provider.RequestOptions) (provider.ImageResult, error) {
	if !model.Supports(provider.Image) {
		return provider.ImageResult{}, provider.ErrUnsupported
	}
	id := model.RemoteID
	var sizes map[string]string
	switch {
	case strings.HasPrefix(id, "qwen-image-2.0"):
		sizes = map[string]string{"16:9": "2688*1536", "9:16": "1536*2688", "1:1": "2048*2048"}
	case strings.HasPrefix(id, "qwen-image-max"), strings.HasPrefix(id, "qwen-image-plus"), id == "qwen-image":
		sizes = map[string]string{"16:9": "1664*928", "9:16": "928*1664", "1:1": "1328*1328"}
	default:
		return provider.ImageResult{}, provider.ErrUnsupported
	}
	ctx, cancel := provider.WithTimeout(ctx, options)
	defer cancel()
	key, err := c.key(ctx, connection)
	if err != nil {
		return provider.ImageResult{}, err
	}
	req := protocol.QianwenImageRequest{Model: id}
	req.Parameters.Size = sizes[input.AspectRatio]
	req.Input.Messages = append(req.Input.Messages, struct {
		Role    string `json:"role"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}{Role: "user", Content: []struct {
		Text string `json:"text"`
	}{{Text: input.Prompt}}})
	var response protocol.QianwenImageResponse
	if err := c.request(ctx, connection, key, http.MethodPost, "/api/v1/services/aigc/multimodal-generation/generation", req, &response, false); err != nil {
		return provider.ImageResult{}, err
	}
	for _, choice := range response.Output.Choices {
		for _, content := range choice.Message.Content {
			if content.Image != "" {
				return provider.ImageResult{URL: content.Image}, nil
			}
		}
	}
	return provider.ImageResult{}, errors.New("image response has no image url")
}

func (c Client) GenerateVideo(ctx context.Context, connection provider.Connection, model provider.Model, input provider.VideoRequest, options provider.RequestOptions) (provider.VideoResult, error) {
	if !model.Supports(provider.Video) || !strings.HasPrefix(model.RemoteID, "wan2.7-i2v") {
		return provider.VideoResult{}, provider.ErrUnsupported
	}
	if !strings.HasPrefix(input.SourceImageURL, "https://") && !strings.HasPrefix(input.SourceImageURL, "http://") && !strings.HasPrefix(input.SourceImageURL, "data:image/") {
		return provider.VideoResult{}, errors.New("source image must be a public URL or an image data URI")
	}
	ctx, cancel := provider.WithTimeout(ctx, options)
	defer cancel()
	key, err := c.key(ctx, connection)
	if err != nil {
		return provider.VideoResult{}, err
	}
	req := protocol.WanxiangVideoRequest{Model: model.RemoteID}
	req.Input.Prompt = input.Prompt
	req.Input.Media = append(req.Input.Media, struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	}{Type: "first_frame", URL: input.SourceImageURL})
	var response protocol.WanxiangVideoResponse
	if err := c.request(ctx, connection, key, http.MethodPost, "/api/v1/services/aigc/video-generation/video-synthesis", req, &response, true); err != nil {
		return provider.VideoResult{}, err
	}
	if response.Output.TaskID == "" {
		return provider.VideoResult{}, fmt.Errorf("video submission has no task id: %s", response.Output.TaskStatus)
	}
	return provider.VideoResult{JobReference: response.Output.TaskID}, nil
}

func (c Client) PollVideo(ctx context.Context, connection provider.Connection, model provider.Model, id string) (provider.VideoResult, bool, error) {
	if !model.Supports(provider.Video) || !strings.HasPrefix(model.RemoteID, "wan2.7-i2v") {
		return provider.VideoResult{}, false, provider.ErrUnsupported
	}
	key, err := c.key(ctx, connection)
	if err != nil {
		return provider.VideoResult{}, false, err
	}
	var response protocol.WanxiangVideoResponse
	if err := c.request(ctx, connection, key, http.MethodGet, "/api/v1/tasks/"+url.PathEscape(id), nil, &response, false); err != nil {
		return provider.VideoResult{}, false, err
	}
	switch response.Output.TaskStatus {
	case "SUCCEEDED":
		if response.Output.VideoURL == "" {
			return provider.VideoResult{}, false, errors.New("video result has no url")
		}
		return provider.VideoResult{JobReference: id, URL: response.Output.VideoURL}, true, nil
	case "PENDING", "RUNNING":
		return provider.VideoResult{JobReference: id}, false, nil
	default:
		return provider.VideoResult{}, false, fmt.Errorf("remote video task %s: %s", id, response.Output.TaskStatus)
	}
}
