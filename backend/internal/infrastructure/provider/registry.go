package provider

import (
	"fmt"

	domain "github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

// NewRegistry 注册内置适配器。外部适配器可以在启动时添加，无需修改领域 processor。
func NewRegistry() *domain.Registry {
	registry := domain.NewRegistry()
	registry.Register("mock", func(config domain.Config) (domain.Adapter, error) {
		mock := Mock{Fail: config.Model == "mock-fail-video"}
		switch config.Capability {
		case domain.Story:
			return StoryMock{Mock: mock}, nil
		case domain.Image:
			return ImageMock{Mock: mock}, nil
		case domain.Video:
			return VideoMock{Mock: mock}, nil
		default:
			return nil, fmt.Errorf("unsupported mock capability %q", config.Capability)
		}
	})
	return registry
}
