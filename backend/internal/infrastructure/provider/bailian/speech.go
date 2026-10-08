package bailian

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"net/http"
	"strings"
)

func (c Client) Voices(m provider.Model) []provider.Voice {
	cap, ok := provider.InputCapabilityFor(m.RemoteID)
	if !ok || !cap.Speech {
		return nil
	}
	return cap.Voices
}
func (c Client) GenerateSpeech(ctx context.Context, connection provider.Connection, m provider.Model, input provider.SpeechRequest, options provider.RequestOptions) (provider.SpeechResult, error) {
	if !m.Supports(provider.Speech) {
		return provider.SpeechResult{}, provider.ErrUnsupported
	}
	valid := false
	for _, v := range c.Voices(m) {
		if v.ID == input.Voice {
			for _, l := range v.Languages {
				if l == input.Language {
					valid = true
				}
			}
		}
	}
	if !valid || strings.TrimSpace(input.Text) == "" || len([]rune(input.Text)) > 600 {
		return provider.SpeechResult{}, errors.New("voice, language or text unsupported")
	}
	if input.Instructions != "" {
		return provider.SpeechResult{}, errors.New("instruction control is not supported by this speech model")
	}
	ctx, cancel := provider.WithTimeout(ctx, options)
	defer cancel()
	key, err := c.key(ctx, connection)
	if err != nil {
		return provider.SpeechResult{}, err
	}
	body := map[string]any{"model": m.RemoteID, "input": map[string]string{"text": input.Text, "voice": input.Voice, "language_type": input.Language}}
	var out struct {
		Output struct {
			Audio struct {
				URL string `json:"url"`
			} `json:"audio"`
		} `json:"output"`
	}
	if err = c.request(ctx, connection, key, http.MethodPost, "/api/v1/services/aigc/multimodal-generation/generation", body, &out, false); err != nil {
		return provider.SpeechResult{}, err
	}
	if out.Output.Audio.URL == "" {
		return provider.SpeechResult{}, errors.New("speech response has no audio URL")
	}
	return provider.SpeechResult{URL: out.Output.Audio.URL}, nil
}
func (c Client) discoverCompatible(ctx context.Context, connection provider.Connection, key string) ([]provider.DiscoveredModel, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.request(ctx, connection, key, http.MethodGet, "/compatible-mode/v1/models", nil, &out, false); err != nil {
		return nil, err
	}
	values := []provider.DiscoveredModel{}
	for _, m := range out.Data {
		caps := []provider.Capability{}
		switch {
		case m.ID == "qwen3-tts-flash":
			caps = append(caps, provider.Speech)
		case imageProtocol(m.ID):
			caps = append(caps, provider.Image)
		case m.ID == "wan2.7-i2v" || m.ID == "wan2.7-i2v-2026-04-25" || m.ID == "wan2.7-r2v" || m.ID == "wan2.7-r2v-2026-06-12":
			caps = append(caps, provider.Video)
		case chatProtocol(m.ID):
			caps = append(caps, provider.Story)
		}
		values = append(values, provider.DiscoveredModel{ID: m.ID, Name: m.ID, Capabilities: caps})
	}
	return values, nil
}
