package customjson

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func (c Client) submit(ctx context.Context, connection domain.Connection, model domain.Model, op domain.Capability, variables map[string]any) (any, error) {
	if err := model.Require(op, domain.CustomProtocol); err != nil {
		return nil, err
	}
	config := model.Operations[op].Custom
	variables["model"] = model.RemoteID
	body, err := render(config.BodyTemplate, variables)
	if err != nil {
		return nil, err
	}
	return c.request(ctx, connection, *config, http.MethodPost, config.Path, body)
}

func (c Client) GenerateText(ctx context.Context, connection domain.Connection, model domain.Model, input domain.StoryRequest, options domain.RequestOptions) (domain.StoryResult, error) {
	ctx, cancel := domain.WithTimeout(ctx, options)
	defer cancel()
	messages := []map[string]string{}
	if input.SystemPrompt != "" {
		messages = append(messages, map[string]string{"role": "system", "content": input.SystemPrompt})
	}
	messages = append(messages, map[string]string{"role": "user", "content": input.Prompt})
	result, err := c.submit(ctx, connection, model, domain.Story, map[string]any{"prompt": input.Prompt, "system_prompt": input.SystemPrompt, "messages": messages})
	if err != nil {
		return domain.StoryResult{}, err
	}
	text, err := extract(result, model.Operations[domain.Story].Custom.ResultPointer)
	return domain.StoryResult{Document: text}, err
}
func referenceURLs(refs []domain.ImageReference, max int) ([]string, error) {
	if len(refs) > max {
		return nil, errors.New("参考图片数量超出配置上限")
	}
	urls := []string{}
	for _, ref := range refs {
		if !validImage(ref.URL) {
			return nil, errors.New("参考图片必须是 HTTP(S) 地址或图片 Data URI")
		}
		urls = append(urls, ref.URL)
	}
	return urls, nil
}
func (c Client) GenerateImage(ctx context.Context, connection domain.Connection, model domain.Model, input domain.ImageRequest, options domain.RequestOptions) (domain.ImageResult, error) {
	if err := model.Require(domain.Image, domain.CustomProtocol); err != nil {
		return domain.ImageResult{}, err
	}
	b := model.Operations[domain.Image]
	if !slices.Contains(b.Ratios, input.AspectRatio) {
		return domain.ImageResult{}, errors.New("图片比例不受支持")
	}
	images, err := referenceURLs(input.References, b.MaxReferences)
	if err != nil {
		return domain.ImageResult{}, err
	}
	ctx, cancel := domain.WithTimeout(ctx, options)
	defer cancel()
	result, err := c.submit(ctx, connection, model, domain.Image, map[string]any{"prompt": input.Prompt, "images": images, "ratio": input.AspectRatio, "size": b.ImageSizes[input.AspectRatio]})
	if err != nil {
		return domain.ImageResult{}, err
	}
	value, err := extract(result, b.Custom.ResultPointer)
	if err != nil {
		return domain.ImageResult{}, err
	}
	value, err = mediaURL(value)
	return domain.ImageResult{URL: value}, err
}
func (c Client) GenerateSpeech(ctx context.Context, connection domain.Connection, model domain.Model, input domain.SpeechRequest, options domain.RequestOptions) (domain.SpeechResult, error) {
	if err := model.Require(domain.Speech, domain.CustomProtocol); err != nil {
		return domain.SpeechResult{}, err
	}
	b := model.Operations[domain.Speech]
	voiceOK := false
	for _, voice := range b.Voices {
		if voice.ID == input.Voice && slices.Contains(voice.Languages, input.Language) {
			voiceOK = true
		}
	}
	if !voiceOK || utf8.RuneCountInString(input.Text) > b.MaxTextLength || strings.TrimSpace(input.Text) == "" {
		return domain.SpeechResult{}, errors.New("文字长度、音色或语言不符合模型配置")
	}
	ctx, cancel := domain.WithTimeout(ctx, options)
	defer cancel()
	result, err := c.submit(ctx, connection, model, domain.Speech, map[string]any{"text": input.Text, "voice": input.Voice, "language": input.Language, "instructions": input.Instructions})
	if err != nil {
		return domain.SpeechResult{}, err
	}
	value, err := extract(result, b.Custom.ResultPointer)
	if err != nil {
		return domain.SpeechResult{}, err
	}
	value, err = mediaURL(value)
	return domain.SpeechResult{URL: value}, err
}
func (c Client) Voices(model domain.Model) []domain.Voice {
	return model.Operations[domain.Speech].Voices
}

func (c Client) GenerateVideo(ctx context.Context, connection domain.Connection, model domain.Model, input domain.VideoRequest, options domain.RequestOptions) (domain.VideoResult, error) {
	if err := model.Require(domain.Video, domain.CustomProtocol); err != nil {
		return domain.VideoResult{}, err
	}
	b := model.Operations[domain.Video]
	capability, _ := model.VideoCapability()
	images, err := referenceURLs(input.References, b.MaxReferences)
	if err != nil {
		return domain.VideoResult{}, err
	}
	usages := []string{}
	if b.InputMode == "reference_images" {
		if input.SourceImageURL != "" || len(images) == 0 {
			return domain.VideoResult{}, errors.New("多图参考模式需要参考图片，不能混用首帧")
		}
		for range images {
			usages = append(usages, "reference_image")
		}
	} else {
		if len(images) > 0 || !validImage(input.SourceImageURL) {
			return domain.VideoResult{}, errors.New("首帧模式需要一张有效图片")
		}
		usages = append(usages, "first_frame")
	}
	if input.LastFrameURL != "" {
		if !validImage(input.LastFrameURL) {
			return domain.VideoResult{}, errors.New("尾帧图片无效")
		}
		usages = append(usages, "last_frame")
	}
	if input.DrivingAudioURL != "" {
		return domain.VideoResult{}, errors.New("自定义接口暂不支持驱动音频")
	}
	if !slices.Contains(b.Ratios, input.AspectRatio) {
		return domain.VideoResult{}, errors.New("视频比例不受支持")
	}
	if err := capability.ValidateInput(input.Duration, input.Resolution, usages); err != nil {
		return domain.VideoResult{}, err
	}
	ctx, cancel := domain.WithTimeout(ctx, options)
	defer cancel()
	result, err := c.submit(ctx, connection, model, domain.Video, map[string]any{"prompt": input.Prompt, "images": images, "first_frame": input.SourceImageURL, "last_frame": input.LastFrameURL, "duration": input.Duration, "resolution": input.Resolution, "ratio": input.AspectRatio})
	if err != nil {
		return domain.VideoResult{}, err
	}
	if b.Custom.Mode == "async" {
		id, err := extract(result, b.Custom.Polling.TaskIDPointer)
		if err == nil {
			err = validateTaskID(id)
		}
		return domain.VideoResult{JobReference: id}, err
	}
	value, err := extract(result, b.Custom.ResultPointer)
	if err != nil {
		return domain.VideoResult{}, err
	}
	value, err = mediaURL(value)
	return domain.VideoResult{URL: value}, err
}
func validateTaskID(id string) error {
	if id == "" || len(id) > 512 || strings.ContainsAny(id, "\r\n") || id == "." || id == ".." {
		return errors.New("接口返回的任务 ID 无效")
	}
	return nil
}
func (c Client) PollVideo(ctx context.Context, connection domain.Connection, model domain.Model, id string) (domain.VideoResult, bool, error) {
	if err := model.Require(domain.Video, domain.CustomProtocol); err != nil {
		return domain.VideoResult{}, false, err
	}
	config := model.Operations[domain.Video].Custom
	if config.Mode != "async" {
		return domain.VideoResult{}, false, errors.New("此接口未配置异步查询")
	}
	if err := validateTaskID(id); err != nil {
		return domain.VideoResult{}, false, err
	}
	p := config.Polling
	result, err := c.request(ctx, connection, *config, http.MethodGet, strings.Replace(p.Path, "{task_id}", url.PathEscape(id), 1), nil)
	if err != nil {
		return domain.VideoResult{}, false, err
	}
	state, err := extract(result, p.StatusPointer)
	if err != nil {
		return domain.VideoResult{}, false, err
	}
	if slices.Contains(p.Pending, state) {
		return domain.VideoResult{JobReference: id}, false, nil
	}
	if slices.Contains(p.Failed, state) {
		return domain.VideoResult{}, false, errors.New("远端自定义视频任务失败或已取消")
	}
	if !slices.Contains(p.Succeeded, state) {
		return domain.VideoResult{}, false, errors.New("远端任务状态未配置映射，请检查状态字段与状态值")
	}
	value, err := extract(result, config.ResultPointer)
	if err != nil {
		return domain.VideoResult{}, false, err
	}
	value, err = mediaURL(value)
	return domain.VideoResult{JobReference: id, URL: value}, err == nil, err
}
