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

func TestSceneIdentitySurvivesReorderCopyAndDelete(t *testing.T) {
	d := Document{}
	for _, title := range []string{"one", "two", "three"} {
		if err := d.AddScene(Scene{Title: title, VisualPrompt: title, DurationSeconds: 5}); err != nil {
			t.Fatal(err)
		}
	}
	firstID := d.Scenes[0].ID
	secondID := d.Scenes[1].ID
	if firstID == "" || secondID == "" || firstID == secondID {
		t.Fatal("scene IDs must be stable and unique")
	}
	if err := d.MoveScene(firstID, 3); err != nil {
		t.Fatal(err)
	}
	if d.Scenes[2].ID != firstID || d.Scenes[2].Order != 3 {
		t.Fatalf("reorder changed identity: %+v", d.Scenes)
	}
	copied, err := d.CopyScene(firstID)
	if err != nil || copied.ID == firstID || copied.Title != "one" || copied.Order != 4 {
		t.Fatalf("copy failed: %+v, %v", copied, err)
	}
	if _, err := d.UpdateScene(secondID, Scene{Title: "new title", VisualPrompt: "new visual", DurationSeconds: 6}); err != nil {
		t.Fatal(err)
	}
	deleted, err := d.DeleteScene(firstID)
	if err != nil || deleted.ID != firstID || len(d.Scenes) != 3 {
		t.Fatalf("delete failed: %+v, %v", deleted, err)
	}
	for i, scene := range d.Scenes {
		if scene.Order != i+1 {
			t.Fatalf("noncontiguous order: %+v", d.Scenes)
		}
	}
}
