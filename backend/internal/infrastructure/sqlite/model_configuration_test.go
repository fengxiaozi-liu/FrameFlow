package sqlite

import (
	"context"
	"errors"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"path/filepath"
	"testing"
	"time"
)

func TestModelConfigurationRevisionAndFrozenTask(t *testing.T) {
	ctx := context.Background()
	r, e := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	m := provider.Model{ID: "c:m", ConnectionID: "c", RemoteID: "qwen-plus", Capabilities: []provider.Capability{provider.Story}, Enabled: true, Supported: true}
	if e = r.SaveModel(ctx, m); e != nil {
		t.Fatal(e)
	}
	m, e = r.GetModel(ctx, m.ID)
	if e != nil || !m.Resolve(provider.Story).Available {
		t.Fatal(m, e)
	}
	item := task.New("frozen", task.KindStory, task.Input{Prompt: "test"}, time.Now())
	item.ModelID = m.ID
	item.ModelConfig = &m
	if e = r.Save(ctx, item); e != nil {
		t.Fatal(e)
	}
	updated, e := r.ConfigureOperations(ctx, m.ID, m.Revision, map[provider.Capability]provider.OperationBinding{provider.Story: {Protocol: provider.ChatProtocol, Source: "manual", Endpoint: "/new/chat"}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.ConfigureOperations(ctx, m.ID, m.Revision, m.Operations, nil); !errors.Is(e, provider.ErrRevisionConflict) {
		t.Fatal("stale edit", e)
	}
	if e = r.SaveModel(ctx, m); !errors.Is(e, provider.ErrRevisionConflict) {
		t.Fatal("stale toggle overwrote config", e)
	}
	item.ModelConfig = &updated
	if e = r.Save(ctx, item); e == nil {
		t.Fatal("snapshot mutation accepted")
	}
	got, e := r.Get(ctx, item.ID)
	if e != nil || got.ModelConfig.Operations[provider.Story].Endpoint != "" {
		t.Fatal(got, e)
	}
}
