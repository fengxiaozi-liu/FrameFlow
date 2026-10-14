package provider

import (
	"strings"
	"testing"
)

func TestRatioOptionsRespectProtocolAndPreserveProviderValues(t *testing.T) {
	binding := OperationBinding{
		Protocol: CustomProtocol, Source: "manual", InputMode: "first_frame",
		Ratios: []string{"adaptive", "21:9", "VendorMode"}, Durations: []int{5},
		Resolutions: []string{"720P"}, MaxReferences: 1, MaxImageBytes: 1024,
		Custom: &CustomInterface{Path: "/videos", AuthMode: "none", Mode: "sync",
			BodyTemplate: `{"model":"{{model}}","prompt":"{{prompt}}","first":"{{first_frame}}","ratio":"{{ratio}}"}`, ResultPointer: "/url"},
	}
	if err := binding.Validate(Video); err != nil {
		t.Fatal(err)
	}
	for _, value := range binding.Ratios {
		if err := binding.ValidateAspectRatio(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"16:9", "vendormode", "Adaptive"} {
		if err := binding.ValidateAspectRatio(value); err == nil {
			t.Fatalf("accepted unconfigured ratio %q", value)
		}
	}
	for _, values := range [][]string{nil, {""}, {"adaptive", "adaptive"}, {" adaptive"}, {"a b"}, {"a\x00b"}, {"a,b"}, {strings.Repeat("x", 65)}, make([]string, 65)} {
		invalid := binding
		invalid.Ratios = values
		if err := invalid.Validate(Video); err == nil {
			t.Fatalf("accepted invalid ratio options %q", values)
		}
	}
	builtin := MigrateLegacyModel(Model{RemoteID: "wan2.7-i2v", Capabilities: []Capability{Video}}).Operations[Video]
	builtin.Ratios = []string{"adaptive"}
	if err := builtin.Validate(Video); err == nil {
		t.Fatal("configuration expanded built-in protocol")
	}
	builtin.Ratios = []string{"16:9", "9:16", "1:1"}
	if err := builtin.Validate(Video); err != nil {
		t.Fatal("legacy configuration rejected", err)
	}
}

func TestCustomImageRatioStillRequiresExplicitOutputSize(t *testing.T) {
	b := OperationBinding{Protocol: CustomProtocol, Source: "manual", ImageModes: []string{TextToImage}, Ratios: []string{"21:9"}, ImageSizes: map[string]string{"21:9": "1344*576"}, Custom: &CustomInterface{Path: "/images", AuthMode: "none", Mode: "sync", ResultPointer: "/url", BodyTemplate: `{"model":"{{model}}","prompt":"{{prompt}}","ratio":"{{ratio}}","size":"{{size}}"}`}}
	if err := b.Validate(Image); err != nil {
		t.Fatal(err)
	}
	b.ImageSizes = nil
	if err := b.Validate(Image); err == nil {
		t.Fatal("image ratio accepted without size")
	}
}
