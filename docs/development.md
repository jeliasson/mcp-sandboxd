# Development

## Local quickstart

```bash
make docker-build-sandbox
cp .env.example .env
make dev
```

See [Configuration](configuration.md) for environment variables, and [Docker backend](docker.md) / [Kubernetes](kubernetes.md) for deployment-specific notes.

## Useful targets

- `make dev`: rebuild/restart on file changes
- `make test`: run unit tests
- `make docker-build`: build server image
- `make docker-build-sandbox`: build sandbox image
- `make docker-run`: run server against a DinD container

## Notes

- The sandbox image is long-running; it should contain `/workspace`, `/artifacts`, and tools needed for `exec` (`bash`, `timeout`, `tar`, `setpriv`).
