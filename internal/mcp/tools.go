package mcp

import (
	"strings"

	"github.com/jeliasson/mcp-sandboxd/internal/config"
)

const runSandboxBaseDescription = "Run commands in a sandbox keyed by identifier. Returns a run_id plus events_url (SSE) and status_url. To stream output, connect to events_url and read until event run_finished or run_failed. If SSE is unavailable, poll status_url until state is completed/failed. Set options.await_completion=true to include a final run summary inline (including captured stdout/stderr, subject to truncation). Command form: provide exactly one of shell (string) or argv (string[]). For compatibility with some clients, shell may be omitted/empty/false when using argv. Use options.as_user=\"root\" for administrative operations (e.g. apt install); use \"sandbox\" for non-privileged commands. Artifacts are copied from /artifacts after completion. Networking and other sandbox policy are configurable server-side."

const deleteSandboxBaseDescription = "Stop and remove the sandbox container for an identifier."

const restartSandboxBaseDescription = "Recreate an empty sandbox container for an identifier (delete + create)."

func ToolsListResult(cfg config.Config) map[string]any {
	runDesc := applyToolDescriptionOverride(cfg, runSandboxBaseDescription, cfg.ToolDescRunSandboxOverride, cfg.ToolDescRunSandboxAppend)
	deleteDesc := applyToolDescriptionOverride(cfg, deleteSandboxBaseDescription, cfg.ToolDescDeleteSandboxOverride, cfg.ToolDescDeleteSandboxAppend)
	restartDesc := applyToolDescriptionOverride(cfg, restartSandboxBaseDescription, cfg.ToolDescRestartSandboxOverride, cfg.ToolDescRestartSandboxAppend)

	return map[string]any{
		"tools": []any{
			map[string]any{
				"name":        "run_sandbox",
				"description": runDesc,
				"inputSchema": runSandboxSchema(),
			},
			map[string]any{
				"name":        "delete_sandbox",
				"description": deleteDesc,
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"identifier": map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
					},
					"required": []string{"identifier"},
				},
			},
			map[string]any{
				"name":        "restart_sandbox",
				"description": restartDesc,
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"identifier": map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
						"options": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"ttl_seconds": map[string]any{"type": "integer", "minimum": 1},
							},
						},
					},
					"required": []string{"identifier"},
				},
			},
		},
	}
}

func applyToolDescriptionOverride(cfg config.Config, base, override, appendText string) string {
	if !cfg.ToolDescOverridesEnabled {
		return base
	}
	out := strings.TrimSpace(base)
	if strings.TrimSpace(override) != "" {
		out = strings.TrimSpace(override)
	}
	if strings.TrimSpace(appendText) != "" {
		out = strings.TrimSpace(out + " " + strings.TrimSpace(appendText))
	}
	return out
}

func runSandboxSchema() map[string]any {
	cmd := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"shell": map[string]any{"type": "string"},
			"argv":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"cwd":   map[string]any{"type": "string"},
			"env": map[string]any{
				"anyOf": []any{
					map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
					map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
			"timeout_ms":    map[string]any{"type": "integer", "minimum": 1},
			"allow_failure": map[string]any{"type": "boolean"},
		},
	}

	opts := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ttl_seconds": map[string]any{"type": "integer", "minimum": 1},
			"default_cwd": map[string]any{"type": "string"},
			"default_env": map[string]any{
				"anyOf": []any{
					map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
					map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
			"as_user": map[string]any{
				"type":        "string",
				"enum":        []string{"sandbox", "root"},
				"description": "Run commands as 'sandbox' (uid 1000) or 'root' (uid 0). Use 'root' for administrative operations such as apt install.",
			},
			"continue_on_error": map[string]any{"type": "boolean"},
			"lock_sandbox":      map[string]any{"type": "boolean"},
			"await_completion":  map[string]any{"type": "boolean", "default": false},
			"await_timeout_ms":  map[string]any{"type": "integer", "minimum": 1, "default": 30000},
		},
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"identifier": map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
			"commands": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items":    cmd,
			},
			"options": opts,
		},
		"required": []string{"identifier", "commands"},
	}
}
