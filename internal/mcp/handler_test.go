package mcp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		writeResp(w, Response{JSONRPC: "2.0", Error: NewError(-32700, "parse_error", nil)})
	})

	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
