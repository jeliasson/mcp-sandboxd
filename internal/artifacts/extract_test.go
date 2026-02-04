package artifacts

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTarToDirStripsBaseAndPreventsTraversal(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Docker cp-style prefix: "artifacts/".
	mustWriteFile(t, tw, "artifacts/hello.txt", []byte("hi"))
	mustWriteFile(t, tw, "artifacts/sub/nested.txt", []byte("ok"))

	// Attempted traversal should be ignored.
	mustWriteFile(t, tw, "artifacts/../evil.txt", []byte("no"))
	mustWriteFile(t, tw, "../outside.txt", []byte("no"))

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	dir := t.TempDir()
	if err := ExtractTarToDir(bytes.NewReader(buf.Bytes()), dir); err != nil {
		t.Fatalf("extract: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("read hello: %v", err)
	}
	if string(b) != "hi" {
		t.Fatalf("unexpected content: %q", string(b))
	}

	b, err = os.ReadFile(filepath.Join(dir, "sub", "nested.txt"))
	if err != nil {
		t.Fatalf("read nested: %v", err)
	}
	if string(b) != "ok" {
		t.Fatalf("unexpected content: %q", string(b))
	}

	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
		t.Fatalf("expected evil.txt to not be extracted")
	}
	if _, err := os.Stat(filepath.Join(dir, "outside.txt")); err == nil {
		t.Fatalf("expected outside.txt to not be extracted")
	}
}

func TestExtractTarToDirWithLimits_TooManyFiles(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	mustWriteFile(t, tw, "artifacts/a.txt", []byte("a"))
	mustWriteFile(t, tw, "artifacts/b.txt", []byte("b"))
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	dir := t.TempDir()
	err := ExtractTarToDirWithLimits(bytes.NewReader(buf.Bytes()), dir, ExtractLimits{MaxFiles: 1})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestExtractTarToDirWithLimits_FileTooLarge(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	mustWriteFile(t, tw, "artifacts/big.bin", bytes.Repeat([]byte{'x'}, 10))
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	dir := t.TempDir()
	err := ExtractTarToDirWithLimits(bytes.NewReader(buf.Bytes()), dir, ExtractLimits{MaxFileBytes: 5})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestExtractTarToDirWithLimits_TotalTooLarge(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	mustWriteFile(t, tw, "artifacts/a.bin", bytes.Repeat([]byte{'x'}, 6))
	mustWriteFile(t, tw, "artifacts/b.bin", bytes.Repeat([]byte{'y'}, 6))
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	dir := t.TempDir()
	err := ExtractTarToDirWithLimits(bytes.NewReader(buf.Bytes()), dir, ExtractLimits{MaxTotalBytes: 10})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestListFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "x.txt"), []byte("123"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("z"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	files, err := ListFiles(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func mustWriteFile(t *testing.T, tw *tar.Writer, name string, content []byte) {
	t.Helper()
	h := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
	if err := tw.WriteHeader(h); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
}
