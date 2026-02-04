package smoke

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWaitForEventsParsesSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		_, _ = w.Write([]byte(": ok\n\n"))
		_, _ = w.Write([]byte("event: run_started\n"))
		_, _ = w.Write([]byte("data: {\"x\":1}\n\n"))
		_, _ = w.Write([]byte("event: run_finished\n"))
		_, _ = w.Write([]byte("data: {\"ok\":true}\n\n"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	events, err := WaitForEvents(ctx, srv.Client(), srv.URL, func(ev SSEEvent) bool { return ev.Event == "run_finished" })
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected >=2 events, got %d", len(events))
	}
	if events[len(events)-1].Event != "run_finished" {
		t.Fatalf("unexpected last event: %s", events[len(events)-1].Event)
	}
}
