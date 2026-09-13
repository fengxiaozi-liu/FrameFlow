package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

type Mock struct{ Fail bool }
type StoryMock struct{ Mock }
type ImageMock struct{ Mock }
type VideoMock struct{ Mock }

func (m Mock) check(ctx context.Context, model string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if m.Fail || strings.HasPrefix(model, "mock-fail") {
		return errors.New("mock provider failure")
	}
	return nil
}
func (m Mock) GenerateStory(ctx context.Context, r provider.StoryRequest, o provider.RequestOptions) (provider.StoryResult, error) {
	ctx, cancel := provider.WithTimeout(ctx, o)
	defer cancel()
	if err := m.check(ctx, r.Model); err != nil {
		return provider.StoryResult{}, err
	}
	return provider.StoryResult{Document: "【内容概览】\n" + r.Prompt + "\n【分镜脚本】\n分镜 1：开场"}, nil
}
func (m Mock) GenerateImage(ctx context.Context, r provider.ImageRequest, o provider.RequestOptions) (provider.ImageResult, error) {
	ctx, cancel := provider.WithTimeout(ctx, o)
	defer cancel()
	if err := m.check(ctx, r.Model); err != nil {
		return provider.ImageResult{}, err
	}
	return provider.ImageResult{URL: "mock://image/preview.png"}, nil
}
func (m Mock) GenerateVideo(ctx context.Context, r provider.VideoRequest, o provider.RequestOptions) (provider.VideoResult, error) {
	ctx, cancel := provider.WithTimeout(ctx, o)
	defer cancel()
	if err := m.check(ctx, r.Model); err != nil {
		return provider.VideoResult{}, err
	}
	return provider.VideoResult{JobReference: "mock-video-job"}, nil
}
func (m StoryMock) Generate(ctx context.Context, r provider.StoryRequest, o provider.RequestOptions) (provider.StoryResult, error) {
	return m.GenerateStory(ctx, r, o)
}
func (m ImageMock) Generate(ctx context.Context, r provider.ImageRequest, o provider.RequestOptions) (provider.ImageResult, error) {
	return m.GenerateImage(ctx, r, o)
}
func (m VideoMock) Generate(ctx context.Context, r provider.VideoRequest, o provider.RequestOptions) (provider.VideoResult, error) {
	return m.GenerateVideo(ctx, r, o)
}
