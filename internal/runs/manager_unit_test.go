package runs

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeliasson/mcp-sandboxd/internal/config"
)

func TestValidateRunArgs(t *testing.T) {
	err := validateRunArgs(RunSandboxArgs{})
	if err == nil {
		t.Fatalf("expected error")
	}

	ok := RunSandboxArgs{
		Identifier: "abc",
		Commands:   []CommandInput{{Shell: OptionalString{Value: "echo hi", Set: true}}},
	}
	if err := validateRunArgs(ok); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}

	compat := RunSandboxArgs{Identifier: "abc", Commands: []CommandInput{{Shell: OptionalString{Value: "/bin/bash", Set: true}, Argv: []string{"ls"}, Env: EnvVars{}, TimeoutMS: 1}}, Options: RunOptions{AwaitCompletion: func() *bool { b := true; return &b }()}}
	if err := normalizeRunArgs(&compat); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if compat.Commands[0].Shell.String() != "ls" || len(compat.Commands[0].Argv) != 0 {
		t.Fatalf("expected shell-only after normalize, got %#v", compat.Commands[0])
	}
	if err := validateRunArgs(compat); err != nil {
		t.Fatalf("expected ok after normalize, got %v", err)
	}
}

func TestSSEHandlerMissingRunID(t *testing.T) {
	cfg := config.Config{MCPPath: "/mcp", ArtifactsDir: t.TempDir(), MaxRuns: 100}
	m := NewManager(cfg, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/mcp/events", nil)
	w := httptest.NewRecorder()
	m.SSEHandler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("content-type"); ct != "text/event-stream" {
		t.Fatalf("expected SSE content-type, got %q", ct)
	}
}

func TestArtifactsHandlerPreventsTraversal(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{ArtifactsDir: root, MaxRuns: 100}
	m := NewManager(cfg, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/artifacts/../secret.txt", nil)
	w := httptest.NewRecorder()
	m.ArtifactsHandler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestArtifactsHandlerLatestAlias(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{ArtifactsDir: root, MaxRuns: 100}
	m := NewManager(cfg, nil, nil, nil)

	identifier := "chat1"
	runID := "run123"
	m.latestRunByIdentifier[identifier] = runID

	path := filepath.Join(root, identifier, runID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "foo.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/artifacts/"+identifier+"/latest/foo.txt", nil)
	w := httptest.NewRecorder()
	m.ArtifactsHandler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Body.String(); got != "ok" {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestCheckDenylist(t *testing.T) {
	cfg := config.Config{DenylistEnabled: true, DenylistPatterns: []string{"\\bdocker\\b"}, MaxRuns: 100}
	m := NewManager(cfg, nil, nil, nil)
	if err := m.checkDenylist("echo hi"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := m.checkDenylist("docker ps"); err == nil {
		t.Fatalf("expected deny")
	}
}
