# Images

Published container images:

- **Server**: `ghcr.io/jeliasson/mcp-sandboxd`
- **Sandbox**: `ghcr.io/jeliasson/mcp-sandboxd-sandbox`

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

Notes:
- `:latest` is only published for releases.
- `mcp-sandboxd` does not interpret tags specially; tags only select which image version you deploy.
- `SANDBOX_IMAGE` decide which sandbox image to (build/)run.
