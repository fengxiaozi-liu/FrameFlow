package task

import (
	"testing"
	"time"
)

func TestSnapshotIgnoresDeliveryURLButTracksContentAndOrder(t *testing.T) {
	s := GenerationSnapshot{ProjectID: "p", DraftID: "d", SceneID: "s", ModelID: "m", CapabilityVersion: "v1", Entities: []SnapshotEntity{{References: []MediaReference{{MaterialID: "a", ContentHash: "1", URL: "signed-a"}, {MaterialID: "b", ContentHash: "2"}}}}}
	a, err := FreezeSnapshot(s)
	if err != nil {
		t.Fatal(err)
	}
	s.Entities[0].References[0].URL = "signed-new"
	s.CreatedAt = time.Now()
	b, _ := FreezeSnapshot(s)
	if a.InputHash != b.InputHash {
		t.Fatal("delivery URL/time changed identity")
	}
	if a.Entities[0].References[0].URL != "signed-a" {
		t.Fatal("snapshot aliased mutable caller input")
	}
	s.Entities[0].References[0], s.Entities[0].References[1] = s.Entities[0].References[1], s.Entities[0].References[0]
	c, _ := FreezeSnapshot(s)
	if a.InputHash == c.InputHash {
		t.Fatal("order not hashed")
	}
	s.Entities[0].References[0].ContentHash = ""
	if _, err := FreezeSnapshot(s); err == nil {
		t.Fatal("missing digest accepted")
	}
}
