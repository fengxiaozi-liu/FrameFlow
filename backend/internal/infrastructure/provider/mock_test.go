package provider

import (
	"context"
	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"testing"
)

func TestMockProviderFlow(t *testing.T) {
	if v, e := (StoryMock{}).Generate(context.Background(), domain.StoryRequest{Prompt: "城市"}, domain.RequestOptions{}); e != nil || v.Document == "" {
		t.Fatal(e)
	}
	if v, e := (ImageMock{}).Generate(context.Background(), domain.ImageRequest{}, domain.RequestOptions{}); e != nil || v.URL == "" {
		t.Fatal(e)
	}
	if v, e := (VideoMock{}).Generate(context.Background(), domain.VideoRequest{}, domain.RequestOptions{}); e != nil || v.JobReference == "" {
		t.Fatal(e)
	}
}
