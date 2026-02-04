package mcp

import (
	"encoding/json"
	"log"
)

func logToolCall(enabled bool, name string, payload any) {
	if !enabled {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("toolcall %s", name)
		return
	}
	log.Printf("toolcall %s %s", name, string(b))
}
