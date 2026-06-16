package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldFederation_CreatesManifestAndDocs(t *testing.T) {
	dir := t.TempDir()
	if err := ScaffoldFederation(&bytes.Buffer{}, dir); err != nil {
		t.Fatalf("ScaffoldFederation: %v", err)
	}
	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("scaffolded FEDERATION.md should be valid: %v", err)
	}
	if len(fed.Domains) != 0 {
		t.Errorf("expected an empty manifest, got %d domains", len(fed.Domains))
	}
	for _, doc := range []string{"docs/SYSTEM_BRIEF.md", "docs/SYSTEM_ARCHITECTURE.md"} {
		if _, err := os.Stat(filepath.Join(dir, doc)); err != nil {
			t.Errorf("expected %s to be scaffolded: %v", doc, err)
		}
	}
}

func TestScaffoldFederation_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, validManifest)
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	custom := []byte("# custom brief\n")
	_ = os.WriteFile(filepath.Join(dir, "docs", "SYSTEM_BRIEF.md"), custom, 0o644)

	if err := ScaffoldFederation(&bytes.Buffer{}, dir); err != nil {
		t.Fatalf("ScaffoldFederation: %v", err)
	}
	fed, err := ReadFederation(dir)
	if err != nil || len(fed.Domains) != 2 {
		t.Fatalf("existing FEDERATION.md must not be overwritten: err=%v domains=%d", err, len(fed.Domains))
	}
	got, _ := os.ReadFile(filepath.Join(dir, "docs", "SYSTEM_BRIEF.md"))
	if string(got) != string(custom) {
		t.Error("existing SYSTEM_BRIEF.md must not be overwritten")
	}
}
