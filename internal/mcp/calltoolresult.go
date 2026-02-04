package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CallToolResult matches MCP 2025-11-25 schema.
type CallToolResult struct {
	Content           []any          `json:"content"`
	IsError           bool           `json:"isError,omitempty"`
	StructuredContent map[string]any `json:"structuredContent,omitempty"`
}

func newToolResult(toolName string, structured any) CallToolResult {
	sc := map[string]any{}
	summary := toolName

	if m, ok := structured.(map[string]any); ok {
		for k, v := range m {
			sc[k] = v
		}
		if rid, _ := m["run_id"].(string); rid != "" {
			summary = fmt.Sprintf("%s: started run %s", toolName, rid)
			if ev, _ := m["events_url"].(string); ev != "" {
				summary += fmt.Sprintf("; events_url=%s", ev)
			}
			if st, _ := m["status_url"].(string); st != "" {
				summary += fmt.Sprintf("; status_url=%s", st)
			}
		}
	}

	// Provide a textual fallback that an LLM/user can read.
	pretty, _ := json.MarshalIndent(structured, "", "  ")
	text := summary
	if len(pretty) > 0 {
		text += "\n" + strings.TrimSpace(string(pretty))
	}

	res := CallToolResult{
		Content: []any{map[string]any{"type": "text", "text": text}},
	}
	if len(sc) > 0 {
		res.StructuredContent = sc
	}
	return res
}
