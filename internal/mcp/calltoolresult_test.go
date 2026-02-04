package mcp

import "testing"

func TestNewToolResultHasContentAndStructuredContent(t *testing.T) {
	res := newToolResult("run_sandbox", map[string]any{"run_id": "r1", "status_url": "/mcp/runs/r1"})
	if len(res.Content) == 0 {
		t.Fatalf("expected content")
	}
	if res.StructuredContent["run_id"] != "r1" {
		t.Fatalf("expected structuredContent.run_id")
	}
}
