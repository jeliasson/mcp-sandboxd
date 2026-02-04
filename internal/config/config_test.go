package config

import "testing"

func TestLoadRequiresSandboxImage(t *testing.T) {
	t.Setenv("SANDBOX_IMAGE", "")
	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadDefaultsAndOverrides(t *testing.T) {
	t.Setenv("SANDBOX_IMAGE", "example:sandbox")
	t.Setenv("PORT", "9090")
	t.Setenv("MCP_PATH", "/mcp2")
	t.Setenv("DEFAULT_TTL_SECONDS", "10")
	t.Setenv("MAX_TTL_SECONDS", "20")
	t.Setenv("DENYLIST_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.MCPPath != "/mcp2" {
		t.Fatalf("expected /mcp2, got %q", cfg.MCPPath)
	}
	if cfg.DefaultTTLSeconds != 10 || cfg.MaxTTLSeconds != 20 {
		t.Fatalf("unexpected ttl: %d %d", cfg.DefaultTTLSeconds, cfg.MaxTTLSeconds)
	}
	if cfg.DenylistEnabled {
		t.Fatalf("expected denylist disabled")
	}
	if len(cfg.DenylistPatterns) != 0 {
		t.Fatalf("expected no patterns when disabled")
	}
}

func TestLoadMCPPathValidation(t *testing.T) {
	t.Setenv("SANDBOX_IMAGE", "example:sandbox")
	t.Setenv("MCP_PATH", "mcp")
	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadCapabilitiesDefaultAndNormalization(t *testing.T) {
	t.Setenv("SANDBOX_IMAGE", "example:sandbox")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.SandboxNetworkMode != "bridge" {
		t.Fatalf("expected network mode bridge, got %q", cfg.SandboxNetworkMode)
	}
	if len(cfg.SandboxCapAdd) == 0 {
		t.Fatalf("expected default cap add")
	}

	t.Setenv("SANDBOX_CAP_ADD", "cap_setuid,SETGID")
	cfg2, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg2.SandboxCapAdd) != 2 || cfg2.SandboxCapAdd[0] != "SETUID" || cfg2.SandboxCapAdd[1] != "SETGID" {
		t.Fatalf("unexpected caps: %#v", cfg2.SandboxCapAdd)
	}
}

func TestLoadCapabilitiesStrictValidation(t *testing.T) {
	t.Setenv("SANDBOX_IMAGE", "example:sandbox")
	t.Setenv("SANDBOX_CAP_ADD", "NOT_A_REAL_CAP")
	t.Setenv("SANDBOX_CAPS_STRICT", "true")
	t.Setenv("SANDBOX_CAPS_BYPASS_CHECK", "false")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}

	t.Setenv("SANDBOX_CAPS_BYPASS_CHECK", "true")
	_, err = Load()
	if err != nil {
		t.Fatalf("expected bypass to succeed, got: %v", err)
	}
}

func TestLoadNetworkModeHostRejected(t *testing.T) {
	t.Setenv("SANDBOX_IMAGE", "example:sandbox")
	t.Setenv("SANDBOX_NETWORK_MODE", "host")
	_, err := Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadToolDescriptionOverrides(t *testing.T) {
	t.Setenv("SANDBOX_IMAGE", "example:sandbox")
	t.Setenv("TOOL_DESC_RUN_SANDBOX_APPEND", "Extra")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.ToolDescOverridesEnabled {
		t.Fatalf("expected overrides enabled by default")
	}
	if cfg.ToolDescRunSandboxAppend != "Extra" {
		t.Fatalf("unexpected append: %q", cfg.ToolDescRunSandboxAppend)
	}
}
