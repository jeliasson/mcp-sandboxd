package mcp

import "net/http"

// SessionDeleteHandler allows clients to close an HTTP MCP session.
// Many clients send DELETE /mcp after completing a session.
func SessionDeleteHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No server-side session state currently.
		w.WriteHeader(http.StatusNoContent)
	})
}
