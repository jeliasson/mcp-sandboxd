package mcp

import (
	"strings"
	"testing"

	"github.com/jeliasson/mcp-sandboxd/internal/config"
)

func TestToolsListResult_DescriptionAppend(t *testing.T) {
	cfg := config.Config{
		ToolDescOverridesEnabled: true,
		ToolDescRunSandboxAppend: "Extra guidance.",
	}
	res := ToolsListResult(cfg)
	tools, ok := res["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("expected tools list")
	}

	first, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first tool map")
	}
	if first["name"] != "run_sandbox" {
		t.Fatalf("expected run_sandbox first, got %v", first["name"])
	}
	desc, _ := first["description"].(string)
	if desc == "" {
		t.Fatalf("expected description")
	}
	if !strings.HasSuffix(desc, "Extra guidance.") {
		t.Fatalf("expected appended text, got %q", desc)
	}
}

func TestToolsListResult_DescriptionOverride(t *testing.T) {
	cfg := config.Config{
		ToolDescOverridesEnabled:   true,
		ToolDescRunSandboxOverride: "Custom.",
		ToolDescRunSandboxAppend:   "More.",
	}
	res := ToolsListResult(cfg)
	tools := res["tools"].([]any)
	first := tools[0].(map[string]any)
	desc := first["description"].(string)
	if desc != "Custom. More." {
		t.Fatalf("unexpected description: %q", desc)
	}
}

func TestToolsListResult_OverridesDisabled(t *testing.T) {
	cfg := config.Config{
		ToolDescOverridesEnabled:   false,
		ToolDescRunSandboxOverride: "Custom.",
		ToolDescRunSandboxAppend:   "More.",
	}
	res := ToolsListResult(cfg)
	tools := res["tools"].([]any)
	first := tools[0].(map[string]any)
	desc := first["description"].(string)
	if desc != runSandboxBaseDescription {
		t.Fatalf("expected base description, got %q", desc)
	}
}
