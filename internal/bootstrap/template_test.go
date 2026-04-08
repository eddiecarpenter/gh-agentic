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

	if len(tmpl.ResolvedStatusOptions()) != 7 {
		t.Fatalf("expected 7 status options, got %d", len(tmpl.ResolvedStatusOptions()))
	}

	expectedNames := []string{"Backlog", "Scoping", "Scheduled", "In Design", "In Development", "In Review", "Done"}
	for i, name := range expectedNames {
		if tmpl.ResolvedStatusOptions()[i].Name != name {
			t.Errorf("option[%d]: expected name %q, got %q", i, name, tmpl.ResolvedStatusOptions()[i].Name)
		}
	}

	// Verify color and description are populated.
	if tmpl.ResolvedStatusOptions()[0].Color != "GRAY" {
		t.Errorf("expected color GRAY for Backlog, got %q", tmpl.ResolvedStatusOptions()[0].Color)
	}
	if tmpl.ResolvedStatusOptions()[0].Description != "Prioritised, ready to start" {
		t.Errorf("unexpected description for Backlog: %q", tmpl.ResolvedStatusOptions()[0].Description)
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

func TestLoadProjectTemplate_StatusFieldFormat_ReturnsOptions(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// New format: statusField.options instead of top-level statusOptions.
	content := `{
  "statusField": {
    "options": [
      {"name": "Backlog", "color": "GRAY", "description": "Prioritised, ready to start"},
      {"name": "Done",    "color": "GREEN", "description": "Merged and closed"}
    ]
  }
}`
	if err := os.WriteFile(filepath.Join(baseDir, "project-template.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpl, err := LoadProjectTemplate(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	opts := tmpl.ResolvedStatusOptions()
	if len(opts) != 2 {
		t.Fatalf("expected 2 status options from statusField.options, got %d", len(opts))
	}
	if opts[0].Name != "Backlog" {
		t.Errorf("expected first option 'Backlog', got %q", opts[0].Name)
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
	if len(tmpl.ResolvedStatusOptions()) != 0 {
		t.Errorf("expected 0 status options, got %d", len(tmpl.ResolvedStatusOptions()))
	}
}
