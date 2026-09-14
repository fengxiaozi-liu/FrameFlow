package application

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"github.com/fengxiaozi-liu/FrameFlow/internal/infrastructure/queue"
	"testing"
)

func TestGenerationRequiresMatchingHealthyProvider(t *testing.T) {
	r := &providerRepo{m: map[string]provider.Config{}}
	p, _ := provider.New("img", "Image", provider.Image)
	p.BaseURL = "http://example"
	p.Model = "image"
	p.MarkHealthy()
	_ = r.Save(p)
	store := queue.NewStore()
	worker := queue.NewWorker(store)
	s := TaskService{Store: store, Enqueuer: worker, Providers: ProviderService{Repo: r}}
	created, err := s.CreateCharacterImage("img", task.Input{Prompt: "一个角色"})
	if err != nil || created.ProviderCode != "img" {
		t.Fatal(err, created)
	}
}
