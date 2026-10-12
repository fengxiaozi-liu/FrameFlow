package provider

import "testing"

func TestImageModesAreExplicitAndIndependentOfModelID(t *testing.T) {
	b := OperationBinding{Protocol: ImageProtocol, Source: "manual", ImageModes: []string{ImageToImage}, MinReferences: 1, MaxReferences: 3, MaxImageBytes: 10 << 20, Ratios: []string{"1:1"}, ImageSizes: map[string]string{"1:1": "1024*1024"}}
	m := Model{RemoteID: "unfamiliar-model", Enabled: true, Supported: true, Capabilities: []Capability{Image}, Operations: map[Capability]OperationBinding{Image: b}}
	if !m.Resolve(Image).Available {
		t.Fatal(m.Resolve(Image))
	}
	for _, tc := range []struct {
		mode  string
		count int
		valid bool
	}{
		{ImageToImage, 1, true}, {ImageToImage, 3, true}, {ImageToImage, 0, false}, {ImageToImage, 4, false}, {TextToImage, 0, false}, {"invalid", 1, false},
	} {
		if err := b.ValidateImageInput(tc.mode, tc.count); (err == nil) != tc.valid {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
	b.ImageModes = []string{TextToImage}
	if err := b.ValidateImageInput(ImageToImage, 1); err == nil {
		t.Fatal("reference limit must not enable explicit disabled editing")
	}
	if err := b.ValidateImageInput(TextToImage, 1); err == nil {
		t.Fatal("text input accepted references")
	}
	b.ImageModes = []string{}
	if err := b.Validate(Image); err == nil {
		t.Fatal("empty explicit modes accepted")
	}
}

func TestImageModeConstraintsAndLegacyResolution(t *testing.T) {
	legacy := OperationBinding{MaxReferences: 3}
	b := legacy.ResolvedImageBinding()
	if !b.SupportsImageMode(ImageToImage) || b.MinReferences != 1 || b.MaxImageBytes != MaxLocalImageBytes || legacy.ImageModes != nil {
		t.Fatal(b, legacy)
	}
	if !(OperationBinding{}).SupportsImageMode(TextToImage) || (OperationBinding{}).SupportsImageMode(ImageToImage) {
		t.Fatal("legacy text model gained editing")
	}
	for _, tc := range []OperationBinding{
		{ImageModes: []string{ImageToImage}, MinReferences: 0, MaxReferences: 3, MaxImageBytes: 1024},
		{ImageModes: []string{ImageToImage}, MinReferences: 4, MaxReferences: 3, MaxImageBytes: 1024},
		{ImageModes: []string{ImageToImage}, MinReferences: 1, MaxReferences: 3},
		{ImageModes: []string{ImageToImage}, MinReferences: 1, MaxReferences: 3, MaxImageBytes: MaxLocalImageBytes + 1},
		{ImageModes: []string{TextToImage, TextToImage}},
	} {
		if tc.validateImageModes() == nil {
			t.Fatal(tc)
		}
	}
}
