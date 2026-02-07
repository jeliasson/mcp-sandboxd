# Architecture (technical)

This document describes the HTTP/MCP surface and backend responsibilities.

Related docs:
- [Development](development.md)
- [Configuration](configuration.md)
- [Observability](observability.md)
- [Docker](docker.md)
- [Kubernetes](kubernetes.md)

## Protocol

- MCP over HTTP: `POST ${MCP_PATH}` (default `/mcp`)
- Streaming output: `GET ${MCP_PATH}/events?run_id=...` (SSE)
- Run status: `GET ${MCP_PATH}/runs/{run_id}`
- Artifacts: `GET /artifacts/{identifier}/{run_id}/{path...}` and `GET /artifacts/{identifier}/latest/{path...}`

## Core concepts

**Identifier**
- A stable key (e.g. chat id) that maps to one persistent sandbox environment.

**Sandbox**
- A long-running execution target keyed by identifier.
- Backends implement `internal/sandbox.API`.

**Run**
- A single invocation of `run_sandbox` producing a `run_id`.
- A run can execute multiple commands sequentially.

**Executor**
- Runs a command inside an existing sandbox environment.
- Implementations: Docker exec and Kubernetes exec.

## Backends

**Docker**
- Manages long-running containers per identifier.
- Executes commands using Docker exec.
- Extracts `/artifacts` using Docker `CopyFromContainer`.

**Kubernetes**
- Manages long-running Pods per identifier.
- Executes commands using Kubernetes `pods/exec` streaming.
- Extracts `/artifacts` by streaming a `tar` archive from inside the Pod.

## Artifact flow

1. Commands run inside the sandbox.
2. After the run completes, the server copies `/artifacts` out of the sandbox into `ARTIFACTS_DIR/{identifier}/{run_id}`.
3. MCP clients download artifacts from the server via `GET /artifacts/...`.

## Configuration

- `SANDBOX_BACKEND=docker|kubernetes`
- `SANDBOX_IMAGE` selects the sandbox image (used by both backends)
- `KUBERNETES_SANDBOX_NAMESPACE` selects where sandbox Pods are created
- `KUBERNETES_SANDBOX_CONTAINER_NAME` selects which container to exec into
