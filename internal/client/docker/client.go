package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
)

type Client struct {
	raw *client.Client
}

func New() (*Client, error) {
	// Prefer explicit DOCKER_HOST if set.
	if os.Getenv("DOCKER_HOST") != "" {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, err
		}
		return &Client{raw: cli}, nil
	}

	// Auto-detect common local sockets (NixOS often uses Podman).
	for _, host := range candidateHosts() {
		cli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
		if err != nil {
			continue
		}
		c := &Client{raw: cli}
		ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
		err = c.Ping(ctx)
		cancel()
		if err == nil {
			return c, nil
		}
		_ = cli.Close()
	}

	// Fall back to Docker defaults if nothing was reachable.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{raw: cli}, nil
}

func candidateHosts() []string {
	var out []string

	// Podman rootless: $XDG_RUNTIME_DIR/podman/podman.sock
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		sock := filepath.Join(dir, "podman", "podman.sock")
		if fileExists(sock) {
			out = append(out, "unix://"+sock)
		}
		// Rootless Docker sometimes uses $XDG_RUNTIME_DIR/docker.sock
		dockerSock := filepath.Join(dir, "docker.sock")
		if fileExists(dockerSock) {
			out = append(out, "unix://"+dockerSock)
		}
	}

	// Podman rootful
	if fileExists("/run/podman/podman.sock") {
		out = append(out, "unix:///run/podman/podman.sock")
	}

	// Common Docker sockets
	if fileExists("/var/run/docker.sock") {
		out = append(out, "unix:///var/run/docker.sock")
	}
	if fileExists("/run/docker.sock") {
		out = append(out, "unix:///run/docker.sock")
	}

	return out
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeSocket != 0
}

func (c *Client) Raw() *client.Client {
	return c.raw
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.raw.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker ping failed: %w", err)
	}
	return nil
}

func (c *Client) Close() error {
	return c.raw.Close()
}
