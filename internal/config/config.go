package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port    int
	MCPPath string

	SandboxImage          string
	SandboxDockerfilePath string
	AutoBuildSandboxImage bool

	ArtifactsDir string

	ArtifactsMaxExtractBytes     int
	ArtifactsMaxExtractFiles     int
	ArtifactsMaxExtractFileBytes int

	DebugUIEnabled  bool
	LogToolCalls    bool
	LogHTTPRequests bool
	LogMCPRequests  bool

	DefaultTTLSeconds int
	MaxTTLSeconds     int

	ReaperInterval time.Duration

	DefaultCPUCores float64
	DefaultMemoryMB int
	DefaultPIDs     int

	DefaultCommandTimeout time.Duration
	DefaultStdoutMaxBytes int
	DefaultStderrMaxBytes int

	DefaultAwaitCompletion bool
	DefaultAwaitTimeout    time.Duration

	MaxRuns int

	CorsAllowOrigins []string

	DenylistEnabled  bool
	DenylistPatterns []string

	SandboxNetworkMode     string
	SandboxCapAdd          []string
	SandboxCapDrop         []string
	SandboxNoNewPrivileges bool
	SandboxCapsStrict      bool
	SandboxCapsBypassCheck bool

	SandboxBackend                 string
	KubernetesSandboxNamespace     string
	KubernetesSandboxContainerName string

	ToolDescOverridesEnabled       bool
	ToolDescRunSandboxOverride     string
	ToolDescRunSandboxAppend       string
	ToolDescDeleteSandboxOverride  string
	ToolDescDeleteSandboxAppend    string
	ToolDescRestartSandboxOverride string
	ToolDescRestartSandboxAppend   string
}

func Load() (Config, error) {
	cfg := Config{}

	cfg.Port = envInt("PORT", 8080)
	cfg.MCPPath = envString("MCP_PATH", "/mcp")
	if cfg.MCPPath == "" || cfg.MCPPath[0] != '/' {
		return Config{}, fmt.Errorf("MCP_PATH must start with '/': %q", cfg.MCPPath)
	}

	cfg.SandboxImage = envString("SANDBOX_IMAGE", "")
	if cfg.SandboxImage == "" {
		return Config{}, errors.New("SANDBOX_IMAGE is required")
	}

	cfg.SandboxDockerfilePath = envString("SANDBOX_DOCKERFILE_PATH", "docker/sandbox.Dockerfile")
	cfg.AutoBuildSandboxImage = envBool("AUTO_BUILD_SANDBOX_IMAGE", true)

	cfg.SandboxBackend = strings.ToLower(strings.TrimSpace(envString("SANDBOX_BACKEND", "docker")))
	if cfg.SandboxBackend == "" {
		cfg.SandboxBackend = "docker"
	}
	if cfg.SandboxBackend != "docker" && cfg.SandboxBackend != "kubernetes" {
		return Config{}, fmt.Errorf("SANDBOX_BACKEND must be docker or kubernetes, got %q", cfg.SandboxBackend)
	}
	cfg.KubernetesSandboxNamespace = strings.TrimSpace(envString("KUBERNETES_SANDBOX_NAMESPACE", ""))
	if cfg.KubernetesSandboxNamespace == "" {
		// Backward-compat: previous prefix.
		cfg.KubernetesSandboxNamespace = strings.TrimSpace(envString("K8S_SANDBOX_NAMESPACE", ""))
	}

	cfg.KubernetesSandboxContainerName = strings.TrimSpace(envString("KUBERNETES_SANDBOX_CONTAINER_NAME", ""))
	if cfg.KubernetesSandboxContainerName == "" {
		// Backward-compat: previous prefix.
		cfg.KubernetesSandboxContainerName = strings.TrimSpace(envString("K8S_SANDBOX_CONTAINER_NAME", "sandbox"))
	}
	if cfg.KubernetesSandboxContainerName == "" {
		cfg.KubernetesSandboxContainerName = "sandbox"
	}

	cfg.ArtifactsDir = envString("ARTIFACTS_DIR", "./data/artifacts")

	// Artifact extraction limits (defense-in-depth against disk/inode exhaustion).
	cfg.ArtifactsMaxExtractBytes = envInt("ARTIFACTS_MAX_EXTRACT_BYTES", 256<<20) // 256 MiB
	cfg.ArtifactsMaxExtractFiles = envInt("ARTIFACTS_MAX_EXTRACT_FILES", 5000)
	cfg.ArtifactsMaxExtractFileBytes = envInt("ARTIFACTS_MAX_EXTRACT_FILE_BYTES", 64<<20) // 64 MiB
	if cfg.ArtifactsMaxExtractBytes < 0 {
		cfg.ArtifactsMaxExtractBytes = 0
	}
	if cfg.ArtifactsMaxExtractFiles < 0 {
		cfg.ArtifactsMaxExtractFiles = 0
	}
	if cfg.ArtifactsMaxExtractFileBytes < 0 {
		cfg.ArtifactsMaxExtractFileBytes = 0
	}

	cfg.DebugUIEnabled = envBool("DEBUG_UI_ENABLED", false)
	cfg.LogToolCalls = envBool("LOG_TOOLCALLS", false)
	cfg.LogHTTPRequests = envBool("LOG_HTTP_REQUESTS", false)
	cfg.LogMCPRequests = envBool("LOG_MCP_REQUESTS", false)

	cfg.DefaultTTLSeconds = envInt("DEFAULT_TTL_SECONDS", 3600)
	cfg.MaxTTLSeconds = envInt("MAX_TTL_SECONDS", 604800)
	if cfg.MaxTTLSeconds <= 0 {
		cfg.MaxTTLSeconds = 604800
	}
	if cfg.DefaultTTLSeconds <= 0 {
		cfg.DefaultTTLSeconds = 3600
	}

	cfg.ReaperInterval = time.Duration(envInt("REAPER_INTERVAL_MS", 5000)) * time.Millisecond
	if cfg.ReaperInterval <= 0 {
		cfg.ReaperInterval = 5 * time.Second
	}

	cfg.DefaultCPUCores = envFloat("DEFAULT_CPU_CORES", 1)
	if cfg.DefaultCPUCores <= 0 {
		cfg.DefaultCPUCores = 1
	}

	cfg.DefaultMemoryMB = envInt("DEFAULT_MEMORY_MB", 1024)
	if cfg.DefaultMemoryMB <= 0 {
		cfg.DefaultMemoryMB = 1024
	}

	cfg.DefaultPIDs = envInt("DEFAULT_PIDS", 256)
	if cfg.DefaultPIDs <= 0 {
		cfg.DefaultPIDs = 256
	}

	cfg.DefaultCommandTimeout = time.Duration(envInt("DEFAULT_COMMAND_TIMEOUT_MS", 120000)) * time.Millisecond
	if cfg.DefaultCommandTimeout <= 0 {
		cfg.DefaultCommandTimeout = 120 * time.Second
	}

	cfg.DefaultStdoutMaxBytes = envInt("DEFAULT_STDOUT_MAX_BYTES", 1048576)
	cfg.DefaultStderrMaxBytes = envInt("DEFAULT_STDERR_MAX_BYTES", 1048576)

	cfg.DefaultAwaitCompletion = envBool("DEFAULT_AWAIT_COMPLETION", false)
	cfg.DefaultAwaitTimeout = time.Duration(envInt("DEFAULT_AWAIT_TIMEOUT_MS", 30000)) * time.Millisecond
	if cfg.DefaultStdoutMaxBytes <= 0 {
		cfg.DefaultStdoutMaxBytes = 1048576
	}
	if cfg.DefaultStderrMaxBytes <= 0 {
		cfg.DefaultStderrMaxBytes = 1048576
	}

	cfg.MaxRuns = envInt("MAX_RUNS", 10000)
	if cfg.MaxRuns <= 0 {
		cfg.MaxRuns = 10000
	}

	corsAllow := strings.TrimSpace(envString("CORS_ALLOW_ORIGINS", ""))
	if corsAllow != "" {
		parts := strings.Split(corsAllow, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.CorsAllowOrigins = append(cfg.CorsAllowOrigins, p)
			}
		}
	}

	cfg.DenylistEnabled = envBool("DENYLIST_ENABLED", true)
	patterns := strings.TrimSpace(envString("DENYLIST_PATTERNS", ""))
	if patterns == "" {
		cfg.DenylistPatterns = []string{`\bdocker\b`, `\bmount\b`, `\bmodprobe\b`, `\bnsenter\b`}
	} else {
		parts := strings.Split(patterns, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.DenylistPatterns = append(cfg.DenylistPatterns, p)
			}
		}
	}
	if !cfg.DenylistEnabled {
		cfg.DenylistPatterns = nil
	}

	cfg.SandboxNetworkMode = strings.TrimSpace(envString("SANDBOX_NETWORK_MODE", "bridge"))
	if cfg.SandboxNetworkMode == "" {
		cfg.SandboxNetworkMode = "bridge"
	}
	if strings.EqualFold(cfg.SandboxNetworkMode, "host") {
		return Config{}, errors.New("SANDBOX_NETWORK_MODE=host is not supported")
	}

	cfg.SandboxNoNewPrivileges = envBool("SANDBOX_NO_NEW_PRIVILEGES", true)
	cfg.SandboxCapsStrict = envBool("SANDBOX_CAPS_STRICT", true)
	cfg.SandboxCapsBypassCheck = envBool("SANDBOX_CAPS_BYPASS_CHECK", false)

	capDropRaw := strings.TrimSpace(envString("SANDBOX_CAP_DROP", "ALL"))
	if capDropRaw == "" {
		capDropRaw = "ALL"
	}
	capAddRaw := strings.TrimSpace(envString("SANDBOX_CAP_ADD", ""))

	capDrop, err := parseCapabilitiesCSV(capDropRaw, cfg.SandboxCapsStrict, cfg.SandboxCapsBypassCheck)
	if err != nil {
		return Config{}, fmt.Errorf("SANDBOX_CAP_DROP: %w", err)
	}
	cfg.SandboxCapDrop = capDrop

	if capAddRaw == "" {
		cfg.SandboxCapAdd = []string{"SETUID", "SETGID", "CHOWN", "FOWNER", "DAC_OVERRIDE"}
	} else {
		capAdd, err := parseCapabilitiesCSV(capAddRaw, cfg.SandboxCapsStrict, cfg.SandboxCapsBypassCheck)
		if err != nil {
			return Config{}, fmt.Errorf("SANDBOX_CAP_ADD: %w", err)
		}
		cfg.SandboxCapAdd = capAdd
	}

	cfg.ToolDescOverridesEnabled = envBool("TOOL_DESC_OVERRIDES_ENABLED", true)
	cfg.ToolDescRunSandboxOverride = envString("TOOL_DESC_RUN_SANDBOX_OVERRIDE", "")
	cfg.ToolDescRunSandboxAppend = envString("TOOL_DESC_RUN_SANDBOX_APPEND", "")
	cfg.ToolDescDeleteSandboxOverride = envString("TOOL_DESC_DELETE_SANDBOX_OVERRIDE", "")
	cfg.ToolDescDeleteSandboxAppend = envString("TOOL_DESC_DELETE_SANDBOX_APPEND", "")
	cfg.ToolDescRestartSandboxOverride = envString("TOOL_DESC_RESTART_SANDBOX_OVERRIDE", "")
	cfg.ToolDescRestartSandboxAppend = envString("TOOL_DESC_RESTART_SANDBOX_APPEND", "")

	return cfg, nil
}

func (c Config) ListenAddr() string {
	return fmt.Sprintf(":%d", c.Port)
}

func envString(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envFloat(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func parseCapabilitiesCSV(raw string, strict bool, bypassCheck bool) ([]string, error) {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		cap := strings.TrimSpace(p)
		if cap == "" {
			continue
		}
		cap = strings.ToUpper(cap)
		cap = strings.TrimPrefix(cap, "CAP_")

		if cap != "ALL" {
			_, ok := knownCapabilities[cap]
			if !ok {
				if strict && !bypassCheck {
					return nil, fmt.Errorf("unknown capability: %q", cap)
				}
			}
		}

		if _, ok := seen[cap]; ok {
			continue
		}
		seen[cap] = struct{}{}
		out = append(out, cap)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

var knownCapabilities = map[string]struct{}{
	"AUDIT_CONTROL":      {},
	"AUDIT_READ":         {},
	"AUDIT_WRITE":        {},
	"BLOCK_SUSPEND":      {},
	"BPF":                {},
	"CHECKPOINT_RESTORE": {},
	"CHOWN":              {},
	"DAC_OVERRIDE":       {},
	"DAC_READ_SEARCH":    {},
	"FOWNER":             {},
	"FSETID":             {},
	"IPC_LOCK":           {},
	"IPC_OWNER":          {},
	"KILL":               {},
	"LEASE":              {},
	"LINUX_IMMUTABLE":    {},
	"MAC_ADMIN":          {},
	"MAC_OVERRIDE":       {},
	"MKNOD":              {},
	"NET_ADMIN":          {},
	"NET_BIND_SERVICE":   {},
	"NET_BROADCAST":      {},
	"NET_RAW":            {},
	"PERFMON":            {},
	"SETGID":             {},
	"SETFCAP":            {},
	"SETPCAP":            {},
	"SETUID":             {},
	"SYS_ADMIN":          {},
	"SYS_BOOT":           {},
	"SYS_CHROOT":         {},
	"SYS_MODULE":         {},
	"SYS_NICE":           {},
	"SYS_PACCT":          {},
	"SYS_PTRACE":         {},
	"SYS_RAWIO":          {},
	"SYS_RESOURCE":       {},
	"SYS_TIME":           {},
	"SYS_TTY_CONFIG":     {},
	"SYSLOG":             {},
	"WAKE_ALARM":         {},
}
