package runs

import (
	"encoding/json"
	"testing"
)

func TestOptionalStringUnmarshal(t *testing.T) {
	var s OptionalString
	if err := json.Unmarshal([]byte(`"/bin/bash"`), &s); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if s.String() != "/bin/bash" {
		t.Fatalf("unexpected: %q", s.String())
	}

	if err := json.Unmarshal([]byte(`false`), &s); err != nil {
		t.Fatalf("unmarshal false: %v", err)
	}
	if s.String() != "" {
		t.Fatalf("expected empty")
	}
}
