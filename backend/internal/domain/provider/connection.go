package provider

import (
	"errors"
	"net/url"
	"strings"
)

// RequestURL combines an origin and an absolute API path without dropping prefixes
// or allowing a protocol adapter to redirect credentials to a different host.
func (c Connection) RequestURL(path string) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	relative, err := url.Parse(path)
	if err != nil || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\\#") || relative.IsAbs() || relative.Host != "" || relative.User != nil || relative.Fragment != "" {
		return "", errors.New("request path must start with / and cannot contain a host or fragment")
	}
	base, _ := url.Parse(strings.TrimSpace(c.BaseURL))
	base.Path, base.RawPath, base.RawQuery, base.ForceQuery = relative.Path, relative.RawPath, relative.RawQuery, relative.ForceQuery
	return base.String(), nil
}
