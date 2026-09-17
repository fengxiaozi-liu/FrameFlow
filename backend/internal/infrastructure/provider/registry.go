package provider

import (
	"context"
	"fmt"

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider/bailian"
)

// Registry routes connections to vendor clients; each client selects its model protocol.
type Registry struct {
	clients map[string]domain.ModelClient
}

func NewRegistry(credentials domain.CredentialReader) *Registry {
	r := &Registry{clients: make(map[string]domain.ModelClient)}
	r.Register("bailian", bailian.Client{Credentials: credentials})
	return r
}

func (r *Registry) Register(vendor string, client domain.ModelClient) {
	r.clients[vendor] = client
}

func (r *Registry) client(connection domain.Connection) (domain.ModelClient, error) {
	if client := r.clients[connection.Vendor]; client != nil {
		return client, nil
	}
	return nil, fmt.Errorf("provider vendor %q is not registered", connection.Vendor)
}

func (r *Registry) GenerateText(ctx context.Context, connection domain.Connection, model domain.Model, input domain.StoryRequest, options domain.RequestOptions) (domain.StoryResult, error) {
	client, err := r.client(connection)
	if err != nil {
		return domain.StoryResult{}, err
	}
	return client.GenerateText(ctx, connection, model, input, options)
}

func (r *Registry) GenerateImage(ctx context.Context, connection domain.Connection, model domain.Model, input domain.ImageRequest, options domain.RequestOptions) (domain.ImageResult, error) {
	client, err := r.client(connection)
	if err != nil {
		return domain.ImageResult{}, err
	}
	return client.GenerateImage(ctx, connection, model, input, options)
}

func (r *Registry) GenerateVideo(ctx context.Context, connection domain.Connection, model domain.Model, input domain.VideoRequest, options domain.RequestOptions) (domain.VideoResult, error) {
	client, err := r.client(connection)
	if err != nil {
		return domain.VideoResult{}, err
	}
	return client.GenerateVideo(ctx, connection, model, input, options)
}

func (r *Registry) PollVideo(ctx context.Context, connection domain.Connection, model domain.Model, id string) (domain.VideoResult, bool, error) {
	client, err := r.client(connection)
	if err != nil {
		return domain.VideoResult{}, false, err
	}
	return client.PollVideo(ctx, connection, model, id)
}

func NewDiscovery() domain.Discovery { return bailian.Client{} }
