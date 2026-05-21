package mount

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFirstTime_AllFilesCreated(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	withStubInstall(t, map[string]string{
		"RULEBOOK.md":            "# Rules",
		"skills/session-init.md": "# Session Init",
		"standards/go.md":        "# Go",
		"recipes/dev.yaml":       "recipe: dev",
		"concepts/philosophy.md": "# Philosophy",
	})

	err := RunFirstTime(&buf, root, "v2.0.0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .agents/RULEBOOK.md exists.
	if _, err := os.Stat(filepath.Join(root, ".agents", "RULEBOOK.md")); os.IsNotExist(err) {
		t.Error(".agents/RULEBOOK.md should exist")
	}

	// The .ai-version flat file was removed in #585; firsttime no longer
	// writes one. The clone is driven by the mocked CloneFunc, so we
	// cannot assert on .agents/.git metadata here — the download step's
	// success is evidence enough that the version flowed through.

	// .gitignore is no longer touched by first-time install — `.agents/` is
	// now a tracked submodule (when run in production), so gitignoring it
	// would actively break the install.

	// Verify CLAUDE.md.
	claude, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(claude), "@AGENTS.md") {
		t.Errorf("CLAUDE.md should reference @AGENTS.md, got: %s", claude)
	}

	// Verify AGENTS.md.
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agents), "@.agents/RULEBOOK.md") {
		t.Errorf("AGENTS.md should reference @.agents/RULEBOOK.md, got: %s", agents)
	}
	if !strings.Contains(string(agents), "@.agents/RULEBOOK.md") {
		t.Errorf("AGENTS.md should contain bootstrap import, got: %s", agents)
	}

	// Verify workflows.
	pipelinePath := filepath.Join(root, ".github", "workflows", "agentic-pipeline.yml")
	if _, err := os.Stat(pipelinePath); os.IsNotExist(err) {
		t.Error("agentic-pipeline.yml should exist")
	}
	pipeline, _ := os.ReadFile(pipelinePath)
	if !strings.Contains(string(pipeline), "@v2.0.0") {
		t.Errorf("pipeline workflow should reference @v2.0.0, got: %s", pipeline)
	}

	releasePath := filepath.Join(root, ".github", "workflows", "release.yml")
	if _, err := os.Stat(releasePath); os.IsNotExist(err) {
		t.Error("release.yml should exist")
	}
	release, _ := os.ReadFile(releasePath)
	if !strings.Contains(string(release), "@v2.0.0") {
		t.Errorf("release workflow should reference @v2.0.0, got: %s", release)
	}

	// Verify output messages.
	output := buf.String()
	if !strings.Contains(output, "Initialising AI-Native Delivery Framework") {
		t.Error("output should contain initialisation message")
	}
	if !strings.Contains(output, "AI Framework successfully mounted") {
		t.Error("output should contain success message")
	}
	if !strings.Contains(output, "Next steps") {
		t.Error("output should contain next steps")
	}
}

func TestRunFirstTime_PreservesExistingCLAUDEMD(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	// Create existing CLAUDE.md.
	_ = os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Custom CLAUDE.md\n"), 0o644)

	withStubInstall(t, map[string]string{
		"RULEBOOK.md": "# Rules",
	})

	err := RunFirstTime(&buf, root, "v2.0.0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve existing CLAUDE.md.
	data, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(data) != "# Custom CLAUDE.md\n" {
		t.Errorf("existing CLAUDE.md should be preserved, got: %s", data)
	}
}

func TestRunFirstTime_PreservesExistingAGENTSMD(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	// Create existing AGENTS.md.
	_ = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Custom AGENTS.md\n"), 0o644)

	withStubInstall(t, map[string]string{
		"RULEBOOK.md": "# Rules",
	})

	err := RunFirstTime(&buf, root, "v2.0.0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve existing AGENTS.md.
	data, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(data) != "# Custom AGENTS.md\n" {
		t.Errorf("existing AGENTS.md should be preserved, got: %s", data)
	}
}

func TestRunFirstTime_DownloadFailure(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	withStubInstallError(t, "network down")

	err := RunFirstTime(&buf, root, "v2.0.0", nil)
	if err == nil {
		t.Fatal("expected error on download failure")
	}
	if !strings.Contains(err.Error(), "installing framework") {
		t.Errorf("error should mention framework installation, got: %v", err)
	}
}

func TestRunFirstTime_NoConfirmationPrompt(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	withStubInstall(t, map[string]string{
		"RULEBOOK.md": "# Rules",
	})

	err := RunFirstTime(&buf, root, "v2.0.0", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should NOT contain any confirmation prompt.
	output := buf.String()
	if strings.Contains(output, "[y/N]") || strings.Contains(output, "confirm") {
		t.Error("first-time mount should not show confirmation prompt")
	}
}

func TestGenerateWorkflows_CreatesDirectory(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	err := GenerateWorkflows(&buf, root, "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Directory should be created.
	info, err := os.Stat(filepath.Join(root, ".github", "workflows"))
	if err != nil {
		t.Fatalf("workflows directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("workflows path should be a directory")
	}
}

func TestWorkflowTemplate_ContainsVersion(t *testing.T) {
	content := workflowTemplate("v2.1.0")
	if !strings.Contains(content, "@v2.1.0") {
		t.Errorf("workflow should reference @v2.1.0, got: %s", content)
	}
	if !strings.Contains(content, "agentic-pipeline.yml") {
		t.Error("workflow should reference pipeline workflow")
	}
	if strings.Contains(content, "agentic-pipeline-reusable.yml") {
		t.Error("workflow must not reference the legacy -reusable filename (collapsed in v2.3.0)")
	}
}

func TestReleaseWorkflowTemplate_ContainsVersion(t *testing.T) {
	content := releaseWorkflowTemplate("v2.1.0")
	if !strings.Contains(content, "@v2.1.0") {
		t.Errorf("release workflow should reference @v2.1.0, got: %s", content)
	}
}
