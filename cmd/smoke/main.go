package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/jeliasson/mcp-sandboxd/internal/smoke"
)

func main() {
	var baseURL string
	var mcpPath string
	var identifier string
	var expectDebug bool

	flag.StringVar(&baseURL, "base-url", "http://localhost:8080", "Server base URL")
	flag.StringVar(&mcpPath, "mcp-path", "/mcp", "MCP path")
	flag.StringVar(&identifier, "identifier", "smoke", "Sandbox identifier")
	flag.BoolVar(&expectDebug, "expect-debug", false, "Expect /debug to be enabled")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := smoke.Run(ctx, smoke.Options{BaseURL: baseURL, MCPPath: mcpPath, Identifier: identifier, ExpectDebug: expectDebug}); err != nil {
		log.Fatalf("smoke failed: %v", err)
	}
	log.Printf("smoke OK")
}
