package kubernetes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewUsesKubeconfigWhenProvided(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	kubeconfig := `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://127.0.0.1
    insecure-skip-tls-verify: true
users:
- name: u
  user:
    token: dummy
contexts:
- name: ctx
  context:
    cluster: c
    user: u
current-context: ctx
`
	if err := os.WriteFile(cfgPath, []byte(kubeconfig), 0o644); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	t.Setenv("KUBECONFIG", cfgPath)
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c == nil || c.Clientset == nil || c.Config == nil {
		t.Fatalf("expected non-nil client")
	}
}

func TestNewErrorsWhenNoConfigAvailable(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	_, err := New()
	if err == nil {
		t.Fatalf("expected error")
	}
}
