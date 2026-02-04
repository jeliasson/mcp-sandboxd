package mcp

import (
	"encoding/json"
	"time"

	"github.com/jeliasson/mcp-sandboxd/internal/config"
	"github.com/jeliasson/mcp-sandboxd/internal/runs"
)

func toolCallTimeout(cfg config.Config, params json.RawMessage) time.Duration {
	var p toolsCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return 0
	}
	if p.Name != "run_sandbox" {
		return 0
	}

	var args runs.RunSandboxArgs
	if err := json.Unmarshal(p.Arguments, &args); err != nil {
		return 0
	}

	await := cfg.DefaultAwaitCompletion
	if args.Options.AwaitCompletion != nil {
		await = *args.Options.AwaitCompletion
	}
	if !await {
		return 0
	}

	waitMs := int(cfg.DefaultAwaitTimeout.Milliseconds())
	if args.Options.AwaitTimeoutMS != nil {
		waitMs = *args.Options.AwaitTimeoutMS
	}
	if waitMs <= 0 {
		waitMs = 30000
	}

	// Add a small cushion for Docker operations/artifact extraction.
	return time.Duration(waitMs)*time.Millisecond + 15*time.Second
}
