package http

import (
	"encoding/json"
	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
	"net/url"
	"testing"
)

func TestCustomModelConfigurationContract(t *testing.T) {
	s, done := testServer(t)
	defer done()
	r := testRouter(s)
	v := request(t, r, "POST", "/api/connections/test-connection/models", `{"model_id":"arbitrary/video","capabilities":["video"]}`)
	if v.Code != 201 {
		t.Fatal(v.Code, v.Body.String())
	}
	var m provider.Model
	if e := json.Unmarshal(v.Body.Bytes(), &m); e != nil {
		t.Fatal(e)
	}
	path := "/api/models/" + url.PathEscape(m.ID)
	v = request(t, r, "GET", path+"/input-capability", "")
	if v.Code != 200 {
		t.Fatal(v.Code, v.Body.String())
	}
	var caps provider.InputCapability
	_ = json.Unmarshal(v.Body.Bytes(), &caps)
	if caps.Operations[provider.Video].Code != "configuration_missing" {
		t.Fatal(caps)
	}
	body := `{"expected_revision":1,"operations":{"video":{"protocol":"dashscope.media-video.v1","source":"manual","input_mode":"first_frame","ratios":["16:9"],"durations":[4,8],"resolutions":["720P"],"max_references":1,"max_image_bytes":10485760}}}`
	v = request(t, r, "PUT", path+"/operations", body)
	if v.Code != 200 {
		t.Fatal(v.Code, v.Body.String())
	}
	v = request(t, r, "GET", path+"/input-capability", "")
	_ = json.Unmarshal(v.Body.Bytes(), &caps)
	if !caps.FirstFrame || len(caps.Durations) != 2 || !caps.Operations[provider.Video].Available {
		t.Fatal(caps)
	}
	if v = request(t, r, "PUT", path+"/operations", body); v.Code != 409 {
		t.Fatal("stale write accepted", v.Code)
	}
}

// A catalogue may return only IDs. Users can declare capability in the same
// revision-checked request as its protocol without a partial metadata update.
func TestConfigureUnclassifiedModelAtomically(t *testing.T) {
	s, done := testServer(t)
	defer done()
	r := testRouter(s)
	v := request(t, r, "POST", "/api/connections/test-connection/models", `{"model_id":"unknown-image","capabilities":[]}`)
	if v.Code != 201 {
		t.Fatal(v.Code, v.Body.String())
	}
	var m provider.Model
	if e := json.Unmarshal(v.Body.Bytes(), &m); e != nil {
		t.Fatal(e)
	}
	path := "/api/models/" + url.PathEscape(m.ID)
	// Incomplete image parameters cannot leave behind the capability declaration.
	v = request(t, r, "PUT", path+"/operations", `{"expected_revision":1,"capabilities":["image"],"operations":{"image":{"protocol":"dashscope.image.v1"}}}`)
	if v.Code != 422 {
		t.Fatal(v.Code, v.Body.String())
	}
	body := `{"expected_revision":1,"capabilities":["image"],"operations":{"image":{"protocol":"dashscope.image.v1","ratios":["1:1"],"image_sizes":{"1:1":"1024*1024"},"max_references":0}}}`
	v = request(t, r, "PUT", path+"/operations", body)
	if v.Code != 200 {
		t.Fatal(v.Code, v.Body.String())
	}
	if e := json.Unmarshal(v.Body.Bytes(), &m); e != nil {
		t.Fatal(e)
	}
	if m.Revision != 2 || len(m.Capabilities) != 1 || m.Capabilities[0] != provider.Image || !m.Supported || m.Enabled {
		t.Fatal(m)
	}
	if v = request(t, r, "PUT", path+"/operations", body); v.Code != 409 {
		t.Fatal(v.Code)
	}
}
