package task

import (
	"errors"
	"testing"
	"time"

	"github.com/fengxiaozi-liu/FrameFlow/internal/domain/provider"
)

func TestUnknownAcceptanceBlocksRetryWithoutChangingState(t *testing.T) {
	item := New("unknown", KindVideo, Input{Prompt: "test"}, time.Now())
	item.Fail("remote acceptance is unknown", time.Now())
	item.RemoteAcceptanceUnknown = true
	if err := item.Retry(time.Now()); !errors.Is(err, provider.ErrRemoteAcceptanceUnknown) {
		t.Fatal(err)
	}
	if item.Status != StatusFailed || item.RetryCount != 0 {
		t.Fatal("unknown task resubmitted")
	}
}
