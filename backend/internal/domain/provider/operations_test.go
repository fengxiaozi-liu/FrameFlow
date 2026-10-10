package provider

import (
	"encoding/json"
	"testing"
)

func TestExplicitBindingIgnoresModelIdentity(t *testing.T) {
	for _, pair := range []struct {
		id string
		op Capability
	}{{"qwen-plus", Story}, {"qwen-image-2.0-pro", Image}, {"wan2.7-r2v", Video}, {"qwen3-tts-flash", Speech}} {
		m := MigrateLegacyModel(Model{RemoteID: pair.id, Enabled: true, Capabilities: []Capability{pair.op}})
		m.RemoteID = "unknown/model-version-999"
		m.Name = "anything"
		if !m.Resolve(pair.op).Available {
			t.Fatal(m.Resolve(pair.op))
		}
	}
}
func TestConfigurationMissingAndInvalid(t *testing.T) {
	m := MigrateLegacyModel(Model{RemoteID: "Wan3.0-video", Enabled: true, Capabilities: []Capability{Video}})
	if m.Resolve(Video).Code != "configuration_missing" {
		t.Fatal(m)
	}
	b := MigrateLegacyModel(Model{RemoteID: "wan2.7-r2v", Capabilities: []Capability{Video}}).Operations[Video]
	b.Durations = []int{4, 8}
	m.Operations[Video] = b
	c, ok := m.VideoCapability()
	if !ok {
		t.Fatal(m.Resolve(Video))
	}
	if c.ValidateInput(6, "720P", []string{"reference_image"}) == nil {
		t.Fatal("accepted unconfigured duration")
	}
	if err := c.ValidateInput(8, "720P", []string{"reference_image"}); err != nil {
		t.Fatal(err)
	}
	b.Protocol = ChatProtocol
	m.Operations[Video] = b
	if m.Resolve(Video).Available {
		t.Fatal("wrong protocol accepted")
	}
}
func TestLegacyMigrationIdempotent(t *testing.T) {
	m := MigrateLegacyModel(Model{RemoteID: "wan2.7-i2v", Capabilities: []Capability{Video}})
	a, _ := json.Marshal(m)
	b, _ := json.Marshal(MigrateLegacyModel(m))
	if string(a) != string(b) {
		t.Fatal("not idempotent")
	}
	m.Operations = map[Capability]OperationBinding{}
	if len(MigrateLegacyModel(m).Operations) != 0 {
		t.Fatal("reapplied removed preset")
	}
}

func TestConfigureCapabilityDeclarations(t *testing.T) {
	m := Model{Revision: 1, Capabilities: []Capability{Story}, Enabled: false, Supported: true}
	bindings := map[Capability]OperationBinding{Story: {Protocol: ChatProtocol}}
	if _, err := m.Configure(1, bindings, []Capability{"unknown"}); err == nil {
		t.Fatal("accepted unknown capability")
	}
	updated, err := m.Configure(1, bindings, nil)
	if err != nil || len(updated.Capabilities) != 1 || updated.Enabled {
		t.Fatal(updated, err)
	}
	cleared, err := m.Configure(1, nil, []Capability{})
	if err != nil || cleared.Supported || cleared.Enabled || cleared.Default || len(cleared.Capabilities) != 0 {
		t.Fatal(cleared, err)
	}
	if len(m.Capabilities) != 1 {
		t.Fatal("changed original aggregate")
	}
}

func TestEndpointModes(t *testing.T) {
	defaultPath := "/compatible-mode/v1/chat/completions"
	for _, tt := range []struct {
		name, mode, endpoint, want string
		invalid                    bool
	}{
		{"legacy default", "", "", defaultPath, false},
		{"legacy custom", "", "/legacy/chat", "/legacy/chat", false},
		{"explicit default ignores stale override", "default", "/old/chat", defaultPath, false},
		{"custom", "custom", "/custom/chat", "/custom/chat", false},
		{"empty custom", "custom", "", "", true},
		{"invalid mode", "other", "", "", true},
		{"absolute URL", "custom", "https://other.example/chat", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := OperationBinding{Protocol: ChatProtocol, EndpointMode: tt.mode, Endpoint: tt.endpoint}
			err := b.Validate(Story)
			if (err != nil) != tt.invalid {
				t.Fatal(err)
			}
			if !tt.invalid && b.RequestPath() != tt.want {
				t.Fatal(b.RequestPath(), tt.want)
			}
		})
	}
	m := Model{Revision: 1}
	updated, err := m.Configure(1, map[Capability]OperationBinding{Story: {Protocol: ChatProtocol, EndpointMode: "default", Endpoint: "/old/chat"}}, []Capability{Story})
	if err != nil || updated.Operations[Story].Endpoint != "" {
		t.Fatal(updated, err)
	}
	for _, p := range Protocols() {
		if p.ID == CustomProtocol {
			if p.DefaultPath != "" || len(p.TemplateVariables) < 2 {
				t.Fatal(p)
			}
			continue
		}
		if p.DefaultPath == "" {
			t.Fatal("protocol missing default path", p.ID)
		}
		if p.Operation == Video && p.PollingPath != VideoTaskPathPrefix+"{task_id}" {
			t.Fatal(p)
		}
	}
}

func TestManualCapabilitySelectionRetainsProtocolAndIntent(t *testing.T) {
	m := Model{Revision: 1, Capabilities: []Capability{Story, Image}, Enabled: true, Supported: true, Default: true, Operations: map[Capability]OperationBinding{Story: {Protocol: ChatProtocol}}}
	selected, err := m.SelectCapabilities([]Capability{Image, Image})
	if err != nil || !selected.CapabilitiesConfigured || len(selected.Capabilities) != 1 || !selected.Enabled {
		t.Fatal(selected, err)
	}
	cleared, err := m.Configure(1, m.Operations, []Capability{})
	if err != nil || !cleared.CapabilitiesConfigured || len(cleared.Capabilities) != 0 || cleared.Enabled || cleared.Default || cleared.Supported {
		t.Fatal(cleared, err)
	}
	if cleared.Operations[Story].Protocol != ChatProtocol {
		t.Fatal("clearing capability deleted protocol settings")
	}
	if len(m.Capabilities) != 2 || m.CapabilitiesConfigured {
		t.Fatal("changed original aggregate")
	}
}

func TestVideoModesAreIndependentCapabilities(t *testing.T) {
	model := MigrateLegacyModel(Model{RemoteID: "wan2.7-r2v", Enabled: true, Supported: true, Capabilities: []Capability{Video}})
	model.RemoteID = "unfamiliar-video"
	binding := model.Operations[Video]
	binding.InputMode = ""
	binding.InputModes = []string{"first_frame", "reference_images"}
	binding.LastFrame = true
	model.Operations[Video] = binding
	cap, ok := model.InputCapability()
	if !ok || !cap.FirstFrame || !cap.ReferenceVideo {
		t.Fatal(cap)
	}
	if _, err := model.VideoCapabilityFor(""); err == nil {
		t.Fatal("ambiguous mode accepted")
	}
	first, err := model.VideoCapabilityFor("first_frame")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.ValidateInput(4, "720P", []string{"first_frame", "last_frame"}); err != nil {
		t.Fatal(err)
	}
	if first.ValidateInput(4, "720P", []string{"first_frame", "reference_image"}) == nil {
		t.Fatal("mixed inputs accepted")
	}
	refs, err := model.VideoCapabilityFor("reference_images")
	if err != nil {
		t.Fatal(err)
	}
	if err := refs.ValidateInput(4, "720P", []string{"reference_image", "reference_image"}); err != nil {
		t.Fatal(err)
	}
	if refs.ValidateInput(4, "720P", []string{"reference_image", "last_frame"}) == nil {
		t.Fatal("tail accepted in reference mode")
	}
	binding.InputModes = []string{"reference_images"}
	model.Operations[Video] = binding
	if model.Resolve(Video).Available {
		t.Fatal("tail support without frame mode accepted")
	}
	binding.LastFrame = false
	binding.InputModes = []string{}
	if binding.Validate(Video) == nil {
		t.Fatal("empty modes accepted")
	}
}
