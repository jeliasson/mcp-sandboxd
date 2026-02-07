# Development

## Local quickstart

Prereqs:

- Docker (or Podman with Docker API compatibility)
- `make`

```bash
make docker-build-sandbox
cp .env.example .env
make dev
```

The server listens on `PORT` (default `8080`) and serves MCP JSON-RPC on `MCP_PATH` (default `/mcp`). See [Configuration](configuration.md) for all environment variables, and [Docker](docker.md) / [Kubernetes](kubernetes.md) for deployment-specific notes.

## Common workflows

- Rebuild sandbox image: `make docker-build-sandbox`
- Run unit tests: `make test`
- Run server once (no file watch): `make run`
- Run a second dev instance: `make dev--agent` (uses `AGENT_PORT`)

## Make

- `make dev`: rebuild/restart on file changes
- `make test`: run unit tests
- `make docker-build`: build server image
- `make docker-build-sandbox`: build sandbox image
- `make docker-run`: run server against a DinD container

## Notes

- The sandbox image is long-running; it should contain `/workspace`, `/artifacts`, and tools needed for `exec` (`bash`, `timeout`, `tar`, `setpriv`).
