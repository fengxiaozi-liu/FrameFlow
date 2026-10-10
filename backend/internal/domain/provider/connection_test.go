package provider

import "testing"

func TestConnectionOriginAndRequestURL(t *testing.T) {
	c := Connection{ID: "c", Name: "Test", Vendor: "custom", BaseURL: "https://example.com:8443/"}
	for _, path := range []string{"/v1/chat/completions", "/v2/render", "/tasks/job%2F1", "/api/v1/models?page_no=2&page_size=100"} {
		got, err := c.RequestURL(path)
		if err != nil || got != "https://example.com:8443"+path {
			t.Fatalf("%s: %q %v", path, got, err)
		}
	}
	for _, path := range []string{"", "v1/models", "//other.example/key", "https://other.example/api", "/\\other.example", "/v1#fragment"} {
		if _, err := c.RequestURL(path); err == nil {
			t.Fatalf("accepted path %q", path)
		}
	}
	for _, origin := range []string{"", "http://example.com", "https://user:pass@example.com", "https://example.com/v1", "https://example.com/?key=secret", "https://example.com/#x", "https://example.com/?", "https://example.com/%2F"} {
		c.BaseURL = origin
		if err := c.Validate(); err == nil {
			t.Fatalf("accepted origin %q", origin)
		}
	}
}
