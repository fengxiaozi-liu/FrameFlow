package application

import (
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
)

type GenerationService struct {
	Tasks     TaskService
	Providers ProviderService
}

func (s GenerationService) Create(kind, providerCode string) (task.Task, error) {
	expected := map[string]provider.Capability{"story": provider.Story, "image": provider.Image, "video": provider.Video}[kind]
	if expected == "" {
		return task.Task{}, errors.New("unsupported generation kind")
	}
	p, ok := s.Providers.Get(providerCode)
	if !ok {
		return task.Task{}, errors.New("provider not found")
	}
	if p.Capability != expected {
		return task.Task{}, errors.New("provider capability mismatch")
	}
	if !p.Enabled || p.Status != provider.Healthy {
		return task.Task{}, errors.New("provider is not ready")
	}
	return s.Tasks.CreateWithProvider(kind, providerCode)
}
func (s GenerationService) CreateCharacterImage(providerCode string) (task.Task, error) {
	return s.Create("image", providerCode)
}
func (s GenerationService) CreateFrameImage(providerCode string) (task.Task, error) {
	return s.Create("image", providerCode)
}
