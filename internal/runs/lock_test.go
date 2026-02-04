package runs

import (
	"context"
	"testing"
	"time"
)

func TestFIFOLockSerializes(t *testing.T) {
	var l FIFOLock
	ctx := context.Background()

	release1, err := l.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire1: %v", err)
	}

	acquired2 := make(chan struct{})
	go func() {
		release2, err := l.Acquire(ctx)
		if err != nil {
			return
		}
		close(acquired2)
		release2()
	}()

	select {
	case <-acquired2:
		t.Fatalf("lock did not serialize")
	case <-time.After(50 * time.Millisecond):
		// Expected: second waiter still blocked.
	}

	release1()

	select {
	case <-acquired2:
		// ok
	case <-time.After(time.Second):
		t.Fatalf("second waiter did not acquire")
	}
}

func TestFIFOLockCancelRemovesWaiter(t *testing.T) {
	var l FIFOLock

	release1, err := l.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire1: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = l.Acquire(ctx)
	if err == nil {
		release1()
		t.Fatalf("expected cancel error")
	}

	release1()

	// Ensure subsequent acquisition still works after release.
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	release2, err := l.Acquire(ctx2)
	if err != nil {
		t.Fatalf("acquire2: %v", err)
	}
	release2()
}
