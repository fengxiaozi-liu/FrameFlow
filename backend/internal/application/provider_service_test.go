package application

import (
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"testing"
)

type providerRepo struct{ m map[string]provider.Config }

func (r *providerRepo) Save(c provider.Config) error {
	r.m[c.Code] = c
	return nil
}

func (r *providerRepo) Get(id string) (provider.Config, bool) {
	c, o := r.m[id]
	return c, o
}

func (r *providerRepo) List(k provider.Capability) []provider.Config {
	o := []provider.Config{}
	for _, c := range r.m {
		if c.Capability == k {
			o = append(o, c)
		}
	}
	return o
}

func (r *providerRepo) Delete(id string) error {
	delete(r.m, id)
	return nil
}

func TestProviderService(t *testing.T) {
	r := &providerRepo{m: map[string]provider.Config{}}
	s := ProviderService{Repo: r}
	c, _ := provider.New("video-a", "Video A", provider.Video)
	if e := s.Save(c); e != nil || len(s.List(provider.Video)) != 1 {
		t.Fatal(e)
	}
	c, e := s.SetEnabled(c.Code, true)
	if e != nil || !c.Enabled {
		t.Fatal(e)
	}
}
