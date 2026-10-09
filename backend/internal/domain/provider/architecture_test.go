package provider

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDomainDependencyBoundary(t *testing.T) {
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, i := range f.Imports {
			v, _ := strconv.Unquote(i.Path.Value)
			for _, bad := range []string{"/application", "/infrastructure", "/interfaces", "gin-gonic", "database/sql", "net/http"} {
				if strings.Contains(v, bad) {
					t.Errorf("domain dependency: %s -> %s", path, v)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
