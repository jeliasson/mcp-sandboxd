package runs

import (
	"context"
	"io"
)

// Executor runs commands inside a sandbox runtime (Docker, Kubernetes, etc.).
//
// SandboxID is runtime-specific (e.g. Docker container ID or Kubernetes Pod name).
type Executor interface {
	Exec(ctx context.Context, sandboxID string, params ExecParams, stdout, stderr io.Writer) (exitCode int, err error)
	CopyArtifacts(ctx context.Context, sandboxID string) (io.ReadCloser, error)
}

type ExecParams struct {
	Cmd    []string
	Cwd    string
	Env    map[string]string
	AsUser string // "root" or "sandbox"
}
