package provider

import (
	"strings"
	"testing"
)

func TestDiscoveryExampleMappings(t *testing.T) {
	p := DefaultCustomDiscoveryProfile()
	p.ResponseJSONExample = `{"data":[{"id":"arbitrary-video"}]}`
	compiled, err := p.Compile()
	if err != nil || compiled.Schema == nil {
		t.Fatal(compiled, err)
	}
	p.ModelListPointer = "/missing"
	if _, err := p.Compile(); err == nil {
		t.Fatal("bad mapping accepted")
	}
}

func TestOperationPlainDemoCompilesTypedMappings(t *testing.T) {
	b := OperationBinding{Protocol: CustomProtocol, Custom: &CustomInterface{Path: "/generate", AuthMode: "bearer"}, Contract: &OperationContract{Execution: SyncJSON, Request: JSONContract{Example: `{"model":"demo","prompt":"demo","temperature":0.5}`, Bindings: map[string]string{"model": "/model", "prompt": "/prompt"}}, Response: JSONContract{Example: `{"answer":"demo"}`, Bindings: map[string]string{"result": "/answer"}}}}
	compiled, err := b.Compile(Story)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Contract.Request.Schema == nil || compiled.Custom.ResultPointer != "/answer" {
		t.Fatal(compiled)
	}
	value, _, _ := ParseJSONExample(`{"answer":7}`)
	if compiled.Contract.Response.ValidateMappings(value) == nil {
		t.Fatal("response type drift accepted")
	}
}

func TestOperationRejectsOverlappingRequestAndNonStringResult(t *testing.T) {
	for _, test := range []struct {
		request, response string
		bindings          map[string]string
		message           string
	}{
		{`{"model":"demo","input":[{"prompt":"demo"}]}`, `{"result":"demo"}`, map[string]string{"model": "/model", "messages": "/input", "prompt": "/input/0/prompt"}, "父节点与子节点"},
		{`{"model":"demo","prompt":"demo"}`, `{"result":42}`, map[string]string{"model": "/model", "prompt": "/prompt"}, "必须是字符串"},
	} {
		contract := OperationContract{Execution: SyncJSON, Request: JSONContract{Example: test.request, Bindings: test.bindings}, Response: JSONContract{Example: test.response, Bindings: map[string]string{"result": "/result"}}}
		if _, _, err := contract.Compile(Story, CustomInterface{Path: "/generate", AuthMode: "none"}); err == nil || !strings.Contains(err.Error(), test.message) {
			t.Fatal("contract failed for wrong reason", err)
		}
	}
}
