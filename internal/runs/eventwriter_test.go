package runs

import (
	"testing"

	"github.com/jeliasson/mcp-sandboxd/internal/sse"
)

func TestEventWriterCapturesAndTruncates(t *testing.T) {
	b := sse.NewBroker(10)
	w := &eventWriter{broker: b, event: "command_stdout", runID: "r1", index: 0, maxBytes: 5, capture: true}

	// Write longer than maxBytes.
	_, _ = w.Write([]byte("hello world"))
	if got := w.String(); got != "hello" {
		t.Fatalf("expected captured 'hello', got %q", got)
	}
	if !w.truncated {
		t.Fatalf("expected truncated")
	}
	if w.bytes != 5 {
		t.Fatalf("expected bytes=5, got %d", w.bytes)
	}
}
