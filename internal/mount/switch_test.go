package mount

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSwitch_ConfirmAndUpdate(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	// Set up existing state.
	_ = WriteAIVersion(root, "v1.5.6")
	_ = os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".github", "workflows", "agentic-pipeline.yml"),
		[]byte(workflowTemplate("v1.5.6")), 0o644)
	_ = os.WriteFile(filepath.Join(root, ".github", "workflows", "release.yml"),
		[]byte(releaseWorkflowTemplate("v1.5.6")), 0o644)

	fetch := fakeFetchTarball(map[string]string{
		"RULEBOOK.md": "# Rules v2.0.0",
	})

	confirm := func(prompt string) (bool, error) {
		return true, nil
	}

	err := RunSwitch(&buf, root, "v1.5.6", "v2.0.0", fetch, confirm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .ai-version updated.
	v, _ := ReadAIVersion(root)
	if v != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %q", v)
	}

	// Verify framework updated.
	data, _ := os.ReadFile(filepath.Join(root, ".ai", "RULEBOOK.md"))
	if !strings.Contains(string(data), "v2.0.0") {
		t.Errorf("RULEBOOK.md should contain v2.0.0 content, got: %s", data)
	}

	// Verify workflows updated.
	pipeline, _ := os.ReadFile(filepath.Join(root, ".github", "workflows", "agentic-pipeline.yml"))
	if !strings.Contains(string(pipeline), "@v2.0.0") {
		t.Errorf("pipeline workflow should reference @v2.0.0, got: %s", pipeline)
	}
	if strings.Contains(string(pipeline), "@v1.5.6") {
		t.Error("pipeline workflow should not still reference @v1.5.6")
	}

	release, _ := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if !strings.Contains(string(release), "@v2.0.0") {
		t.Errorf("release workflow should reference @v2.0.0, got: %s", release)
	}

	// Verify output.
	output := buf.String()
	if !strings.Contains(output, "v1.5.6 → v2.0.0") {
		t.Errorf("output should show version transition, got:\n%s", output)
	}
	if !strings.Contains(output, "AI Framework successfully mounted") {
		t.Error("output should show success message")
	}
}

func TestRunSwitch_Declined(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	_ = WriteAIVersion(root, "v1.5.6")

	fetch := fakeFetchTarball(map[string]string{
		"RULEBOOK.md": "# Rules v2.0.0",
	})

	confirm := func(prompt string) (bool, error) {
		return false, nil // Decline.
	}

	err := RunSwitch(&buf, root, "v1.5.6", "v2.0.0", fetch, confirm)
	if err != nil {
		t.Fatalf("expected nil error when declined, got: %v", err)
	}

	// Version should NOT have changed.
	v, _ := ReadAIVersion(root)
	if v != "v1.5.6" {
		t.Errorf("version should remain v1.5.6, got %q", v)
	}
}

func TestRunSwitch_NilConfirm(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	_ = WriteAIVersion(root, "v1.5.6")
	_ = os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".github", "workflows", "agentic-pipeline.yml"),
		[]byte(workflowTemplate("v1.5.6")), 0o644)

	fetch := fakeFetchTarball(map[string]string{
		"RULEBOOK.md": "# Rules v2.0.0",
	})

	// nil confirm should proceed without prompting.
	err := RunSwitch(&buf, root, "v1.5.6", "v2.0.0", fetch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v, _ := ReadAIVersion(root)
	if v != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %q", v)
	}
}

func TestRunSwitch_DownloadFailure(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer

	_ = WriteAIVersion(root, "v1.5.6")

	err := RunSwitch(&buf, root, "v1.5.6", "v2.0.0", fakeFetchError("network"), nil)
	if err == nil {
		t.Fatal("expected error on download failure")
	}
	if !strings.Contains(err.Error(), "switching framework") {
		t.Errorf("error should mention switching, got: %v", err)
	}
}

func TestUpdateWorkflowVersions_UpdatesTags(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)

	// Write a workflow with an old version tag.
	content := `name: Test
jobs:
  pipeline:
    uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline-reusable.yml@v1.0.0
    secrets: inherit
`
	_ = os.WriteFile(filepath.Join(workflowsDir, "test.yml"), []byte(content), 0o644)

	err := UpdateWorkflowVersions(root, "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(workflowsDir, "test.yml"))
	if !strings.Contains(string(data), "@v2.0.0") {
		t.Errorf("workflow should reference @v2.0.0, got: %s", data)
	}
	if strings.Contains(string(data), "@v1.0.0") {
		t.Error("workflow should not still reference @v1.0.0")
	}
}

func TestUpdateWorkflowVersions_NoWorkflowsDir(t *testing.T) {
	root := t.TempDir()
	// No .github/workflows/ directory.
	err := UpdateWorkflowVersions(root, "v2.0.0")
	if err != nil {
		t.Errorf("expected no error when workflows dir missing, got: %v", err)
	}
}

func TestUpdateWorkflowVersions_IgnoresNonYAML(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)

	// Write a non-YAML file.
	_ = os.WriteFile(filepath.Join(workflowsDir, "README.md"),
		[]byte("# Workflows\neddiecarpenter/gh-agentic/.github/workflows/x.yml@v1.0.0"), 0o644)

	err := UpdateWorkflowVersions(root, "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Non-YAML file should be unchanged.
	data, _ := os.ReadFile(filepath.Join(workflowsDir, "README.md"))
	if strings.Contains(string(data), "@v2.0.0") {
		t.Error("non-YAML file should not be modified")
	}
}

func TestUpdateWorkflowVersions_MultipleWorkflows(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)

	_ = os.WriteFile(filepath.Join(workflowsDir, "pipeline.yml"),
		[]byte(workflowTemplate("v1.0.0")), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"),
		[]byte(releaseWorkflowTemplate("v1.0.0")), 0o644)

	err := UpdateWorkflowVersions(root, "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pipeline, _ := os.ReadFile(filepath.Join(workflowsDir, "pipeline.yml"))
	if !strings.Contains(string(pipeline), "@v2.0.0") {
		t.Error("pipeline should reference @v2.0.0")
	}

	release, _ := os.ReadFile(filepath.Join(workflowsDir, "release.yml"))
	if !strings.Contains(string(release), "@v2.0.0") {
		t.Error("release should reference @v2.0.0")
	}
}
