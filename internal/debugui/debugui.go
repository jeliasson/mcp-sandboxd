package debugui

import (
	"embed"
	"io"
	"net/http"
	"strings"
)

//go:embed index.html
var content embed.FS

func Handler(enabled bool, mcpPath string) http.Handler {
	if !enabled {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := content.Open("index.html")
		if err != nil {
			http.Error(w, "missing debug asset", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "read debug asset", http.StatusInternalServerError)
			return
		}

		html := strings.ReplaceAll(string(b), "__MCP_PATH__", mcpPath)
		w.Header().Set("content-type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})
}
