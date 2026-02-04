package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewRegistersCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)
	// Instantiate label metrics so they appear in Gather().
	m.RunsTotal.WithLabelValues("completed").Inc()
	m.CommandsTotal.WithLabelValues("completed").Inc()
	m.CommandDurationSec.Observe(0.01)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	want := map[string]bool{
		"mcp_active_sandboxes":         true,
		"mcp_active_runs":              true,
		"mcp_runs_total":               true,
		"mcp_commands_total":           true,
		"mcp_command_duration_seconds": true,
		"mcp_reaper_deletes_total":     true,
	}

	for _, mf := range mfs {
		if mf.GetName() != "" {
			if _, ok := want[mf.GetName()]; ok {
				want[mf.GetName()] = false
			}
		}
	}

	for name, missing := range want {
		if missing {
			t.Fatalf("missing metric: %s", name)
		}
	}
}
