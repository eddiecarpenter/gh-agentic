package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectTemplate_ValidJSON_ReturnsOptions(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{
  "statusOptions": [
    {"name": "Backlog",        "color": "GRAY",   "description": "Prioritised, ready to start"},
    {"name": "Scoping",        "color": "PURPLE", "description": "Requirement or feature being scoped"},
    {"name": "Scheduled",      "color": "BLUE",   "description": "Scoped and queued, waiting for design"},
    {"name": "In Design",      "color": "PINK",   "description": "Feature Design session active"},
    {"name": "In Development", "color": "YELLOW", "description": "Dev Session active"},
    {"name": "In Review",      "color": "ORANGE", "description": "PR open, awaiting review"},
    {"name": "Done",           "color": "GREEN",  "description": "Merged and closed"}
  ]
}`
	if err := os.WriteFile(filepath.Join(baseDir, "project-template.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpl, err := LoadProjectTemplate(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tmpl.StatusOptions) != 7 {
		t.Fatalf("expected 7 status options, got %d", len(tmpl.StatusOptions))
	}

	expectedNames := []string{"Backlog", "Scoping", "Scheduled", "In Design", "In Development", "In Review", "Done"}
	for i, name := range expectedNames {
		if tmpl.StatusOptions[i].Name != name {
			t.Errorf("option[%d]: expected name %q, got %q", i, name, tmpl.StatusOptions[i].Name)
		}
	}

	// Verify color and description are populated.
	if tmpl.StatusOptions[0].Color != "GRAY" {
		t.Errorf("expected color GRAY for Backlog, got %q", tmpl.StatusOptions[0].Color)
	}
	if tmpl.StatusOptions[0].Description != "Prioritised, ready to start" {
		t.Errorf("unexpected description for Backlog: %q", tmpl.StatusOptions[0].Description)
	}
}

func TestLoadProjectTemplate_MissingFile_ReturnsError(t *testing.T) {
	root := t.TempDir()

	_, err := LoadProjectTemplate(root)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadProjectTemplate_MalformedJSON_ReturnsError(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(baseDir, "project-template.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProjectTemplate(root)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestLoadProjectTemplate_EmptyOptions_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(baseDir, "project-template.json"), []byte(`{"statusOptions": []}`), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpl, err := LoadProjectTemplate(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tmpl.StatusOptions) != 0 {
		t.Errorf("expected 0 status options, got %d", len(tmpl.StatusOptions))
	}
}
