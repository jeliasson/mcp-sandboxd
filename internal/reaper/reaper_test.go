package reaper

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/jeliasson/mcp-sandboxd/internal/metrics"
)

type fakeManager struct {
	count   atomic.Int64
	deleted atomic.Int64
}

func (f *fakeManager) ReapOnce(ctx context.Context) (int, error) {
	f.deleted.Add(2)
	f.count.Add(-2)
	return 2, nil
}

func (f *fakeManager) Count() int {
	return int(f.count.Load())
}

func TestReaperRunsAndStops(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	mgr := &fakeManager{}
	mgr.count.Store(5)

	r := New(mgr, m, 10*time.Millisecond)
	r.Start()
	defer r.Stop()

	time.Sleep(25 * time.Millisecond)
	if mgr.deleted.Load() == 0 {
		t.Fatalf("expected deletes")
	}
}
