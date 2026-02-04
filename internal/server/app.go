package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	dockerclient "github.com/jeliasson/mcp-sandboxd/internal/client/docker"
	kubernetesclient "github.com/jeliasson/mcp-sandboxd/internal/client/kubernetes"
	"github.com/jeliasson/mcp-sandboxd/internal/config"
	"github.com/jeliasson/mcp-sandboxd/internal/debugui"
	"github.com/jeliasson/mcp-sandboxd/internal/mcp"
	"github.com/jeliasson/mcp-sandboxd/internal/metrics"
	"github.com/jeliasson/mcp-sandboxd/internal/reaper"
	"github.com/jeliasson/mcp-sandboxd/internal/runs"
	"github.com/jeliasson/mcp-sandboxd/internal/sandbox"
	sandboxkubernetes "github.com/jeliasson/mcp-sandboxd/internal/sandbox/kubernetes"
)

type App struct {
	cfg config.Config

	docker     *dockerclient.Client
	kubernetes *kubernetesclient.Client

	metricsReg *prometheus.Registry
	metrics    *metrics.Metrics

	sandboxes sandbox.API
	runs      *runs.Manager
	reaper    *reaper.Reaper

	router http.Handler
}

func NewApp(cfg config.Config) (*App, error) {
	app := &App{cfg: cfg}

	app.metricsReg = prometheus.NewRegistry()
	app.metricsReg.MustRegister(collectors.NewGoCollector())
	app.metricsReg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	app.metrics = metrics.New(app.metricsReg)

	var pinger mcp.Pinger
	var executor runs.Executor

	switch cfg.SandboxBackend {
	case "docker":
		d, err := dockerclient.New()
		if err != nil {
			return nil, err
		}
		app.docker = d

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = d.Ping(ctx)
		cancel()
		if err != nil {
			log.Printf("docker unavailable at startup: %v", err)
		}

		app.sandboxes = sandbox.NewManager(cfg, d)
		executor = runs.NewDockerExecutor(d)
		pinger = d
	case "kubernetes":
		kubernetesClient, err := kubernetesclient.New()
		if err != nil {
			return nil, err
		}
		app.kubernetes = kubernetesClient

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = kubernetesClient.Ping(ctx)
		cancel()
		if err != nil {
			log.Printf("kubernetes unavailable at startup: %v", err)
		}

		sbxMgr, err := sandboxkubernetes.NewManager(cfg, kubernetesClient.Clientset)
		if err != nil {
			return nil, err
		}
		app.sandboxes = sbxMgr

		ns := cfg.KubernetesSandboxNamespace
		if strings.TrimSpace(ns) == "" {
			ns = os.Getenv("POD_NAMESPACE")
			if strings.TrimSpace(ns) == "" {
				b, _ := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
				ns = strings.TrimSpace(string(b))
			}
			if strings.TrimSpace(ns) == "" {
				ns = "default"
			}
		}

		executor = runs.NewKubernetesExecutor(
			kubernetesClient.Config,
			kubernetesClient.Clientset,
			ns,
			cfg.KubernetesSandboxContainerName,
		)
		pinger = kubernetesClient
	default:
		return nil, fmt.Errorf("unsupported SANDBOX_BACKEND: %q", cfg.SandboxBackend)
	}

	app.runs = runs.NewManager(cfg, executor, app.sandboxes, app.metrics)
	app.reaper = reaper.New(app.sandboxes, app.metrics, cfg.ReaperInterval)
	app.reaper.Start()

	r := chi.NewRouter()

	// chi requires all middleware to be registered before routes.
	r.Use(httpLoggingMiddleware(cfg.LogHTTPRequests))
	if len(cfg.CorsAllowOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: cfg.CorsAllowOrigins,
			AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},

			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Mcp-Session-Id"},
			ExposedHeaders:   []string{"Mcp-Session-Id"},
			AllowCredentials: false,
			MaxAge:           300,
		}))

	}

	// Handle OPTIONS everywhere to avoid 405 on preflight.
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mcpHandler := mcp.NewHandler(cfg, app.runs, app.sandboxes, pinger)

	// MCP surface
	r.Method("GET", cfg.MCPPath, mcp.InfoHandler(cfg.MCPPath))
	r.Method("POST", cfg.MCPPath, mcpHandler)
	r.Method("DELETE", cfg.MCPPath, mcp.SessionDeleteHandler())
	r.Method("GET", cfg.MCPPath+"/events", app.runs.SSEHandler())
	r.Method("GET", cfg.MCPPath+"/runs/{run_id}", app.runs.StatusHandler())

	// Artifacts
	r.Method("GET", "/artifacts/*", app.runs.ArtifactsHandler())

	// Metrics
	r.Method("GET", "/metrics", promhttp.HandlerFor(app.metricsReg, promhttp.HandlerOpts{}))

	// Debug UI
	r.Method("GET", "/debug", debugui.Handler(cfg.DebugUIEnabled, cfg.MCPPath))

	app.router = r
	return app, nil
}

func (a *App) Router() http.Handler {
	return a.router
}

func (a *App) Close() {
	if a.reaper != nil {
		a.reaper.Stop()
	}
	if a.docker != nil {
		_ = a.docker.Close()
	}
}
