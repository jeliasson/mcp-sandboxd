package sandbox

import "testing"

func TestContainerNameIsStableAndSafe(t *testing.T) {
	id := "Chat/With Weird Chars: ABC_123"
	name1 := containerName(id)
	name2 := containerName(id)
	if name1 != name2 {
		t.Fatalf("expected stable name, got %q and %q", name1, name2)
	}
	if len(name1) == 0 {
		t.Fatalf("expected non-empty name")
	}
	if len(name1) > 80 {
		t.Fatalf("expected reasonably short name, got %d", len(name1))
	}
	if name1[:8] != "mcp-sbx-" {
		t.Fatalf("unexpected prefix: %q", name1)
	}
}
