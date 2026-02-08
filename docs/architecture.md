# Architecture

This document describes the core concepts, backends, HTTP/MCP endpoints and artifacts for `mcp-sandboxd`.

**Related docs**

- [Development](development.md)
- [Configuration](configuration.md)
- [Observability](observability.md)
- [Security](security.md)
- [Docker](docker.md)
- [Kubernetes](kubernetes.md)

## Core concepts

**Identifier**
- A stable key (e.g. conversation id) that maps to one persistent sandbox environment.
- Must match `^[a-zA-Z0-9_-]{1,36}$`

**Sandbox**
- A long-running execution target keyed by `identifier`. 
- Terminates automatically after [`DEFAULT_TTL_SECONDS`](configuration.md#DEFAULT_TTL_SECONDS) or [`MAX_TTL_SECONDS`](configuration.md#MAX_TTL_SECONDS) seconds.

**Run**
- A single invocation of [`run_sandbox`](tools.md) producing a `run_id`.
- A run can execute multiple commands sequentially.

**Executor**
- Runs a command inside an existing sandbox environment.
- Talks to backend, such as Docker or Kuebrnetes.

## Backends

**Docker**
- Manages long-running containers per `identifier`.
- Executes commands using Docker's `docker exec`.
- Extracts `/artifacts` using Docker `CopyFromContainer`.

**Kubernetes**
- Manages long-running Pods per `identifier`.
- Executes commands using Kubernetes `pods/exec`.
- Extracts `/artifacts` by streaming a `tar` archive from inside the Pod.

## Endpoints

- MCP over HTTP: `POST ${MCP_PATH}` (default `/mcp`)
- Streaming output: `GET ${MCP_PATH}/events?run_id=...` (SSE)
- Run status: `GET ${MCP_PATH}/runs/{run_id}`
- Artifacts: `GET /artifacts/{identifier}/{run_id}/{path...}` and `GET /artifacts/{identifier}/latest/{path...}`

## Artifact flow

1. Commands run inside the sandbox.
2. After the run completes, the server copies `/artifacts` out of the sandbox into `${ARTIFACTS_DIR}/{identifier}/{run_id}`.
3. MCP clients download artifacts from the server via `GET /artifacts/...`.

## Configuration

- [`SANDBOX_BACKEND`](configuration.md#SANDBOX_BACKEND) selects the backend (`docker` or `kubernetes`).
- [`SANDBOX_IMAGE`](configuration.md#SANDBOX_IMAGE) selects the sandbox image (used by both backends).
- [`KUBERNETES_SANDBOX_NAMESPACE`](configuration.md#KUBERNETES_SANDBOX_NAMESPACE) selects where sandbox Pods are created.
- [`KUBERNETES_SANDBOX_CONTAINER_NAME`](configuration.md#KUBERNETES_SANDBOX_CONTAINER_NAME) selects which container to exec into.
