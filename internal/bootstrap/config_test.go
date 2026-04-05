package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAgentUser_FilePresent_ReturnsValue(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENT_USER"), []byte("goose-agent\n"), 0644); err != nil {
		t.Fatal(err)
	}

	user, err := ReadAgentUser(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != "goose-agent" {
		t.Errorf("expected %q, got %q", "goose-agent", user)
	}
}

func TestReadAgentUser_FileAbsent_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()

	user, err := ReadAgentUser(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != "" {
		t.Errorf("expected empty string, got %q", user)
	}
}

func TestReadAgentUser_FileWithWhitespace_ReturnsTrimmed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENT_USER"), []byte("  my-agent  \n\n"), 0644); err != nil {
		t.Fatal(err)
	}

	user, err := ReadAgentUser(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != "my-agent" {
		t.Errorf("expected %q, got %q", "my-agent", user)
	}
}
