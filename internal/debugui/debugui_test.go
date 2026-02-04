package debugui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerDisabledReturns404(t *testing.T) {
	h := Handler(false, "/mcp")

	req := httptest.NewRequest(http.MethodGet, "/debug", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandlerEnabledServesHTMLWithMCPPath(t *testing.T) {
	h := Handler(true, "/mcpX")

	req := httptest.NewRequest(http.MethodGet, "/debug", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "MCP_PATH") {
		t.Fatalf("expected debug html")
	}
	if !strings.Contains(body, "/mcpX") {
		t.Fatalf("expected MCP path replacement")
	}
}
