package provider

import (
	"context"
	"time"
)

type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

type RequestOptions struct {
	Timeout time.Duration
	Retry   RetryPolicy
}

type StoryRequest struct{ Prompt, Model string }
type StoryResult struct{ Document string }
type ImageRequest struct{ Prompt, Model, AspectRatio string }
type ImageResult struct{ URL string }
type VideoRequest struct{ Prompt, Model, AspectRatio string }
type VideoResult struct{ JobReference string }

type StoryPort interface {
	Generate(context.Context, StoryRequest, RequestOptions) (StoryResult, error)
}
type ImagePort interface {
	Generate(context.Context, ImageRequest, RequestOptions) (ImageResult, error)
}
type VideoPort interface {
	Generate(context.Context, VideoRequest, RequestOptions) (VideoResult, error)
}

// Context cancellation is the common cancellation protocol for every provider.
func WithTimeout(parent context.Context, options RequestOptions) (context.Context, context.CancelFunc) {
	if options.Timeout <= 0 {
		options.Timeout = 60 * time.Second
	}
	return context.WithTimeout(parent, options.Timeout)
}
