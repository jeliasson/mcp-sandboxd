package mcp

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
)

func logMCPRequest(enabled bool, req *Request) {
	if !enabled || req == nil {
		return
	}

	id := strings.TrimSpace(string(req.ID))
	if id == "" {
		id = "null"
	}
	if len(id) > 120 {
		id = id[:120] + "…"
	}

	params := strings.TrimSpace(string(req.Params))
	if len(params) > 400 {
		params = params[:400] + "…"
	}

	log.Printf("mcp method=%s id=%s params=%s", req.Method, id, strconv.Quote(params))
}

func logMCPUnknownMethod(enabled bool, method string) {
	if !enabled {
		return
	}
	b, _ := json.Marshal(map[string]any{"method": method})
	log.Printf("mcp unknown_method %s", string(b))
}
