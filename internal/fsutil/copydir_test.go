package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyDir_Success(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create a file in src.
	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	// Verify file was copied.
	data, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatalf("expected copied file: %v", err)
	}
	if string(data) != "world" {
		t.Errorf("expected %q, got %q", "world", string(data))
	}
}

func TestCopyDir_NestedDirectories(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create nested structure: a/b/c.txt
	nested := filepath.Join(src, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "c.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "a", "b", "c.txt"))
	if err != nil {
		t.Fatalf("expected nested file: %v", err)
	}
	if string(data) != "deep" {
		t.Errorf("expected %q, got %q", "deep", string(data))
	}
}

func TestCopyDir_PreservesFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions not reliably testable on Windows")
	}

	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	// Create an executable file.
	if err := os.WriteFile(filepath.Join(src, "script.sh"), []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(dst, "script.sh"))
	if err != nil {
		t.Fatalf("expected copied file: %v", err)
	}

	// Check that the executable bit is preserved.
	if info.Mode()&0o111 == 0 {
		t.Errorf("expected executable permissions, got %v", info.Mode())
	}
}

func TestCopyDir_MissingSource_ReturnsError(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dest")

	err := CopyDir("/nonexistent/path", dst)
	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}
}

func TestCopyDir_SourceIsFile_ReturnsError(t *testing.T) {
	src := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(src, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "dest")

	err := CopyDir(src, dst)
	if err == nil {
		t.Fatal("expected error when source is a file, got nil")
	}
}

func TestCopyDir_MultipleFiles(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest")

	files := map[string]string{
		"a.txt": "alpha",
		"b.txt": "bravo",
		"c.txt": "charlie",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(src, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	for name, expected := range files {
		data, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil {
			t.Errorf("expected file %s: %v", name, err)
			continue
		}
		if string(data) != expected {
			t.Errorf("file %s: expected %q, got %q", name, expected, string(data))
		}
	}
}
