package story

import (
	"testing"
	"time"
)

func TestDocumentScene(t *testing.T) {
	d := Document{}
	if err := d.UpdateBody("body", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := d.AddScene(Scene{Title: "opening"}); err != nil || len(d.Scenes) != 1 || d.Scenes[0].Order != 1 {
		t.Fatal(err, d.Scenes)
	}
}
