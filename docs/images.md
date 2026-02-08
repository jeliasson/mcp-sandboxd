# Images

This document describes images being built and published for `mcp-sandboxd` and `mcp-sandboxd-sandbox`.

**Related docs**

- [Configuration](configuration.md)
- [Docker](docker.md)
- [Kubernetes](kubernetes.md)
- [Security](security.md)

**Published container images**

- **Server** `ghcr.io/jeliasson/mcp-sandboxd`
- **Sandbox** `ghcr.io/jeliasson/mcp-sandboxd-sandbox`

## Tags

The CI publishes different tag flavors depending on pipeline.

- **Release**:
  - `:<short-sha>`
  - `:latest`
  - `:<release-tag>` (e.g. `:v1.2.3`)

- **Development**:
  - `:<short-sha>-dev`

- **Playground**:
  - `:<short-sha>-play`
