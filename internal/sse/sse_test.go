package sse

import (
	"testing"
	"time"
)

func TestBrokerHistoryAndSubscribe(t *testing.T) {
	b := NewBroker(10)
	b.Publish("a", map[string]any{"x": 1})
	b.Publish("b", map[string]any{"y": 2})

	ch, cancel := b.Subscribe(10)
	defer cancel()

	// History should replay immediately.
	timeout := time.After(time.Second)
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case msg := <-ch:
			seen[msg.Event] = true
		case <-timeout:
			t.Fatalf("timeout waiting for history")
		}
	}
}

func TestBrokerCloseClosesSubscribers(t *testing.T) {
	b := NewBroker(10)
	ch, _ := b.Subscribe(1)
	b.Close()
	_, ok := <-ch
	if ok {
		t.Fatalf("expected channel closed")
	}
}
