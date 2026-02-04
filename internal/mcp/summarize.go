package mcp

import (
	"strings"

	"github.com/jeliasson/mcp-sandboxd/internal/runs"
)

func summarizeRunSandbox(args runs.RunSandboxArgs) map[string]any {
	summary := map[string]any{
		"identifier": args.Identifier,
		"commands":   len(args.Commands),
		"options": map[string]any{
			"ttl_seconds":       args.Options.TTLSeconds,
			"lock_sandbox":      args.Options.LockSandbox,
			"continue_on_error": args.Options.ContinueOnError,
			"await_completion":  args.Options.AwaitCompletion,
			"as_user":           args.Options.AsUser,
		},
	}

	var cmdPreview []any
	for i, c := range args.Commands {
		if i >= 3 {
			cmdPreview = append(cmdPreview, map[string]any{"more": len(args.Commands) - 3})
			break
		}
		text := c.Shell.String()
		if text == "" {
			text = strings.Join(c.Argv, " ")
		}
		text = strings.TrimSpace(text)
		if len(text) > 200 {
			text = text[:200] + "…"
		}
		cmdPreview = append(cmdPreview, map[string]any{
			"index":         i,
			"cwd":           c.Cwd,
			"timeout_ms":    c.TimeoutMS,
			"allow_failure": c.AllowFailure,
			"cmd":           text,
		})
	}
	if len(cmdPreview) > 0 {
		summary["command_preview"] = cmdPreview
	}
	return summary
}
