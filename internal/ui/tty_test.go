package ui

import (
	"bytes"
	"os"
	"testing"
)

func TestIsInteractive_Buffer_ReturnsFalse(t *testing.T) {
	if IsInteractive(&bytes.Buffer{}) {
		t.Fatalf("expected IsInteractive(buffer) = false")
	}
}

func TestIsInteractive_NilWriter_ReturnsFalse(t *testing.T) {
	// A typed nil *os.File is still an io.Writer but has no fd. Under
	// term.IsTerminal a zero/invalid fd returns false — we mirror that
	// here so callers can safely pass the sentinel.
	var f *os.File
	if IsInteractive(f) {
		t.Fatalf("expected IsInteractive(nil *os.File) = false")
	}
}

func TestIsInteractive_PipeFD_ReturnsFalse(t *testing.T) {
	// os.Pipe produces *os.File ends that are not TTYs — exactly the
	// shape IsInteractive should reject.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })

	if IsInteractive(w) {
		t.Fatalf("expected IsInteractive(pipe writer) = false")
	}
}

func TestIsInteractive_NonFileWriter_ReturnsFalse(t *testing.T) {
	// An arbitrary io.Writer that isn't an *os.File can never be a TTY.
	w := struct {
		bytes.Buffer
	}{}
	if IsInteractive(&w) {
		t.Fatalf("expected IsInteractive(non-file writer) = false")
	}
}

func TestIsCI_GitHubActionsTrue_ReturnsTrue(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("CI", "")
	if !IsCI() {
		t.Fatalf("expected IsCI() = true when GITHUB_ACTIONS=true")
	}
}

func TestIsCI_CITrue_ReturnsTrue(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("CI", "true")
	if !IsCI() {
		t.Fatalf("expected IsCI() = true when CI=true")
	}
}

func TestIsCI_NeitherSet_ReturnsFalse(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("CI", "")
	if IsCI() {
		t.Fatalf("expected IsCI() = false when neither is set")
	}
}

func TestIsCI_NonTrueValues_ReturnFalse(t *testing.T) {
	// The check is strict — only "true" counts. A literal "1" or "yes"
	// does not set IsCI. This avoids false positives from unrelated
	// scripts that set CI=<something> for their own purposes.
	tests := []struct {
		gh string
		ci string
	}{
		{gh: "1", ci: ""},
		{gh: "false", ci: ""},
		{gh: "", ci: "1"},
		{gh: "", ci: "yes"},
	}
	for _, tc := range tests {
		t.Setenv("GITHUB_ACTIONS", tc.gh)
		t.Setenv("CI", tc.ci)
		if IsCI() {
			t.Errorf("expected IsCI() = false for GITHUB_ACTIONS=%q CI=%q", tc.gh, tc.ci)
		}
	}
}
