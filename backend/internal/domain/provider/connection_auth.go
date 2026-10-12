package provider

import (
	"errors"
	"slices"
	"strings"
)

type ConnectionAuth struct {
	Mode   string `json:"mode"`
	Header string `json:"header,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

func (a ConnectionAuth) Validate() error {
	if a.Mode == "none" || a.Mode == "bearer" {
		return nil
	}
	if a.Mode != "header" || !headerName.MatchString(a.Header) || slices.Contains([]string{"host", "cookie", "connection", "content-length", "content-type", "transfer-encoding"}, strings.ToLower(a.Header)) || strings.ContainsAny(a.Prefix, "\r\n") || len(a.Prefix) > 64 {
		return errors.New("连接认证配置无效")
	}
	return nil
}
func (c Connection) Authentication() ConnectionAuth {
	if c.Auth == nil {
		return ConnectionAuth{Mode: "bearer"}
	}
	return *c.Auth
}

// GenerationSnapshot contains only execution settings and owns its mutable
// values. Directory demos are unrelated to a submitted generation task.
func (c Connection) GenerationSnapshot() Connection {
	c.Discovery = nil
	if c.Auth != nil {
		auth := *c.Auth
		c.Auth = &auth
	}
	return c
}
