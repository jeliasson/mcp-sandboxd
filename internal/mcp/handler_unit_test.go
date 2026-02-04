package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeliasson/mcp-sandboxd/internal/config"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(ctx context.Context) error { return f.err }

func TestInitialize(t *testing.T) {
	cfg := config.Config{MCPPath: "/mcp", ToolDescOverridesEnabled: true}
	h := NewHandler(cfg, nil, nil, fakePinger{}).(http.Handler)

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"x","version":"y"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Mcp-Session-Id") == "" {
		t.Fatalf("expected Mcp-Session-Id header")
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("serverInfo")) {
		t.Fatalf("expected serverInfo in response")
	}
}

func TestInitializedNotificationNoID(t *testing.T) {
	cfg := config.Config{MCPPath: "/mcp", ToolDescOverridesEnabled: true}
	h := NewHandler(cfg, nil, nil, fakePinger{}).(http.Handler)

	body := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestToolsList(t *testing.T) {
	cfg := config.Config{MCPPath: "/mcp"}
	h := NewHandler(cfg, nil, nil, fakePinger{}).(http.Handler)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	m, ok := resp.Result.(map[string]any)
	if !ok {
		// json.Unmarshal into interface uses map[string]any, but Result is typed any
		// only when decoding; here Response.Result is any and got encoded already.
		// So just ensure response body contains tool names.
		if !bytes.Contains(w.Body.Bytes(), []byte("run_sandbox")) {
			t.Fatalf("expected tools")
		}
		return
	}
	_ = m
}

func TestToolsCallInvalidParams(t *testing.T) {
	cfg := config.Config{MCPPath: "/mcp"}
	h := NewHandler(cfg, nil, nil, fakePinger{}).(http.Handler)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"not-an-object"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("invalid_params")) {
		t.Fatalf("expected invalid_params, got %s", w.Body.String())
	}
}

func TestToolsCallDockerUnavailable(t *testing.T) {
	cfg := config.Config{MCPPath: "/mcp"}
	h := NewHandler(cfg, nil, nil, fakePinger{err: context.DeadlineExceeded}).(http.Handler)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"run_sandbox","arguments":{}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("BACKEND_UNAVAILABLE")) {
		t.Fatalf("expected BACKEND_UNAVAILABLE, got %s", w.Body.String())
	}
}
