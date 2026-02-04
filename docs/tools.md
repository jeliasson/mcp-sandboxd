# Tools

This document describes the MCP tools exposed by `mcp-sandboxd` and the JSON schemas returned by `tools/list`.

Source of truth: `internal/mcp/tools.go`.

## `run_sandbox`

Run one or more commands in a sandbox keyed by `identifier`.

Notes:
- Each command should provide exactly one of `shell` (string) or `argv` (string array). (This constraint is described in the tool description but not enforced by the JSON schema.)
- `env` supports either an object map (`{"KEY":"VALUE"}`) or an array of `KEY=VALUE` strings.

### inputSchema

```json
{
  "type": "object",
  "properties": {
    "identifier": { "type": "string", "minLength": 1, "maxLength": 256 },
    "commands": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "properties": {
          "shell": { "type": "string" },
          "argv": { "type": "array", "items": { "type": "string" } },
          "cwd": { "type": "string" },
          "env": {
            "anyOf": [
              { "type": "object", "additionalProperties": { "type": "string" } },
              { "type": "array", "items": { "type": "string" } }
            ]
          },
          "timeout_ms": { "type": "integer", "minimum": 1 },
          "allow_failure": { "type": "boolean" }
        }
      }
    },
    "options": {
      "type": "object",
      "properties": {
        "ttl_seconds": { "type": "integer", "minimum": 1 },
        "default_cwd": { "type": "string" },
        "default_env": {
          "anyOf": [
            { "type": "object", "additionalProperties": { "type": "string" } },
            { "type": "array", "items": { "type": "string" } }
          ]
        },
        "as_user": {
          "type": "string",
          "enum": ["sandbox", "root"],
          "description": "Run commands as 'sandbox' (uid 1000) or 'root' (uid 0). Use 'root' for administrative operations such as apt install."
        },
        "continue_on_error": { "type": "boolean" },
        "lock_sandbox": { "type": "boolean" },
        "await_completion": { "type": "boolean", "default": false },
        "await_timeout_ms": { "type": "integer", "minimum": 1, "default": 30000 }
      }
    }
  },
  "required": ["identifier", "commands"]
}
```

## `delete_sandbox`

Delete a sandbox environment.

### inputSchema

```json
{
  "type": "object",
  "properties": {
    "identifier": { "type": "string", "minLength": 1, "maxLength": 256 }
  },
  "required": ["identifier"]
}
```

## `restart_sandbox`

Recreate a fresh sandbox for an identifier.

### inputSchema

```json
{
  "type": "object",
  "properties": {
    "identifier": { "type": "string", "minLength": 1, "maxLength": 256 },
    "options": {
      "type": "object",
      "properties": {
        "ttl_seconds": { "type": "integer", "minimum": 1 }
      }
    }
  },
  "required": ["identifier"]
}
```
