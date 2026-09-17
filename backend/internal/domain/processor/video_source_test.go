package processor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVideoSourceReadsLocalImage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "generated-1.png"), []byte("png"), 0600); err != nil {
		t.Fatal(err)
	}
	p := Processor{MediaDir: dir}
	got, err := p.videoSource("/media/generated-1.png")
	if err != nil || got != "data:image/png;base64,cG5n" {
		t.Fatalf("source = %q, %v", got, err)
	}
	for _, source := range []string{"/media/../private.png", "/media/missing.png", "/media/video.mp4"} {
		if _, err := p.videoSource(source); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
	if got, err := p.videoSource("https://example.com/image.png"); err != nil || !strings.HasPrefix(got, "https://") {
		t.Fatalf("remote source = %q, %v", got, err)
	}
}
