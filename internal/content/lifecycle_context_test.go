package content

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLifecycleWaitCanBeCancelled(t *testing.T) {
	objects := Objects{Dir: t.TempDir()}
	release, e := objects.Lease(true)
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, e = objects.LeaseContext(ctx, false); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
}
