package runs

import (
	"encoding/json"
	"testing"
)

func TestEnvVarsUnmarshalObject(t *testing.T) {
	var e EnvVars
	if err := json.Unmarshal([]byte(`{"A":"1","B":"2"}`), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	m := e.Map()
	if m["A"] != "1" || m["B"] != "2" {
		t.Fatalf("unexpected: %#v", m)
	}
}

func TestEnvVarsUnmarshalArray(t *testing.T) {
	var e EnvVars
	if err := json.Unmarshal([]byte(`["A=1","B=2","NOPE"]`), &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	m := e.Map()
	if m["A"] != "1" || m["B"] != "2" {
		t.Fatalf("unexpected: %#v", m)
	}
}
