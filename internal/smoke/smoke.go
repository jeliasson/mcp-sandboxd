package smoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Options struct {
	BaseURL string
	MCPPath string

	Identifier  string
	ExpectDebug bool
}

func Run(ctx context.Context, opt Options) error {
	if opt.BaseURL == "" {
		opt.BaseURL = "http://localhost:8080"
	}
	if opt.MCPPath == "" {
		opt.MCPPath = "/mcp"
	}
	if opt.Identifier == "" {
		opt.Identifier = "smoke"
	}

	c := NewClient(opt.BaseURL, opt.MCPPath)

	// initialize handshake (many MCP clients require this)
	if _, err := c.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	// tools/list
	toolsRaw, err := c.ToolsList(ctx)
	if err != nil {
		return fmt.Errorf("tools/list failed: %w", err)
	}
	if !strings.Contains(string(toolsRaw), "run_sandbox") {
		return errors.New("tools/list missing run_sandbox")
	}

	// debug gating
	if err := checkDebug(ctx, c, opt.ExpectDebug); err != nil {
		return err
	}

	// run_sandbox (simple)
	runID1, containerID1, err := runAndWait(ctx, c, opt.Identifier, false, 5)
	if err != nil {
		return fmt.Errorf("run_sandbox #1 failed: %w", err)
	}

	// run_sandbox reuse
	runID2, containerID2, err := runAndWait(ctx, c, opt.Identifier, false, 5)
	if err != nil {
		return fmt.Errorf("run_sandbox #2 failed: %w", err)
	}
	if containerID1 == "" || containerID2 == "" || containerID1 != containerID2 {
		return fmt.Errorf("expected sandbox reuse, got container1=%s container2=%s", containerID1, containerID2)
	}

	// artifacts
	artRunID, _, err := runArtifacts(ctx, c, opt.Identifier)
	if err != nil {
		return fmt.Errorf("artifacts run failed: %w", err)
	}
	if err := verifyArtifactDownload(ctx, c, opt.Identifier, artRunID); err != nil {
		return err
	}

	// restart_sandbox -> new container id
	restartRaw, err := c.ToolsCall(ctx, "restart_sandbox", map[string]any{"identifier": opt.Identifier, "options": map[string]any{"ttl_seconds": 60}})
	if err != nil {
		return fmt.Errorf("restart_sandbox failed: %w", err)
	}
	var restartResp struct {
		ContainerID string `json:"container_id"`
	}
	_ = json.Unmarshal(restartRaw, &restartResp)
	if restartResp.ContainerID != "" && restartResp.ContainerID == containerID2 {
		return errors.New("expected restart to create new container id")
	}

	// delete_sandbox
	_, err = c.ToolsCall(ctx, "delete_sandbox", map[string]any{"identifier": opt.Identifier})
	if err != nil {
		return fmt.Errorf("delete_sandbox failed: %w", err)
	}

	// metrics
	if err := checkMetrics(ctx, c); err != nil {
		return err
	}

	_ = runID1
	_ = runID2
	return nil
}

func runAndWait(ctx context.Context, c *Client, identifier string, lock bool, ttlSeconds int) (runID string, containerID string, err error) {
	args := map[string]any{
		"identifier": identifier,
		"commands":   []any{map[string]any{"shell": "echo hello from sandbox"}},
		"options":    map[string]any{"lock_sandbox": lock, "ttl_seconds": ttlSeconds},
	}

	resRaw, err := c.ToolsCall(ctx, "run_sandbox", args)
	if err != nil {
		return "", "", err
	}
	var res struct {
		StructuredContent struct {
			RunID string `json:"run_id"`
		} `json:"structuredContent"`
	}
	if err := json.Unmarshal(resRaw, &res); err != nil {
		return "", "", fmt.Errorf("parse run_sandbox result: %w", err)
	}
	if res.StructuredContent.RunID == "" {
		return "", "", errors.New("missing run_id")
	}

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	runID = res.StructuredContent.RunID

	_, err = WaitForEvents(waitCtx, c.HTTP, c.EventsURL(runID), func(ev SSEEvent) bool {
		return ev.Event == "run_finished" || ev.Event == "run_failed"
	})
	if err != nil {
		return runID, "", fmt.Errorf("wait SSE: %w", err)
	}

	status, err := fetchStatus(ctx, c, runID)
	if err != nil {
		return runID, "", err
	}
	if status.State != "completed" && status.State != "failed" {
		return runID, "", fmt.Errorf("unexpected state: %s", status.State)
	}
	return runID, status.SandboxContainerID, nil
}

func runArtifacts(ctx context.Context, c *Client, identifier string) (runID string, containerID string, err error) {
	args := map[string]any{
		"identifier": identifier,
		"commands":   []any{map[string]any{"shell": "echo artifact-data > /artifacts/out.txt"}},
		"options":    map[string]any{"ttl_seconds": 60},
	}
	resRaw, err := c.ToolsCall(ctx, "run_sandbox", args)
	if err != nil {
		return "", "", err
	}
	var res struct {
		StructuredContent struct {
			RunID string `json:"run_id"`
		} `json:"structuredContent"`
	}
	_ = json.Unmarshal(resRaw, &res)
	if res.StructuredContent.RunID == "" {
		return "", "", errors.New("missing run_id")
	}

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	runID = res.StructuredContent.RunID

	_, err = WaitForEvents(waitCtx, c.HTTP, c.EventsURL(runID), func(ev SSEEvent) bool {
		return ev.Event == "artifacts_extracted" || ev.Event == "run_failed" || ev.Event == "run_finished"
	})
	if err != nil {
		return runID, "", err
	}

	status, err := fetchStatus(ctx, c, runID)
	if err != nil {
		return runID, "", err
	}
	return runID, status.SandboxContainerID, nil
}

type statusResp struct {
	State              string `json:"state"`
	SandboxContainerID string `json:"sandbox_container_id"`
	Identifier         string `json:"identifier"`
	Artifacts          []struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	} `json:"artifacts"`
}

func fetchStatus(ctx context.Context, c *Client, runID string) (statusResp, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.StatusURL(runID), nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return statusResp{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != 200 {
		return statusResp{}, fmt.Errorf("status http %d: %s", resp.StatusCode, string(b))
	}
	var out statusResp
	if err := json.Unmarshal(b, &out); err != nil {
		return statusResp{}, err
	}
	return out, nil
}

func verifyArtifactDownload(ctx context.Context, c *Client, identifier, runID string) error {
	status, err := fetchStatus(ctx, c, runID)
	if err != nil {
		return err
	}
	found := false
	for _, f := range status.Artifacts {
		if f.Path == "out.txt" {
			found = true
			break
		}
	}
	if !found {
		return errors.New("expected artifact out.txt")
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.ArtifactsURL(identifier, runID, "out.txt"), nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != 200 {
		return fmt.Errorf("artifact download http %d: %s", resp.StatusCode, string(b))
	}
	if !strings.Contains(string(b), "artifact-data") {
		return fmt.Errorf("unexpected artifact content: %q", string(b))
	}
	return nil
}

func checkMetrics(ctx context.Context, c *Client) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.MetricsURL(), nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return fmt.Errorf("metrics http %d", resp.StatusCode)
	}
	text := string(b)
	for _, name := range []string{
		"mcp_active_sandboxes",
		"mcp_active_runs",
		"mcp_runs_total",
		"mcp_commands_total",
		"mcp_command_duration_seconds",
		"mcp_reaper_deletes_total",
	} {
		if !strings.Contains(text, name) {
			return fmt.Errorf("missing metric %s", name)
		}
	}
	return nil
}

func checkDebug(ctx context.Context, c *Client, expectEnabled bool) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.DebugURL(), nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if expectEnabled {
		if resp.StatusCode != 200 {
			return fmt.Errorf("expected /debug 200, got %d", resp.StatusCode)
		}
		return nil
	}

	if resp.StatusCode != 404 {
		return fmt.Errorf("expected /debug 404 when disabled, got %d", resp.StatusCode)
	}
	return nil
}
