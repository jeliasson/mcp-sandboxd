package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	ActiveSandboxes prometheus.Gauge
	ActiveRuns      prometheus.Gauge

	RunsTotal          *prometheus.CounterVec
	CommandsTotal      *prometheus.CounterVec
	CommandDurationSec prometheus.Histogram
	ReaperDeletesTotal prometheus.Counter
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{}

	m.ActiveSandboxes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_active_sandboxes",
		Help: "Number of active sandboxes tracked in-memory.",
	})
	m.ActiveRuns = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mcp_active_runs",
		Help: "Number of active runs currently executing.",
	})

	m.RunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcp_runs_total",
		Help: "Total runs by final status.",
	}, []string{"status"})

	m.CommandsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcp_commands_total",
		Help: "Total commands by final status.",
	}, []string{"status"})

	m.CommandDurationSec = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "mcp_command_duration_seconds",
		Help:    "Command duration in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	m.ReaperDeletesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mcp_reaper_deletes_total",
		Help: "Total sandbox deletions performed by reaper.",
	})

	reg.MustRegister(
		m.ActiveSandboxes,
		m.ActiveRuns,
		m.RunsTotal,
		m.CommandsTotal,
		m.CommandDurationSec,
		m.ReaperDeletesTotal,
	)

	return m
}
