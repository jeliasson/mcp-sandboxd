package mcp

// This package implements a minimal MCP-over-HTTP JSON-RPC surface:
// - tools/list
// - tools/call
//
// The server intentionally keeps protocol handling lightweight and returns
// structured JSON results for tool calls.
