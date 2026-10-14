package http

import (
	"context"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/task"
	"strings"
	"sync"
	"testing"
)

func TestReferenceImageTaskScopeValidationAndReplay(t *testing.T) {
	s, done := testServer(t)
	defer done()
	s.TaskService.Enqueuer = nil
	r := testRouter(s)
	url := "/api/projects/test-project/drafts/test-draft/reference-image-tasks"
	body := `{"idempotency_key":"stable","kind":"character","name":"林夏","prompt":"短发人物","model_id":"test-connection:image","aspect_ratio":"1:1"}`
	var wg sync.WaitGroup
	codes := make(chan int, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- request(t, r, "POST", url, body).Code }()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 202 {
			t.Fatal(code)
		}
	}
	list, err := s.TaskService.Store.List(context.Background())
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	first := list[0]
	if first.Target != "reference_character" || first.SceneID != "" || first.Input.MaterialName != "林夏" || first.Snapshot != nil {
		t.Fatal(first)
	}
	replay := request(t, r, "POST", url, body)
	var got task.Task
	_ = json.Unmarshal(replay.Body.Bytes(), &got)
	if got.ID != first.ID {
		t.Fatal(got)
	}
	if w := request(t, r, "POST", url, strings.Replace(body, "短发人物", "长发人物", 1)); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, bad := range []string{strings.Replace(body, "character", "music", 1), strings.Replace(body, "1:1", "2:3", 1), strings.Replace(body, "短发人物", " ", 1), strings.ReplaceAll(strings.Replace(body, "stable", "disabled", 1), "test-connection:image", "test-connection:video")} {
		bad = strings.Replace(bad, "stable", "invalid-input", 1)
		if w := request(t, r, "POST", url, bad); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w := request(t, r, "POST", strings.Replace(url, "test-draft", "missing", 1), body); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestCustomImageRatioSubmissionUsesModelOptions(t *testing.T) {
	s, done := testServer(t)
	defer done()
	s.TaskService.Enqueuer = nil
	ctx := context.Background()
	m, err := s.Providers.Catalog.Models.GetModel(ctx, "test-connection:image")
	if err != nil {
		t.Fatal(err)
	}
	m.Operations[provider.Image] = provider.OperationBinding{Protocol: provider.CustomProtocol, Source: "manual", ImageModes: []string{provider.TextToImage}, Ratios: []string{"21:9"}, ImageSizes: map[string]string{"21:9": "1344*576"}, Custom: &provider.CustomInterface{Path: "/images", AuthMode: "none", Mode: "sync", ResultPointer: "/url", BodyTemplate: `{"model":"{{model}}","prompt":"{{prompt}}","ratio":"{{ratio}}","size":"{{size}}"}`}}
	if err := s.Providers.Catalog.Models.SaveModel(ctx, m); err != nil {
		t.Fatal(err)
	}
	router := testRouter(s)
	url := "/api/projects/test-project/drafts/test-draft/reference-image-tasks"
	body := `{"idempotency_key":"wide","kind":"scene","name":"宽幅","prompt":"远景","model_id":"test-connection:image","aspect_ratio":"21:9"}`
	response := request(t, router, "POST", url, body)
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	var saved task.Task
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil || saved.Input.AspectRatio != "21:9" {
		t.Fatal(saved.Input, err)
	}
	if rejected := request(t, router, "POST", url, strings.ReplaceAll(strings.Replace(body, "wide", "unsupported", 1), "21:9", "16:9")); rejected.Code != 400 {
		t.Fatal(rejected.Code, rejected.Body.String())
	}
}
