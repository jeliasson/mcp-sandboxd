package sandbox

import "context"

// API is implemented by a sandbox backend (Docker, Kubernetes, etc.).
//
// A sandbox is keyed by identifier and represents a persistent execution environment.
// Implementations should keep the sandbox alive until TTL expiry (or explicit delete/restart).
type API interface {
	Ensure(ctx context.Context, identifier string, ttlSeconds int) (*Sandbox, error)
	Delete(ctx context.Context, identifier string) error
	Restart(ctx context.Context, identifier string, ttlSeconds int) (*Sandbox, error)

	ReapOnce(ctx context.Context) (int, error)
	Count() int
}
