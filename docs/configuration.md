# Configuration

This document describes `mcp-sandboxd` environment variables.

If you prefer, copy `.env.example` to `.env` and tweak.

Related docs:
- [Development](development.md)
- [Docker](docker.md)
- [Kubernetes](kubernetes.md)
- [Images](images.md)

## General

- `PORT` (default `8080`): HTTP listen port.
- `MCP_PATH` (default `/mcp`): MCP JSON-RPC endpoint path.
- `ARTIFACTS_DIR` (default `./data/artifacts`): where extracted artifacts are stored and served from.

## Backend selection

- `SANDBOX_BACKEND` (default `docker`): `docker` or `kubernetes`.
- `SANDBOX_IMAGE` (required): sandbox image reference (used by both backends).

### Image tags

See [Container images](images.md) for the published image references and tag scheme.

## Docker backend

- `DOCKER_HOST` (optional): docker/podman socket or daemon endpoint.
- `AUTO_BUILD_SANDBOX_IMAGE` (default `true`): build sandbox image if missing.
- `SANDBOX_DOCKERFILE_PATH` (default `docker/sandbox.Dockerfile`): Dockerfile to build sandbox image.
- `SANDBOX_NETWORK_MODE` (default `bridge`)
- `SANDBOX_NO_NEW_PRIVILEGES` (default `true`)
- `SANDBOX_CAP_DROP` (default `ALL`)
- `SANDBOX_CAP_ADD` (default `SETUID,SETGID,CHOWN,FOWNER,DAC_OVERRIDE`)
- `SANDBOX_CAPS_STRICT` (default `true`)
- `SANDBOX_CAPS_BYPASS_CHECK` (default `false`)

## Kubernetes backend

- `KUBERNETES_SANDBOX_NAMESPACE` (optional): namespace where sandbox Pods are created (defaults to the server namespace).
- `KUBERNETES_SANDBOX_CONTAINER_NAME` (default `sandbox`): container name inside sandbox Pods.
- `KUBERNETES_SANDBOX_LABEL_PREFIX` (default `mcp-sandboxd.jeliasson.dev`): label key prefix (DNS subdomain) used for sandbox Pod labels.

## Tool description overrides

- `TOOL_DESC_OVERRIDES_ENABLED` (default `true`)
- `TOOL_DESC_RUN_SANDBOX_OVERRIDE`, `TOOL_DESC_RUN_SANDBOX_APPEND`
- `TOOL_DESC_DELETE_SANDBOX_OVERRIDE`, `TOOL_DESC_DELETE_SANDBOX_APPEND`
- `TOOL_DESC_RESTART_SANDBOX_OVERRIDE`, `TOOL_DESC_RESTART_SANDBOX_APPEND`

## Limits

- `DEFAULT_TTL_SECONDS` (default `3600`)
- `MAX_TTL_SECONDS` (default `604800`)
- `REAPER_INTERVAL_MS` (default `5000`)
- `DEFAULT_CPU_CORES` (default `1`)
- `DEFAULT_MEMORY_MB` (default `1024`)
- `DEFAULT_PIDS` (default `256`)
- `DEFAULT_COMMAND_TIMEOUT_MS` (default `120000`)
- `DEFAULT_STDOUT_MAX_BYTES` (default `1048576`)
- `DEFAULT_STDERR_MAX_BYTES` (default `1048576`)
- `MAX_RUNS` (default `10000`)

## Logging

- `LOG_HTTP_REQUESTS` (default `false`)
- `LOG_MCP_REQUESTS` (default `false`)
- `LOG_TOOLCALLS` (default `false`)

## CORS

- `CORS_ALLOW_ORIGINS` (optional, comma-separated)
