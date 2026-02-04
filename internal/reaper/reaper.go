package reaper

import (
	"context"
	"log"
	"time"

	"github.com/jeliasson/mcp-sandboxd/internal/metrics"
)

type Manager interface {
	ReapOnce(ctx context.Context) (int, error)
	Count() int
}

type Reaper struct {
	mgr     Manager
	metrics *metrics.Metrics

	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func New(mgr Manager, metrics *metrics.Metrics, interval time.Duration) *Reaper {
	return &Reaper{
		mgr:      mgr,
		metrics:  metrics,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (r *Reaper) Start() {
	go func() {
		defer close(r.doneCh)
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				deleted, err := r.mgr.ReapOnce(ctx)
				cancel()
				if err != nil {
					log.Printf("reaper error: %v", err)
					continue
				}
				if deleted > 0 {
					r.metrics.ReaperDeletesTotal.Add(float64(deleted))
				}
				r.metrics.ActiveSandboxes.Set(float64(r.mgr.Count()))
			case <-r.stopCh:
				return
			}
		}
	}()
}

func (r *Reaper) Stop() {
	close(r.stopCh)
	<-r.doneCh
}
