package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jeliasson/mcp-sandboxd/internal/artifacts"
	"github.com/jeliasson/mcp-sandboxd/internal/config"
	"github.com/jeliasson/mcp-sandboxd/internal/metrics"
	"github.com/jeliasson/mcp-sandboxd/internal/sandbox"
	"github.com/jeliasson/mcp-sandboxd/internal/sse"
)

type Manager struct {
	cfg       config.Config
	executor  Executor
	sandboxes sandbox.API
	metrics   *metrics.Metrics

	denylist []*regexp.Regexp

	mu      sync.Mutex
	runs    map[string]*Run
	brokers map[string]*sse.Broker
	locks   map[string]*FIFOLock

	latestRunByIdentifier map[string]string
}

func NewManager(cfg config.Config, executor Executor, sandboxes sandbox.API, metrics *metrics.Metrics) *Manager {
	m := &Manager{
		cfg:                   cfg,
		executor:              executor,
		sandboxes:             sandboxes,
		metrics:               metrics,
		runs:                  map[string]*Run{},
		brokers:               map[string]*sse.Broker{},
		locks:                 map[string]*FIFOLock{},
		latestRunByIdentifier: map[string]string{},
	}

	if cfg.DenylistEnabled {
		for _, p := range cfg.DenylistPatterns {
			re, err := regexp.Compile(p)
			if err == nil {
				m.denylist = append(m.denylist, re)
			}
		}
	}

	return m
}

func (m *Manager) getLock(identifier string) *FIFOLock {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.locks[identifier]
	if !ok {
		l = &FIFOLock{}
		m.locks[identifier] = l
	}
	return l
}

func (m *Manager) getBroker(runID string) *sse.Broker {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.brokers[runID]
	if !ok {
		b = sse.NewBroker(200)
		m.brokers[runID] = b
	}
	return b
}

func (m *Manager) RunSandbox(ctx context.Context, args RunSandboxArgs) (any, error) {
	if err := normalizeRunArgs(&args); err != nil {
		return nil, err
	}
	if err := validateRunArgs(args); err != nil {
		return nil, err
	}

	ttl := args.Options.TTLSeconds
	if ttl <= 0 {
		ttl = m.cfg.DefaultTTLSeconds
	}
	if ttl > m.cfg.MaxTTLSeconds {
		ttl = m.cfg.MaxTTLSeconds
	}

	sbx, err := m.sandboxes.Ensure(ctx, args.Identifier, ttl)
	if err != nil {
		return nil, err
	}
	m.metrics.ActiveSandboxes.Set(float64(m.sandboxes.Count()))

	runID := uuid.NewString()
	run := &Run{
		RunID:              runID,
		Identifier:         args.Identifier,
		State:              StateQueued,
		CreatedAt:          time.Now().UTC(),
		SandboxContainerID: sbx.ContainerID,
		SandboxName:        sbx.Name,
		Done:               make(chan struct{}),
	}

	for i, c := range args.Commands {
		cwd := c.Cwd
		if cwd == "" {
			cwd = args.Options.DefaultCwd
		}
		if cwd == "" {
			cwd = "/workspace"
		}

		env := map[string]string{}
		for k, v := range args.Options.DefaultEnv.Map() {
			env[k] = v
		}
		for k, v := range c.Env.Map() {
			env[k] = v
		}

		run.Commands = append(run.Commands, &CommandResult{
			Index:  i,
			Shell:  c.Shell.String(),
			Argv:   c.Argv,
			Cwd:    cwd,
			Env:    env,
			Status: CommandQueued,
		})

	}

	b := m.getBroker(runID)

	m.mu.Lock()
	m.runs[runID] = run
	m.mu.Unlock()

	m.evictIfNeeded()

	go m.executeRun(runID, args, sbx, b)

	result := map[string]any{
		"run_id":       runID,
		"events_url":   fmt.Sprintf("%s/events?run_id=%s", m.cfg.MCPPath, runID),
		"status_url":   fmt.Sprintf("%s/runs/%s", m.cfg.MCPPath, runID),
		"identifier":   args.Identifier,
		"sandbox_name": sbx.Name,
	}

	await := m.cfg.DefaultAwaitCompletion
	if args.Options.AwaitCompletion != nil {
		await = *args.Options.AwaitCompletion
	}

	if await {
		waitMs := int(m.cfg.DefaultAwaitTimeout.Milliseconds())
		if args.Options.AwaitTimeoutMS != nil {
			waitMs = *args.Options.AwaitTimeoutMS
		}
		if waitMs <= 0 {
			waitMs = 30000
		}
		waitCtx, cancel := context.WithTimeout(ctx, time.Duration(waitMs)*time.Millisecond)
		defer cancel()
		select {
		case <-run.Done:
			final, _ := m.Get(runID)
			result["final"] = final
		case <-waitCtx.Done():
			result["final"] = nil
		}
	}

	return result, nil
}

func (m *Manager) executeRun(runID string, args RunSandboxArgs, sbx *sandbox.Sandbox, b *sse.Broker) {
	m.metrics.ActiveRuns.Inc()
	defer m.metrics.ActiveRuns.Dec()

	release := func() {}
	if args.Options.LockSandbox {
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		l := m.getLock(args.Identifier)
		releaseFn, err := l.Acquire(ctx)
		cancel()
		if err == nil {
			release = releaseFn
		}
	}
	defer release()

	// Ensure the sandbox is actually ready for exec.
	{
		readyCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		sbxReady, err := m.sandboxes.Ensure(readyCtx, args.Identifier, args.Options.TTLSeconds)
		cancel()
		if err != nil {
			m.mu.Lock()
			run := m.runs[runID]
			m.mu.Unlock()
			if run != nil {
				// Run might have been evicted; best-effort fail.
				ended := time.Now().UTC()
				m.mu.Lock()
				run.State = StateFailed
				run.EndedAt = ended
				run.Error = err.Error()
				m.mu.Unlock()
				m.metrics.RunsTotal.WithLabelValues(string(StateFailed)).Inc()
				b.Publish("run_failed", map[string]any{
					"run_id":   runID,
					"ended_at": ended.Format(time.RFC3339Nano),
					"error":    err.Error(),
				})
				close(run.Done)
			}
			return
		}
		sbx = sbxReady
		m.mu.Lock()
		run := m.runs[runID]
		if run != nil {
			run.SandboxContainerID = sbx.ContainerID
			run.SandboxName = sbx.Name
		}
		m.mu.Unlock()
	}

	now := time.Now().UTC()

	m.mu.Lock()
	run := m.runs[runID]
	run.State = StateRunning
	run.StartedAt = now
	m.mu.Unlock()

	b.Publish("run_started", map[string]any{
		"run_id":               runID,
		"identifier":           args.Identifier,
		"sandbox_container_id": sbx.ContainerID,
		"started_at":           now.Format(time.RFC3339Nano),
	})

	runFailed := false
	failRun := func(err error) {
		if runFailed {
			return
		}
		runFailed = true
		ended := time.Now().UTC()
		m.mu.Lock()
		run.State = StateFailed
		run.EndedAt = ended
		run.Error = err.Error()
		m.mu.Unlock()

		m.metrics.RunsTotal.WithLabelValues(string(StateFailed)).Inc()
		b.Publish("run_failed", map[string]any{
			"run_id":   runID,
			"ended_at": ended.Format(time.RFC3339Nano),
			"error":    err.Error(),
		})
	}

	var firstFailure error

	for i, cmd := range args.Commands {
		cmdRes := run.Commands[i]
		cmdRes.StartedAt = time.Now().UTC()

		commandText := cmd.Shell.String()
		if commandText == "" {
			commandText = strings.Join(cmd.Argv, " ")
		}
		if err := m.checkDenylist(commandText); err != nil {
			cmdRes.Status = CommandFailed
			cmdRes.EndedAt = time.Now().UTC()
			cmdRes.ExitCode = 1
			failRun(err)
			break
		}

		b.Publish("command_started", map[string]any{
			"run_id":     runID,
			"index":      i,
			"shell":      cmd.Shell.String(),
			"argv":       cmd.Argv,
			"cwd":        cmdRes.Cwd,
			"started_at": cmdRes.StartedAt.Format(time.RFC3339Nano),
		})

		exitCode, timedOut, stdoutInfo, stderrInfo, err := m.execCommand(context.Background(), sbx.ContainerID, args, runID, i, cmd, cmdRes, b)
		cmdRes.ExitCode = exitCode
		cmdRes.StdoutBytes = stdoutInfo.Bytes
		cmdRes.StderrBytes = stderrInfo.Bytes
		cmdRes.StdoutTruncated = stdoutInfo.Truncated
		cmdRes.StderrTruncated = stderrInfo.Truncated
		cmdRes.Stdout = stdoutInfo.Text
		cmdRes.Stderr = stderrInfo.Text
		cmdRes.EndedAt = time.Now().UTC()

		status := CommandCompleted
		if timedOut {
			status = CommandTimedOut
		} else if err != nil || exitCode != 0 {
			status = CommandFailed
		}
		cmdRes.Status = status

		b.Publish("command_finished", map[string]any{
			"run_id":           runID,
			"index":            i,
			"exit_code":        exitCode,
			"status":           status,
			"stdout_truncated": cmdRes.StdoutTruncated,
			"stderr_truncated": cmdRes.StderrTruncated,
			"ended_at":         cmdRes.EndedAt.Format(time.RFC3339Nano),
		})

		m.metrics.CommandDurationSec.Observe(cmdRes.EndedAt.Sub(cmdRes.StartedAt).Seconds())
		m.metrics.CommandsTotal.WithLabelValues(string(status)).Inc()

		failed := status != CommandCompleted
		if failed && !cmd.AllowFailure {
			if firstFailure == nil {
				if err != nil {
					firstFailure = err
				} else if timedOut {
					firstFailure = fmt.Errorf("command %d timed out", i)
				} else {
					firstFailure = fmt.Errorf("command %d failed with exit %d", i, exitCode)
				}
			}

			if !args.Options.ContinueOnError {
				failRun(firstFailure)
				break
			}
		}
	}

	m.extractArtifacts(runID, sbx, b)

	if !runFailed {
		ended := time.Now().UTC()
		m.mu.Lock()
		run.EndedAt = ended
		if firstFailure != nil {
			run.State = StateFailed
			run.Error = firstFailure.Error()
		} else {
			run.State = StateCompleted
		}
		m.mu.Unlock()

		if firstFailure != nil {
			m.metrics.RunsTotal.WithLabelValues(string(StateFailed)).Inc()
			b.Publish("run_failed", map[string]any{
				"run_id":   runID,
				"ended_at": ended.Format(time.RFC3339Nano),
				"error":    firstFailure.Error(),
			})
		} else {
			m.metrics.RunsTotal.WithLabelValues(string(StateCompleted)).Inc()
			b.Publish("run_finished", map[string]any{
				"run_id":   runID,
				"ended_at": ended.Format(time.RFC3339Nano),
			})
		}
	}

	close(run.Done)
}

type streamInfo struct {
	Bytes     int
	Truncated bool
	Text      string
}

func (m *Manager) execCommand(ctx context.Context, sandboxID string, args RunSandboxArgs, runID string, index int, cmd CommandInput, cmdRes *CommandResult, b *sse.Broker) (exitCode int, timedOut bool, stdout streamInfo, stderr streamInfo, execErr error) {
	timeout := m.cfg.DefaultCommandTimeout
	if cmd.TimeoutMS > 0 {
		timeout = time.Duration(cmd.TimeoutMS) * time.Millisecond
	}
	if timeout <= 0 {
		timeout = m.cfg.DefaultCommandTimeout
	}

	base := []string{}
	if cmd.Shell.String() != "" {
		base = []string{"bash", "-lc", cmd.Shell.String()}
	} else {
		base = append([]string{}, cmd.Argv...)
	}

	seconds := int(math.Ceil(timeout.Seconds()))
	wrapped := append([]string{"timeout", "--preserve-status", "--signal=KILL", fmt.Sprintf("%ds", seconds)}, base...)

	asUser := strings.ToLower(args.Options.AsUser)
	if asUser != "root" {
		asUser = "sandbox"
	}

	stdoutWriter := &eventWriter{broker: b, event: "command_stdout", runID: runID, index: index, maxBytes: m.cfg.DefaultStdoutMaxBytes, capture: true}
	stderrWriter := &eventWriter{broker: b, event: "command_stderr", runID: runID, index: index, maxBytes: m.cfg.DefaultStderrMaxBytes, capture: true}

	exitCode, execErr = m.executor.Exec(ctx, sandboxID, ExecParams{Cmd: wrapped, Cwd: cmdRes.Cwd, Env: cmdRes.Env, AsUser: asUser}, stdoutWriter, stderrWriter)
	stdout = streamInfo{Bytes: stdoutWriter.bytes, Truncated: stdoutWriter.truncated, Text: stdoutWriter.String()}
	stderr = streamInfo{Bytes: stderrWriter.bytes, Truncated: stderrWriter.truncated, Text: stderrWriter.String()}
	if execErr != nil {
		return exitCode, false, stdout, stderr, execErr
	}

	if exitCode == 124 || exitCode == 137 {
		timedOut = true
	}

	return exitCode, timedOut, stdout, stderr, nil
}

type eventWriter struct {
	broker *sse.Broker
	event  string
	runID  string
	index  int

	bytes     int
	maxBytes  int
	truncated bool

	capture bool
	buf     []byte
}

func (w *eventWriter) String() string {
	if !w.capture {
		return ""
	}
	return string(w.buf)
}

func (w *eventWriter) Write(p []byte) (int, error) {
	if w.maxBytes <= 0 {
		w.maxBytes = 1 << 20
	}

	remaining := w.maxBytes - w.bytes
	chunk := p
	truncatedNow := false
	if remaining <= 0 {
		w.truncated = true
		return len(p), nil
	}
	if len(chunk) > remaining {
		chunk = chunk[:remaining]
		w.truncated = true
		truncatedNow = true
	}

	w.bytes += len(chunk)
	if w.capture {
		w.buf = append(w.buf, chunk...)
	}
	w.broker.Publish(w.event, map[string]any{
		"run_id":    w.runID,
		"index":     w.index,
		"chunk":     string(chunk),
		"truncated": truncatedNow,
	})

	return len(p), nil
}

func (m *Manager) checkDenylist(text string) error {
	if len(m.denylist) == 0 {
		return nil
	}
	for _, re := range m.denylist {
		if re.MatchString(text) {
			return fmt.Errorf("denylist matched: %s", re.String())
		}
	}
	return nil
}

const maxIdentifierLen = 36

var identifierRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,36}$`)

func safeJoinUnderRoot(root string, elems ...string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	full := absRoot
	for _, e := range elems {
		full = filepath.Join(full, e)
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absFull)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes artifacts root")
	}
	return absFull, nil
}

func validateRunArgs(args RunSandboxArgs) error {
	if !identifierRe.MatchString(args.Identifier) {
		return fmt.Errorf("identifier must match ^[a-zA-Z0-9_-]{1,%d}$", maxIdentifierLen)
	}

	if len(args.Commands) == 0 {
		return errors.New("commands must be non-empty")
	}
	for i, c := range args.Commands {
		hasShell := strings.TrimSpace(c.Shell.String()) != ""

		hasArgv := len(c.Argv) > 0
		if hasShell == hasArgv {
			return fmt.Errorf("command %d must set exactly one of shell or argv", i)
		}
		if c.Cwd != "" && !strings.HasPrefix(c.Cwd, "/") {
			return fmt.Errorf("command %d cwd must be absolute", i)
		}
		if c.TimeoutMS < 0 || c.TimeoutMS > int((24*time.Hour).Milliseconds()) {
			return fmt.Errorf("command %d timeout_ms out of range", i)
		}
	}
	return nil
}

func (m *Manager) Get(runID string) (*Run, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[runID]
	if !ok {
		return nil, false
	}
	// Shallow copy; command structs are shared but effectively immutable after end.
	cp := *r
	return &cp, true
}

func (m *Manager) StatusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "run_id")
		run, ok := m.Get(runID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(run)
	})
}

func (m *Manager) SSEHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always present as SSE to satisfy strict clients.
		w.Header().Set("content-type", "text/event-stream")
		w.Header().Set("cache-control", "no-cache")
		w.Header().Set("connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		sendError := func(msg string) {
			_, _ = w.Write([]byte("event: error\n"))
			_, _ = w.Write([]byte("data: {\"error\": " + strconv.Quote(msg) + "}\n\n"))
			flusher.Flush()
		}

		runID := r.URL.Query().Get("run_id")
		if runID == "" {
			sendError("missing run_id")
			return
		}

		b := m.getBroker(runID)
		ch, cancel := b.Subscribe(200)
		defer cancel()

		// Initial comment to open stream.
		_, _ = w.Write([]byte(": ok\n\n"))
		flusher.Flush()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				_, _ = w.Write([]byte("event: " + msg.Event + "\n"))
				_, _ = w.Write([]byte("data: "))
				_, _ = w.Write(msg.Data)
				_, _ = w.Write([]byte("\n\n"))
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
}

func (m *Manager) ArtifactsHandler() http.Handler {
	root := m.cfg.ArtifactsDir
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Path format: /artifacts/{identifier}/{run_id}/{path...}
		// Also supports: /artifacts/{identifier}/latest/{path...}
		p := strings.TrimPrefix(r.URL.Path, "/artifacts/")
		p = filepath.Clean(p)
		if p == "." || strings.HasPrefix(p, "..") {
			http.NotFound(w, r)
			return
		}

		parts := strings.Split(filepath.ToSlash(p), "/")
		if len(parts) >= 2 && parts[1] == "latest" {
			identifier := parts[0]
			m.mu.Lock()
			runID := m.latestRunByIdentifier[identifier]
			m.mu.Unlock()
			if runID == "" {
				http.NotFound(w, r)
				return
			}
			parts[1] = runID
			p = filepath.Join(parts...)
		}

		full := filepath.Join(root, p)
		rel, err := filepath.Rel(root, full)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, full)
	})
}

func (m *Manager) extractArtifacts(runID string, sbx *sandbox.Sandbox, b *sse.Broker) {
	m.mu.Lock()
	run := m.runs[runID]
	m.mu.Unlock()

	dest, err := safeJoinUnderRoot(m.cfg.ArtifactsDir, run.Identifier, runID)
	if err != nil {
		m.mu.Lock()
		run.ArtifactsError = err.Error()
		m.mu.Unlock()
		return
	}

	_ = os.RemoveAll(dest)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		m.mu.Lock()
		run.ArtifactsError = err.Error()
		m.mu.Unlock()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	rc, err := m.executor.CopyArtifacts(ctx, sbx.ContainerID)
	if err != nil {
		m.mu.Lock()
		run.ArtifactsError = err.Error()
		m.mu.Unlock()
		return
	}
	defer rc.Close()

	if err := artifacts.ExtractTarToDirWithLimits(rc, dest, artifacts.ExtractLimits{
		MaxTotalBytes: int64(m.cfg.ArtifactsMaxExtractBytes),
		MaxFiles:      m.cfg.ArtifactsMaxExtractFiles,
		MaxFileBytes:  int64(m.cfg.ArtifactsMaxExtractFileBytes),
	}); err != nil {
		m.mu.Lock()
		run.ArtifactsError = err.Error()
		m.mu.Unlock()
		return
	}

	listed, err := artifacts.ListFiles(dest)
	if err != nil {
		m.mu.Lock()
		run.ArtifactsError = err.Error()
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	run.Artifacts = nil
	for _, f := range listed {
		run.Artifacts = append(run.Artifacts, ArtifactFile{Path: f.Path, Size: f.Size})
	}
	m.latestRunByIdentifier[run.Identifier] = runID
	m.mu.Unlock()

	b.Publish("artifacts_extracted", map[string]any{"run_id": runID, "count": len(listed)})
}

func (m *Manager) evictIfNeeded() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Age-based eviction (7 days) per spec.
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	for id, r := range m.runs {
		if !r.EndedAt.IsZero() && r.EndedAt.Before(cutoff) {
			delete(m.runs, id)
			if b, ok := m.brokers[id]; ok {
				b.Close()
				delete(m.brokers, id)
			}
		}
	}

	if len(m.runs) <= m.cfg.MaxRuns {
		return
	}

	// Naive eviction: remove oldest completed runs until under limit.
	type pair struct {
		id  string
		end time.Time
	}
	var ended []pair
	for id, r := range m.runs {
		if !r.EndedAt.IsZero() {
			ended = append(ended, pair{id: id, end: r.EndedAt})
		}
	}
	// Selection sort small-ish; avoids extra deps.
	for i := 0; i < len(ended); i++ {
		min := i
		for j := i + 1; j < len(ended); j++ {
			if ended[j].end.Before(ended[min].end) {
				min = j
			}
		}
		ended[i], ended[min] = ended[min], ended[i]
	}

	for len(m.runs) > m.cfg.MaxRuns && len(ended) > 0 {
		drop := ended[0]
		ended = ended[1:]
		delete(m.runs, drop.id)
		if b, ok := m.brokers[drop.id]; ok {
			b.Close()
			delete(m.brokers, drop.id)
		}
	}

	if len(m.runs) > m.cfg.MaxRuns {
		log.Printf("run retention: still over limit (%d > %d)", len(m.runs), m.cfg.MaxRuns)
	}
}
