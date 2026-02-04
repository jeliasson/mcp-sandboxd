package smoke

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Client struct {
	BaseURL   string
	MCPPath   string
	SessionID string
	HTTP      *http.Client
}

func NewClient(baseURL, mcpPath string) *Client {
	if mcpPath == "" {
		mcpPath = "/mcp"
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		MCPPath: mcpPath,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) MCPURL() string {
	u, _ := url.Parse(c.BaseURL)
	u.Path = path.Join(u.Path, c.MCPPath)
	return u.String()
}

func (c *Client) EventsURL(runID string) string {
	u, _ := url.Parse(c.BaseURL)
	u.Path = path.Join(u.Path, c.MCPPath, "events")
	q := u.Query()
	q.Set("run_id", runID)
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Client) StatusURL(runID string) string {
	u, _ := url.Parse(c.BaseURL)
	u.Path = path.Join(u.Path, c.MCPPath, "runs", runID)
	return u.String()
}

func (c *Client) ArtifactsURL(identifier, runID, p string) string {
	u, _ := url.Parse(c.BaseURL)
	u.Path = path.Join(u.Path, "artifacts", identifier, runID, p)
	return u.String()
}

func (c *Client) MetricsURL() string {
	return c.BaseURL + "/metrics"
}

func (c *Client) DebugURL() string {
	return c.BaseURL + "/debug"
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error"`
}

func (c *Client) Initialize(ctx context.Context) (json.RawMessage, error) {
	resp, err := c.call(ctx, rpcRequest{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "smoke", "version": "dev"},
	}})
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func (c *Client) ToolsList(ctx context.Context) (json.RawMessage, error) {
	resp, err := c.call(ctx, rpcRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{}})
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func (c *Client) ToolsCall(ctx context.Context, name string, args any) (json.RawMessage, error) {
	resp, err := c.call(ctx, rpcRequest{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": name, "arguments": args}})
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

func (c *Client) call(ctx context.Context, req rpcRequest) (*rpcResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.MCPURL(), bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("content-type", "application/json")
	if c.SessionID != "" {
		hreq.Header.Set("Mcp-Session-Id", c.SessionID)
	}

	hresp, err := c.HTTP.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer hresp.Body.Close()

	if sid := hresp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.SessionID = sid
	}

	body, _ := io.ReadAll(io.LimitReader(hresp.Body, 2<<20))
	if hresp.StatusCode < 200 || hresp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", hresp.StatusCode, string(body))
	}

	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w (%s)", err, string(body))
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d %s: %s", resp.Error.Code, resp.Error.Message, string(resp.Error.Data))
	}

	return &resp, nil
}
