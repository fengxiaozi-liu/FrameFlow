package bailian

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"net/http"
	"strings"
)

func (c Client) Voices(m provider.Model) []provider.Voice {
	cap, ok := m.InputCapability()
	if !ok || !cap.Speech {
		return nil
	}
	return cap.Voices
}
func (c Client) GenerateSpeech(ctx context.Context, connection provider.Connection, m provider.Model, input provider.SpeechRequest, options provider.RequestOptions) (provider.SpeechResult, error) {
	if m.Require(provider.Speech, provider.SpeechProtocol) != nil {
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
	if !valid || strings.TrimSpace(input.Text) == "" || len([]rune(input.Text)) > m.Operations[provider.Speech].MaxTextLength {
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
	if err = c.request(ctx, connection, key, http.MethodPost, operationPath(m, provider.Speech), body, &out, false); err != nil {
		return provider.SpeechResult{}, err
	}
	if out.Output.Audio.URL == "" {
		return provider.SpeechResult{}, errors.New("speech response has no audio URL")
	}
	return provider.SpeechResult{URL: out.Output.Audio.URL}, nil
}
