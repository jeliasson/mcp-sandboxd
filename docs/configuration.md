# Configuration

This document describes the environment variables for `mcp-sandboxd`. If you prefer, copy `.env.example` to `.env` and tweak.

**Related docs**

- [Development](development.md)
- [Docker](docker.md)
- [Kubernetes](kubernetes.md)
- [Images](images.md)
- [Security](security.md)

## General

| Key | Default | Description |
| --- | ------- | ----------- |
| `PORT` | `8080` | HTTP listen port. |
| `MCP_PATH` | `/mcp` | MCP JSON-RPC endpoint path. |
| `ARTIFACTS_DIR` | `./data/artifacts` | Where extracted artifacts are stored and served from. |

## Sandbox

### Backend

| Key | Default | Description |
| --- | ------- | ----------- |
| `SANDBOX_BACKEND` | `docker` | `docker` or `kubernetes`. |
| `SANDBOX_IMAGE` | Required | Sandbox image reference. |

### Runtime isolation

These settings apply to sandbox isolation for both the Docker and Kubernetes backends.

| Key | Default | Description |
| --- | ------- | ----------- |
| `SANDBOX_NO_NEW_PRIVILEGES` | `true` | Enables `no-new-privileges` (Docker) / disables privilege escalation (Kubernetes). |
| `SANDBOX_CAP_DROP` | `ALL` | Capabilities to drop. |
| `SANDBOX_CAP_ADD` | `SETUID,SETGID,CHOWN,FOWNER,DAC_OVERRIDE` | Capabilities to add. |
| `SANDBOX_CAPS_STRICT` | `true` | Strict capability checking. |
| `SANDBOX_CAPS_BYPASS_CHECK` | `false` | Bypass capability checks. |

## Backend

### Docker

| Key | Default | Description |
| --- | ------- | ----------- |
| `DOCKER_HOST` | Optional | Docker/Podman socket or daemon endpoint. |
| `AUTO_BUILD_SANDBOX_IMAGE` | `true` | Build sandbox image if missing. |
| `SANDBOX_DOCKERFILE_PATH` | `docker/sandbox.Dockerfile` | Dockerfile to build sandbox image. |
| `SANDBOX_NETWORK_MODE` | `bridge` | Docker network mode. |

### Kubernetes

| Key | Default | Description |
| --- | ------- | ----------- |
| `KUBERNETES_SANDBOX_NAMESPACE` | \<Server namespace\> | Namespace where sandbox Pods are created. |
| `KUBERNETES_SANDBOX_CONTAINER_NAME` | `sandbox` | Container name inside sandbox Pods. |
| `KUBERNETES_SANDBOX_LABEL_PREFIX` | `mcp-sandboxd.jeliasson.dev` | Label key prefix (DNS subdomain) used for sandbox Pod labels. |

## Tool description overrides

| Key | Default | Description |
| --- | ------- | ----------- |
| `TOOL_DESC_OVERRIDES_ENABLED` | `true` | Enable tool description overrides. |
| `TOOL_DESC_RUN_SANDBOX_OVERRIDE` | `null` | Override for `run_sandbox` description. |
| `TOOL_DESC_RUN_SANDBOX_APPEND` | `null` | Text to append to `run_sandbox` description. |
| `TOOL_DESC_DELETE_SANDBOX_OVERRIDE` | `null` | Override for `delete_sandbox` description. |
| `TOOL_DESC_DELETE_SANDBOX_APPEND` | `null` | Text to append to `delete_sandbox` description. |
| `TOOL_DESC_RESTART_SANDBOX_OVERRIDE` | `null` | Override for `restart_sandbox` description. |
| `TOOL_DESC_RESTART_SANDBOX_APPEND` | `null` | Text to append to `restart_sandbox` description. |

## Limits

| Key | Default | Description |
| --- | ------- | ----------- |
| `DEFAULT_CPU_CORES` | `1` | Default CPU cores allocated to sandboxes. |
| `DEFAULT_MEMORY_MB` | `1024` | Default memory in MB allocated to sandboxes. |
| `DEFAULT_PIDS` | `256` | Default PIDs limit for sandboxes. |
| `DEFAULT_COMMAND_TIMEOUT_MS` | `120000` | Default timeout in milliseconds for commands. |
| `DEFAULT_STDOUT_MAX_BYTES` | `1048576` | Maximum stdout size in bytes. |
| `DEFAULT_STDERR_MAX_BYTES` | `1048576` | Maximum stderr size in bytes. |
| `DEFAULT_TTL_SECONDS` | `3600` | Default Time-To-Live for sandboxes. |
| `MAX_TTL_SECONDS` | `604800` | Maximum Time-To-Live for sandboxes. |
| `MAX_RUNS` | `10000` | Maximum number of concurrent runs. |
| `REAPER_INTERVAL_MS` | `5000` | Interval for sandbox reaper to run. |

## Logging

| Key | Default | Description |
| --- | ------- | ----------- |
| `LOG_HTTP_REQUESTS` | `false` | Log all HTTP requests. |
| `LOG_MCP_REQUESTS` | `false` | Log all MCP requests. |
| `LOG_TOOLCALLS` | `false` | Log all tool calls. |

## CORS

| Key | Default | Description |
| --- | ------- | ----------- |
| `CORS_ALLOW_ORIGINS` | Optional | Comma-separated list of allowed CORS origins. |
