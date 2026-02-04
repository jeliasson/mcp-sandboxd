package runs

import (
	"context"
	"sync"
)

// FIFOLock provides best-effort FIFO serialization per identifier.
// It is used only when lock_sandbox=true.
//
// This is intentionally simple and in-memory.
type FIFOLock struct {
	mu     sync.Mutex
	queue  []chan struct{}
	locked bool
}

func (l *FIFOLock) Acquire(ctx context.Context) (func(), error) {
	waiter := make(chan struct{})

	l.mu.Lock()
	l.queue = append(l.queue, waiter)
	if !l.locked && len(l.queue) == 1 {
		l.locked = true
		close(waiter)
	}
	l.mu.Unlock()

	select {
	case <-waiter:
		return func() { l.release() }, nil
	case <-ctx.Done():
		// If we were granted at the same time as cancellation,
		// treat as acquired to avoid leaving the lock stuck.
		select {
		case <-waiter:
			return func() { l.release() }, nil
		default:
		}

		l.mu.Lock()
		for i, ch := range l.queue {
			if ch == waiter {
				l.queue = append(l.queue[:i], l.queue[i+1:]...)
				break
			}
		}
		l.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (l *FIFOLock) release() {
	l.mu.Lock()
	if len(l.queue) > 0 {
		l.queue = l.queue[1:]
	}
	if len(l.queue) == 0 {
		l.locked = false
		l.mu.Unlock()
		return
	}
	next := l.queue[0]
	close(next)
	l.mu.Unlock()
}
