package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/jeliasson/mcp-sandboxd/internal/config"
	"github.com/jeliasson/mcp-sandboxd/internal/runs"
	"github.com/jeliasson/mcp-sandboxd/internal/sandbox"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	cfg       config.Config
	runs      *runs.Manager
	sandboxes sandbox.API
	pinger    Pinger
}

func NewHandler(cfg config.Config, runsMgr *runs.Manager, sandboxes sandbox.API, pinger Pinger) http.Handler {
	h := &Handler{cfg: cfg, runs: runsMgr, sandboxes: sandboxes, pinger: pinger}
	return http.HandlerFunc(h.serveHTTP)
}

const sessionHeader = "Mcp-Session-Id"

func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")

	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeResp(w, Response{JSONRPC: "2.0", Error: NewError(-32700, "read_error", nil)})
		return
	}
	defer r.Body.Close()

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeResp(w, Response{JSONRPC: "2.0", Error: NewError(-32700, "parse_error", nil)})
		return
	}
	logMCPRequest(h.cfg.LogMCPRequests, &req)
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	notification := len(req.ID) == 0 || string(req.ID) == "null"
	resp := Response{JSONRPC: req.JSONRPC, ID: req.ID}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	if req.Method == "tools/call" {
		// Allow long-polling tool calls when awaiting completion.
		if t := toolCallTimeout(h.cfg, req.Params); t > 30*time.Second {
			cancel()
			ctx, cancel = context.WithTimeout(r.Context(), t)
		}
	}
	defer cancel()

	// MCP HTTP session header support.
	// Many MCP clients expect the server to provide a session id on initialize and
	// send it on subsequent calls.
	sessionID := r.Header.Get(sessionHeader)

	switch req.Method {
	case "initialize":
		if sessionID == "" {
			sessionID = uuid.NewString()
		}
		w.Header().Set(sessionHeader, sessionID)

		result, rpcErr := handleInitialize(req.Params)
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	case "notifications/initialized":
		if sessionID != "" {
			w.Header().Set(sessionHeader, sessionID)
		}
		// Notification; no response expected.
		if notification {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		resp.Result = map[string]any{}
	case "ping":
		if sessionID != "" {
			w.Header().Set(sessionHeader, sessionID)
		}
		resp.Result = map[string]any{}
	case "tools/list":
		if sessionID != "" {
			w.Header().Set(sessionHeader, sessionID)
		}
		resp.Result = ToolsListResult(h.cfg)
	case "tools/call":
		if sessionID != "" {
			w.Header().Set(sessionHeader, sessionID)
		}
		result, rpcErr := h.toolsCall(ctx, req.Params)
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			// MCP requires CallToolResult with a `content` array.
			name := "tool"
			if m, ok := result.(map[string]any); ok {
				if s, _ := m["tool"].(string); s != "" {
					name = s
				}
			}
			resp.Result = newToolResult(name, result)
		}
	default:
		logMCPUnknownMethod(h.cfg.LogMCPRequests, req.Method)
		resp.Error = NewError(-32601, "method_not_found", map[string]any{"method": req.Method, "supported": []string{"initialize", "notifications/initialized", "ping", "tools/list", "tools/call"}})
	}

	if notification {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeResp(w, resp)
}

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (h *Handler) toolsCall(ctx context.Context, params json.RawMessage) (any, *Error) {
	var p toolsCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewError(-32602, "invalid_params", nil)
	}

	if h.pinger != nil {
		if err := h.pinger.Ping(ctx); err != nil {
			return nil, NewError(-32001, "BACKEND_UNAVAILABLE", map[string]any{"error": err.Error()})
		}
	}

	switch p.Name {
	case "run_sandbox":
		var args runs.RunSandboxArgs
		if err := json.Unmarshal(p.Arguments, &args); err != nil {
			return nil, NewError(-32602, "invalid_arguments", nil)
		}
		logToolCall(h.cfg.LogToolCalls, "run_sandbox", summarizeRunSandbox(args))
		out, err := h.runs.RunSandbox(ctx, args)
		if err != nil {
			return nil, NewError(-32000, "run_failed", map[string]any{"error": err.Error()})
		}
		if m, ok := out.(map[string]any); ok {
			m["tool"] = "run_sandbox"
		}
		return out, nil
	case "delete_sandbox":
		var args struct {
			Identifier string `json:"identifier"`
		}
		if err := json.Unmarshal(p.Arguments, &args); err != nil || args.Identifier == "" {
			return nil, NewError(-32602, "invalid_arguments", nil)
		}
		logToolCall(h.cfg.LogToolCalls, "delete_sandbox", map[string]any{"identifier": args.Identifier})
		if err := h.sandboxes.Delete(ctx, args.Identifier); err != nil {
			return nil, NewError(-32000, "delete_failed", map[string]any{"error": err.Error()})
		}
		return map[string]any{"tool": "delete_sandbox", "deleted": true, "identifier": args.Identifier}, nil
	case "restart_sandbox":
		var args struct {
			Identifier string `json:"identifier"`
			Options    struct {
				TTLSeconds int `json:"ttl_seconds"`
			} `json:"options"`
		}
		if err := json.Unmarshal(p.Arguments, &args); err != nil || args.Identifier == "" {
			return nil, NewError(-32602, "invalid_arguments", nil)
		}
		logToolCall(h.cfg.LogToolCalls, "restart_sandbox", map[string]any{"identifier": args.Identifier, "ttl_seconds": args.Options.TTLSeconds})
		sbx, err := h.sandboxes.Restart(ctx, args.Identifier, args.Options.TTLSeconds)
		if err != nil {
			return nil, NewError(-32000, "restart_failed", map[string]any{"error": err.Error()})
		}
		return map[string]any{"tool": "restart_sandbox", "sandbox": sbx}, nil
	default:
		return nil, NewError(-32601, "tool_not_found", map[string]any{"name": p.Name})
	}
}

func writeResp(w http.ResponseWriter, resp Response) {
	enc := json.NewEncoder(w)
	_ = enc.Encode(resp)
}
