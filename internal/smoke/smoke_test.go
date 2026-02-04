package smoke

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientToolsListAndCall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		method, _ := req["method"].(string)
		resp := map[string]any{"jsonrpc": "2.0", "id": req["id"]}
		switch method {
		case "initialize":
			resp["result"] = map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "x", "version": "y"}}
		case "tools/list":
			resp["result"] = map[string]any{"tools": []any{map[string]any{"name": "run_sandbox"}}}
		case "tools/call":
			resp["result"] = map[string]any{"ok": true}
		default:
			resp["error"] = map[string]any{"code": -32601, "message": "method_not_found"}
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(srv.URL, "/mcp")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	tools, err := c.ToolsList(ctx)
	if err != nil {
		t.Fatalf("tools list: %v", err)
	}
	if !strings.Contains(string(tools), "run_sandbox") {
		t.Fatalf("unexpected tools: %s", string(tools))
	}

	_, err = c.ToolsCall(ctx, "run_sandbox", map[string]any{"identifier": "x", "commands": []any{map[string]any{"shell": "echo"}}})
	if err != nil {
		t.Fatalf("tools call: %v", err)
	}
}
