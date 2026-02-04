# Observability

This document describes the built-in Prometheus metrics exposed by `mcp-sandboxd`.

Related docs:
- [Architecture](architecture.md)
- [Configuration](configuration.md)
- [Kubernetes](kubernetes.md)

## Prometheus metrics

`mcp-sandboxd` exposes a Prometheus-compatible metrics endpoint:

- `GET /metrics`

This endpoint is currently always enabled.

### Built-in collectors

The `/metrics` endpoint includes:

- Go runtime metrics (`go_*`)
- Process metrics (`process_*`)
- Application metrics (`mcp_*`)

### Application metrics

The following application metrics are exported.

Gauges:

- `mcp_active_sandboxes`: number of active sandboxes tracked in-memory.
- `mcp_active_runs`: number of active runs currently executing.

Counters:

- `mcp_runs_total{status="completed|failed"}`: total runs by final status.
- `mcp_commands_total{status="completed|failed"}`: total commands by final status.
- `mcp_reaper_deletes_total`: total sandbox deletions performed by the reaper.

Histogram:

- `mcp_command_duration_seconds`: command duration in seconds (Prometheus default buckets).

### Quick check

```sh
curl -s http://localhost:8080/metrics | head
```

## Prometheus scrape example

### Minimal scrape config

```yaml
scrape_configs:
  - job_name: mcp-sandboxd
    metrics_path: /metrics
    static_configs:
      - targets:
          - localhost:8080
```

### Kubernetes (Service)

If you expose `mcp-sandboxd` as a Service, configure Prometheus to scrape that Service DNS name.

Example target:

- `mcp-sandboxd.mcp-sandboxd.svc.cluster.local:8080`

If you use the Prometheus Operator, consider using a `ServiceMonitor`.

## Security notes

- Treat `/metrics` as operational data; expose it only to your monitoring network.
- If you publish `mcp-sandboxd` behind an Ingress, consider restricting `/metrics` with network policy and/or auth at the ingress layer.