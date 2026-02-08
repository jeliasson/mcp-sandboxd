# Development

This document describes a high-level overview of the development and contribution cycle for `mcp-sandboxd`.

**Related docs**

- [Architecture](architecture.md)
- [Configuration](configuration.md)

## Local quickstart

Prereqs:

- Docker (or Podman with Docker API compatibility)
- `make`

```bash
make docker-build-sandbox
cp .env.example .env
make dev
```

The server listens on `PORT` (default `8080`) and serves MCP JSON-RPC on `MCP_PATH` (default `/mcp`).

See [Configuration](configuration.md) for all environment variables.

## Contribution

0. Open Issue describing what you want
1. Build sandbox image: `make docker-build-sandbox`
2. Run server once: `make run`
3. Implement your changes
4. Make MCP request
5. Validate result
6. Run unit tests: `make test`
7. (Repeat Step 1-6...)
8. Open Pull Request and reference the issue

## Make

- `make dev`: rebuild/restart on file changes
- `make test`: run unit tests
- `make docker-build`: build server image
- `make docker-build-sandbox`: build sandbox image
- `make docker-run`: run server against a DinD container

## Notes

- The sandbox image is long-running; it should contain `/workspace`, `/artifacts`, and tools needed for `exec` (`bash`, `timeout`, `tar`, `setpriv`).
