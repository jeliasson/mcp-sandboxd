package server

import (
	"testing"

	"github.com/jeliasson/mcp-sandboxd/internal/config"
)

func TestNewAppUnsupportedBackend(t *testing.T) {
	cfg := config.Config{SandboxBackend: "nope"}
	_, err := NewApp(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
}
