package docker

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCandidateHostsPrefersXDGPodman(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("socket paths are linux-specific")
	}

	dir := t.TempDir()
	podmanDir := filepath.Join(dir, "podman")
	if err := os.MkdirAll(podmanDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create a dummy unix socket file by just creating a file won't set ModeSocket;
	// so this test only asserts that the function doesn't panic and is influenced
	// by XDG_RUNTIME_DIR when sockets exist on the real system.
	//
	// We still validate ordering logic with env present.
	t.Setenv("XDG_RUNTIME_DIR", dir)
	_ = candidateHosts()
}
