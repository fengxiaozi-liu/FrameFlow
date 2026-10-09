package provider

import (
	"encoding/json"
	"testing"
)

func customStory() CustomInterface {
	return CustomInterface{Path: "/generate", BodyTemplate: `{"name":"{{model}}","input":"{{prompt}}"}`, AuthMode: "bearer", Mode: "sync", ResultPointer: "/answer"}
}
func TestCustomContractValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*CustomInterface)
	}{
		{"external path", func(c *CustomInterface) { c.Path = "https://other.example/x" }},
		{"encoded traversal", func(c *CustomInterface) { c.Path = "/a/%2e%2e/b" }},
		{"query", func(c *CustomInterface) { c.Path = "/generate?api_key=secret" }},
		{"non object", func(c *CustomInterface) { c.BodyTemplate = `[]` }},
		{"unsupported variable", func(c *CustomInterface) { c.BodyTemplate = `{"model":"{{model}}","prompt":"{{duration}}"}` }},
		{"partial variable", func(c *CustomInterface) { c.BodyTemplate = `{"model":"{{model}}","prompt":"prefix {{prompt}}"}` }},
		{"missing prompt", func(c *CustomInterface) { c.BodyTemplate = `{"model":"{{model}}"}` }},
		{"missing model", func(c *CustomInterface) { c.BodyTemplate = `{"prompt":"{{prompt}}"}` }},
		{"pointer", func(c *CustomInterface) { c.ResultPointer = "data.answer" }},
		{"bad escape", func(c *CustomInterface) { c.ResultPointer = "/data/~2" }},
		{"auth", func(c *CustomInterface) { c.AuthMode = "header"; c.AuthHeader = "Host" }},
		{"auth injection", func(c *CustomInterface) { c.AuthMode = "header"; c.AuthHeader = "X-Key"; c.AuthPrefix = "x\r\n" }},
		{"async text", func(c *CustomInterface) { c.Mode = "async" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := customStory()
			tc.edit(&c)
			if c.Validate(Story) == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	valid := customStory()
	if err := valid.Validate(Story); err != nil {
		t.Fatal(err)
	}
	valid.BodyTemplate = `{"model":"{{model}}","messages":"{{messages}}","temperature":0.7}`
	if err := valid.Validate(Story); err != nil {
		t.Fatal(err)
	}
}
func TestCustomAsyncAndFrozenConfiguration(t *testing.T) {
	c := customStory()
	c.Mode = "async"
	c.BodyTemplate = `{"model":"{{model}}","prompt":"{{prompt}}","pictures":"{{images}}"}`
	c.Polling = &CustomPolling{Path: "/jobs/{task_id}", TaskIDPointer: "/job/id", StatusPointer: "/job/state", Pending: []string{"queued", "running"}, Succeeded: []string{"done"}, Failed: []string{"failed", "cancelled"}}
	b := OperationBinding{Protocol: CustomProtocol, Custom: &c, InputMode: "reference_images", MaxReferences: 2, MaxImageBytes: 1024, Ratios: []string{"16:9"}, Durations: []int{4}, Resolutions: []string{"720P"}}
	if err := b.Validate(Video); err != nil {
		t.Fatal(err)
	}
	m := Model{Revision: 1}
	updated, err := m.Configure(1, map[Capability]OperationBinding{Video: b}, []Capability{Video})
	if err != nil {
		t.Fatal(err)
	}
	c.Polling.Pending[0] = "changed"
	if updated.Operations[Video].Custom.Polling.Pending[0] != "queued" {
		t.Fatal("mutable configuration alias")
	}
	data, _ := json.Marshal(updated)
	var roundTrip Model
	if err = json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if err = roundTrip.Operations[Video].Validate(Video); err != nil {
		t.Fatal(err)
	}
	c.Polling.Failed = []string{"done"}
	if b.Validate(Video) == nil {
		t.Fatal("overlapping states accepted")
	}
	c.Polling.Failed = []string{"failed"}
	c.Polling.Path = "/jobs"
	if b.Validate(Video) == nil {
		t.Fatal("missing id accepted")
	}
	b.Protocol = VideoProtocol
	if b.Validate(Video) == nil {
		t.Fatal("custom config allowed under builtin protocol")
	}
}
