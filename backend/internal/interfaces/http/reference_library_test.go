package http

import (
	"bytes"
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/material"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestReferenceUploadScopeAndExplicitLibraryPromotion(t *testing.T) {
	s, done := testServer(t)
	defer done()
	s.UploadDir = t.TempDir()
	router := testRouter(s)
	upload := func(project string, scoped bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		f, _ := form.CreateFormFile("file", "reference.png")
		_ = png.Encode(f, image.NewRGBA(image.Rect(0, 0, 16, 16)))
		_ = form.WriteField("kind", "character")
		_ = form.WriteField("name", "林夏")
		if scoped {
			_ = form.WriteField("reference_only", "true")
			_ = form.WriteField("project_id", project)
		}
		_ = form.Close()
		req := httptest.NewRequest("POST", "/api/materials/upload", &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	if w := upload("", true); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := upload("missing", true); w.Code < 400 {
		t.Fatal("unknown project accepted")
	}
	w := upload("test-project", true)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var local material.Asset
	if err := json.Unmarshal(w.Body.Bytes(), &local); err != nil {
		t.Fatal(err)
	}
	if !local.ReferenceOnly || local.ProjectID != "test-project" {
		t.Fatal(local)
	}
	listed := func(path string) []material.Asset {
		t.Helper()
		w := request(t, router, "GET", path, "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result struct {
			Materials []material.Asset `json:"materials"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Materials
	}
	if len(listed("/api/materials")) != 0 || len(listed("/api/materials?project_id=other")) != 0 {
		t.Fatal("reference leaked into library")
	}
	if len(listed("/api/materials?project_id=test-project")) != 1 {
		t.Fatal("reference unavailable after reload")
	}
	// A normal metadata write from an older client cannot accidentally publish a reference.
	raw, _ := json.Marshal(map[string]any{"name": "renamed", "kind": "character"})
	if w := request(t, router, "PUT", "/api/materials/"+local.ID, string(raw)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(listed("/api/materials")) != 0 {
		t.Fatal("metadata update published reference")
	}
	for i := 0; i < 2; i++ {
		if w := request(t, router, "POST", "/api/materials/"+local.ID+"/library", ""); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if values := listed("/api/materials"); len(values) != 1 || values[0].ReferenceOnly || values[0].ID != local.ID || values[0].URL != local.URL {
		t.Fatal(values)
	}
	if w := upload("", false); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(listed("/api/materials")) != 2 {
		t.Fatal("legacy uploads no longer enter library")
	}
}
