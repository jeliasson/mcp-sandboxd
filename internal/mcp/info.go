package mcp

import (
	"encoding/json"
	"net/http"
)

func InfoHandler(mcpPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message":  "MCP endpoint expects POST with JSON body.",
			"mcp_path": mcpPath,
			"examples": map[string]any{
				"tools_list": map[string]any{
					"method": "POST",
					"path":   mcpPath,
					"body":   map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{}},
				},
			},
		})
	})
}
