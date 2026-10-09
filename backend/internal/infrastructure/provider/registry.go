package provider

import (
	"context"
	"fmt"

	"github.com/fengxiaozi-liu/FrameFlow/internal/application/modelport"
	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider/bailian"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/provider/customjson"
)

// Registry resolves explicit protocol versions independently of vendor and model IDs.
type ProtocolAdapter struct {
	Text   modelport.TextGenerator
	Image  modelport.ImageGenerator
	Video  modelport.VideoGenerator
	Speech domain.SpeechClient
	Audio  modelport.AudioUploader
}
type Registry struct{ clients map[string]ProtocolAdapter }

func NewRegistry(credentials domain.CredentialReader) *Registry {
	r := &Registry{clients: map[string]ProtocolAdapter{}}
	client := bailian.Client{Credentials: credentials}
	r.Register(domain.ChatProtocol, ProtocolAdapter{Text: client})
	r.Register(domain.ImageProtocol, ProtocolAdapter{Image: client})
	r.Register(domain.VideoProtocol, ProtocolAdapter{Video: client, Audio: client})
	r.Register(domain.SpeechProtocol, ProtocolAdapter{Speech: client})
	custom := customjson.Client{Credentials: credentials}
	r.Register(domain.CustomProtocol, ProtocolAdapter{Text: custom, Image: custom, Video: custom, Speech: custom})
	return r
}
func (r *Registry) Register(protocol string, adapter ProtocolAdapter) { r.clients[protocol] = adapter }

func (r *Registry) client(model domain.Model, operation domain.Capability) (ProtocolAdapter, error) {
	status := model.Resolve(operation)
	if !status.Available {
		return ProtocolAdapter{}, fmt.Errorf("%s: %s", status.Code, status.Reason)
	}
	if client, ok := r.clients[status.Binding.Protocol]; ok {
		valid := operation == domain.Story && client.Text != nil || operation == domain.Image && client.Image != nil || operation == domain.Video && client.Video != nil || operation == domain.Speech && client.Speech != nil
		if !valid {
			return ProtocolAdapter{}, domain.ErrUnsupported
		}
		return client, nil
	}
	return ProtocolAdapter{}, fmt.Errorf("protocol %q is not registered", status.Binding.Protocol)
}

func (r *Registry) GenerateText(ctx context.Context, connection domain.Connection, model domain.Model, input domain.StoryRequest, options domain.RequestOptions) (domain.StoryResult, error) {
	client, err := r.client(model, domain.Story)
	if err != nil {
		return domain.StoryResult{}, err
	}
	return client.Text.GenerateText(ctx, connection, model, input, options)
}

func (r *Registry) GenerateImage(ctx context.Context, connection domain.Connection, model domain.Model, input domain.ImageRequest, options domain.RequestOptions) (domain.ImageResult, error) {
	client, err := r.client(model, domain.Image)
	if err != nil {
		return domain.ImageResult{}, err
	}
	return client.Image.GenerateImage(ctx, connection, model, input, options)
}

func (r *Registry) GenerateVideo(ctx context.Context, connection domain.Connection, model domain.Model, input domain.VideoRequest, options domain.RequestOptions) (domain.VideoResult, error) {
	client, err := r.client(model, domain.Video)
	if err != nil {
		return domain.VideoResult{}, err
	}
	return client.Video.GenerateVideo(ctx, connection, model, input, options)
}

func (r *Registry) PollVideo(ctx context.Context, connection domain.Connection, model domain.Model, id string) (domain.VideoResult, bool, error) {
	client, err := r.client(model, domain.Video)
	if err != nil {
		return domain.VideoResult{}, false, err
	}
	return client.Video.PollVideo(ctx, connection, model, id)
}

func (r *Registry) UploadAudio(ctx context.Context, connection domain.Connection, model domain.Model, localPath string) (string, error) {
	client, err := r.client(model, domain.Video)
	if err != nil {
		return "", err
	}
	if client.Audio == nil {
		return "", domain.ErrUnsupported
	}
	return client.Audio.UploadAudio(ctx, connection, model, localPath)
}

func NewDiscovery() domain.Discovery { return bailian.Client{} }

func (r *Registry) GenerateSpeech(ctx context.Context, connection domain.Connection, model domain.Model, input domain.SpeechRequest, options domain.RequestOptions) (domain.SpeechResult, error) {
	client, err := r.client(model, domain.Speech)
	if err != nil {
		return domain.SpeechResult{}, err
	}
	return client.Speech.GenerateSpeech(ctx, connection, model, input, options)
}

func (r *Registry) Voices(connection domain.Connection, model domain.Model) []domain.Voice {
	client, err := r.client(model, domain.Speech)
	if err != nil {
		return nil
	}
	return client.Speech.Voices(model)
}
