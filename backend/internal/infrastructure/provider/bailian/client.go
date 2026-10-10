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
	"slices"
	"strconv"
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

func endpoint(connection provider.Connection, path string) (string, error) {
	return connection.RequestURL(path)
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
	if video, ok := body.(protocol.WanxiangVideoRequest); ok {
		for _, media := range video.Input.Media {
			if strings.HasPrefix(media.URL, "oss://") {
				req.Header.Set("X-DashScope-OssResourceResolve", "enable")
				break
			}
		}
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

func (c Client) GenerateText(ctx context.Context, connection provider.Connection, model provider.Model, input provider.StoryRequest, options provider.RequestOptions) (provider.StoryResult, error) {
	if model.Require(provider.Story, provider.ChatProtocol) != nil {
		return provider.StoryResult{}, provider.ErrUnsupported
	}
	ctx, cancel := provider.WithTimeout(ctx, options)
	defer cancel()
	key, err := c.key(ctx, connection)
	if err != nil {
		return provider.StoryResult{}, err
	}
	req := protocol.QianwenChatRequest{Model: model.RemoteID}
	if input.SystemPrompt != "" {
		req.Messages = append(req.Messages, struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{Role: "system", Content: input.SystemPrompt})
	}
	req.Messages = append(req.Messages, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "user", Content: input.Prompt})
	var response protocol.QianwenChatResponse
	if err := c.request(ctx, connection, key, http.MethodPost, operationPath(model, provider.Story), req, &response, false); err != nil {
		return provider.StoryResult{}, err
	}
	if len(response.Choices) == 0 {
		return provider.StoryResult{}, errors.New("text response has no choices")
	}
	return provider.StoryResult{Document: response.Choices[0].Message.Content}, nil
}

func (c Client) GenerateImage(ctx context.Context, connection provider.Connection, model provider.Model, input provider.ImageRequest, options provider.RequestOptions) (provider.ImageResult, error) {
	if !model.Supports(provider.Image) {
		return provider.ImageResult{}, provider.ErrUnsupported
	}
	if err := model.Require(provider.Image, provider.ImageProtocol); err != nil {
		return provider.ImageResult{}, err
	}
	id := model.RemoteID
	binding := model.Operations[provider.Image]
	sizes := binding.ImageSizes
	if !slices.Contains(binding.Ratios, input.AspectRatio) {
		return provider.ImageResult{}, errors.New("unsupported image ratio")
	}
	ctx, cancel := provider.WithTimeout(ctx, options)
	defer cancel()
	key, err := c.key(ctx, connection)
	if err != nil {
		return provider.ImageResult{}, err
	}
	req := protocol.QianwenImageRequest{Model: id}
	req.Parameters.Size = sizes[input.AspectRatio]
	content := []protocol.ImageContent{}
	if len(input.References) > 0 {
		if len(input.References) > binding.MaxReferences {
			return provider.ImageResult{}, errors.New("reference_limit_exceeded")
		}
		for _, ref := range input.References {
			if ref.URL == "" {
				return provider.ImageResult{}, errors.New("missing reference image")
			}
			content = append(content, protocol.ImageContent{Image: ref.URL})
		}
	}
	content = append(content, protocol.ImageContent{Text: input.Prompt})
	req.Input.Messages = append(req.Input.Messages, protocol.ImageMessage{Role: "user", Content: content})
	var response protocol.QianwenImageResponse
	if err := c.request(ctx, connection, key, http.MethodPost, operationPath(model, provider.Image), req, &response, false); err != nil {
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
	capability, known := model.VideoCapability()
	if model.Require(provider.Video, provider.VideoProtocol) != nil || !known {
		return provider.VideoResult{}, provider.ErrUnsupported
	}
	referenceMode := capability.InputMode == "reference_images"
	if referenceMode {
		if input.SourceImageURL != "" || input.LastFrameURL != "" || input.DrivingAudioURL != "" {
			return provider.VideoResult{}, errors.New("reference video only accepts reference images")
		}
		for _, ref := range input.References {
			if !strings.HasPrefix(ref.URL, "https://") && !strings.HasPrefix(ref.URL, "http://") && !strings.HasPrefix(ref.URL, "data:image/") {
				return provider.VideoResult{}, errors.New("reference image must be a public URL or image data URI")
			}
		}
		if input.AspectRatio == "" {
			input.AspectRatio = "16:9"
		}
		if !slices.Contains(model.Operations[provider.Video].Ratios, input.AspectRatio) {
			return provider.VideoResult{}, errors.New("unsupported reference video ratio")
		}
	} else if len(input.References) > 0 {
		return provider.VideoResult{}, errors.New("first-frame model does not support reference images")
	} else if !strings.HasPrefix(input.SourceImageURL, "https://") && !strings.HasPrefix(input.SourceImageURL, "http://") && !strings.HasPrefix(input.SourceImageURL, "data:image/") {
		return provider.VideoResult{}, errors.New("source image must be a public URL or an image data URI")
	}
	if input.LastFrameURL != "" && !strings.HasPrefix(input.LastFrameURL, "https://") && !strings.HasPrefix(input.LastFrameURL, "http://") && !strings.HasPrefix(input.LastFrameURL, "data:image/") {
		return provider.VideoResult{}, errors.New("last frame must be a public URL or an image data URI")
	}
	if input.DrivingAudioURL != "" && !strings.HasPrefix(input.DrivingAudioURL, "https://") && !strings.HasPrefix(input.DrivingAudioURL, "http://") && !strings.HasPrefix(input.DrivingAudioURL, "oss://") {
		return provider.VideoResult{}, errors.New("driving audio must be a public URL or an OSS URL")
	}
	if input.DrivingAudioURL != "" {
		audioURL, parseErr := url.Parse(input.DrivingAudioURL)
		if parseErr != nil {
			return provider.VideoResult{}, parseErr
		}
		if expiry := audioURL.Query().Get("Expires"); expiry != "" {
			expiryUnix, parseErr := strconv.ParseInt(expiry, 10, 64)
			if parseErr != nil || expiryUnix <= time.Now().Unix()+300 {
				return provider.VideoResult{}, errors.New("driving audio URL has expired or expires too soon")
			}
		}
	}
	if input.Duration == 0 {
		input.Duration = capability.Durations[0]
	}
	if input.Resolution == "" {
		input.Resolution = capability.Resolutions[0]
	}
	usages := []string{"first_frame"}
	if referenceMode {
		usages = nil
		for range input.References {
			usages = append(usages, "reference_image")
		}
	}
	if input.LastFrameURL != "" {
		usages = append(usages, "last_frame")
	}
	if input.DrivingAudioURL != "" {
		usages = append(usages, "driving_audio")
	}
	if err := capability.ValidateInput(input.Duration, input.Resolution, usages); err != nil {
		return provider.VideoResult{}, err
	}
	ctx, cancel := provider.WithTimeout(ctx, options)
	defer cancel()
	key, err := c.key(ctx, connection)
	if err != nil {
		return provider.VideoResult{}, err
	}
	req := protocol.WanxiangVideoRequest{Model: model.RemoteID}
	req.Input.Prompt = input.Prompt
	if referenceMode {
		for _, ref := range input.References {
			req.Input.Media = append(req.Input.Media, struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			}{Type: "reference_image", URL: ref.URL})
		}
		req.Parameters.Ratio = input.AspectRatio
	} else {
		req.Input.Media = append(req.Input.Media, struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		}{Type: "first_frame", URL: input.SourceImageURL})
	}
	if input.LastFrameURL != "" {
		req.Input.Media = append(req.Input.Media, struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		}{Type: "last_frame", URL: input.LastFrameURL})
	}
	if input.DrivingAudioURL != "" {
		req.Input.Media = append(req.Input.Media, struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		}{Type: "driving_audio", URL: input.DrivingAudioURL})
	}
	req.Parameters.Resolution = input.Resolution
	req.Parameters.Duration = input.Duration
	var response protocol.WanxiangVideoResponse
	if err := c.request(ctx, connection, key, http.MethodPost, operationPath(model, provider.Video), req, &response, true); err != nil {
		return provider.VideoResult{}, err
	}
	if response.Output.TaskID == "" {
		return provider.VideoResult{}, fmt.Errorf("video submission has no task id: %s", response.Output.TaskStatus)
	}
	return provider.VideoResult{JobReference: response.Output.TaskID}, nil
}

func (c Client) PollVideo(ctx context.Context, connection provider.Connection, model provider.Model, id string) (provider.VideoResult, bool, error) {
	_, known := model.VideoCapability()
	if model.Require(provider.Video, provider.VideoProtocol) != nil || !known {
		return provider.VideoResult{}, false, provider.ErrUnsupported
	}
	key, err := c.key(ctx, connection)
	if err != nil {
		return provider.VideoResult{}, false, err
	}
	var response protocol.WanxiangVideoResponse
	if err := c.request(ctx, connection, key, http.MethodGet, provider.VideoTaskPathPrefix+url.PathEscape(id), nil, &response, false); err != nil {
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

func operationPath(m provider.Model, op provider.Capability) string {
	return m.Operations[op].RequestPath()
}
