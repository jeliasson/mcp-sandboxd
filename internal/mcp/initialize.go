package mcp

import "encoding/json"

// Minimal MCP initialize support for clients that require a handshake.

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

func handleInitialize(params json.RawMessage) (any, *Error) {
	var p initializeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, NewError(-32602, "invalid_params", map[string]any{"error": err.Error()})
		}
	}

	proto := p.ProtocolVersion
	if proto == "" {
		proto = "2025-11-25"
	}

	// Advertise only what we implement.
	capabilities := map[string]any{
		"tools": map[string]any{
			"listChanged": false,
		},
		"resources": map[string]any{
			"subscribe":   false,
			"listChanged": false,
		},
		"prompts": map[string]any{
			"listChanged": false,
		},
	}

	return map[string]any{
		"protocolVersion": proto,
		"capabilities":    capabilities,
		"serverInfo": map[string]any{
			"name":    "mcp-sandboxd",
			"version": "dev",
		},
	}, nil
}
