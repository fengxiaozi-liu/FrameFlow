package material

import (
	"testing"
	"time"
)

func TestAsset(t *testing.T) {
	a, e := New("1", "cover", Visual, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = a.UpdateURL("/media/a"); e != nil || a.URL != "/media/a" {
		t.Fatal(e)
	}
}
