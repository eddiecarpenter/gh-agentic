package doctorv2

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
)

// setupHealthyRepo creates a temp directory that passes all framework checks.
func setupHealthyRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Framework files.
	aiDir := filepath.Join(root, ".ai")
	_ = os.MkdirAll(filepath.Join(aiDir, "skills"), 0o755)
	_ = os.MkdirAll(filepath.Join(aiDir, "standards"), 0o755)
	_ = os.WriteFile(filepath.Join(aiDir, "RULEBOOK.md"), []byte("# Rules"), 0o644)

	// .ai-version.
	_ = mount.WriteAIVersion(root, "v2.0.0")

	// .gitignore.
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".ai/\n"), 0o644)

	// Agent files.
	_ = os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# CLAUDE.md\n@AGENTS.md"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# AGENTS.md\n@.ai/RULEBOOK.md"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "LOCALRULES.md"), []byte("# Local Rules"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "README.md"), []byte("# Readme"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "skills"), 0o755)

	// Workflows.
	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline-reusable.yml@v2.0.0"), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/release-reusable.yml@v2.0.0"), 0o644)

	return root
}

func TestRunAllChecks_HealthyRepo(t *testing.T) {
	root := setupHealthyRepo(t)

	deps := CheckDeps{
		Root:         root,
		RepoFullName: "owner/repo",
		Owner:        "owner",
		RepoName:     "repo",
		OwnerType:    "User",
		Run: func(name string, args ...string) (string, error) {
			// Fake variable/secret checks.
			if name == "gh" && len(args) > 0 {
				if args[0] == "variable" && args[1] == "get" {
					return "some-value", nil
				}
				if args[0] == "secret" && args[1] == "list" {
					return "GOOSE_AGENT_PAT\tUpdated 2026-04-01", nil
				}
			}
			return "", nil
		},
		ReadCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return []byte(`{"token":"abc"}`), nil
		},
	}

	report := RunAllChecks(deps)

	if report.HasFailures() {
		var buf bytes.Buffer
		report.Render(&buf)
		t.Errorf("expected no failures on healthy repo, got:\n%s", buf.String())
	}

	// Verify all groups present.
	groupNames := make(map[string]bool)
	for _, g := range report.Groups {
		groupNames[g.Name] = true
	}
	for _, expected := range []string{"Repository", "Framework", "Agent files", "Workflows", "Variables & secrets"} {
		if !groupNames[expected] {
			t.Errorf("expected group %q", expected)
		}
	}
}

func TestRunAllChecks_MissingFramework(t *testing.T) {
	root := t.TempDir()
	// Empty repo — no .ai/, no .ai-version.

	deps := CheckDeps{
		Root:         root,
		RepoFullName: "owner/repo",
	}

	report := RunAllChecks(deps)

	if !report.HasFailures() {
		t.Error("expected failures for missing framework")
	}
}

func TestCheckFramework_Mounted(t *testing.T) {
	root := setupHealthyRepo(t)
	deps := CheckDeps{Root: root, RepoFullName: "owner/repo"}

	g := checkFramework(deps)

	for _, r := range g.Results {
		if r.Name == "ai-mounted" && r.Status != Pass {
			t.Errorf("expected ai-mounted to pass, got status %d: %s", r.Status, r.Message)
		}
		if r.Name == "ai-version" && r.Status != Pass {
			t.Errorf("expected ai-version to pass, got status %d: %s", r.Status, r.Message)
		}
		if r.Name == "gitignore" && r.Status != Pass {
			t.Errorf("expected gitignore to pass, got status %d: %s", r.Status, r.Message)
		}
	}
}

func TestCheckFramework_NotMounted(t *testing.T) {
	root := t.TempDir()
	deps := CheckDeps{Root: root}

	g := checkFramework(deps)

	for _, r := range g.Results {
		if r.Name == "ai-mounted" {
			if r.Status != Fail {
				t.Errorf("expected ai-mounted to fail, got status %d", r.Status)
			}
			if r.Remediation == "" {
				t.Error("expected remediation for missing mount")
			}
		}
	}
}

func TestCheckWorkflows_VersionMismatch(t *testing.T) {
	root := t.TempDir()
	_ = mount.WriteAIVersion(root, "v2.0.0")

	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline-reusable.yml@v1.0.0"), 0o644)

	deps := CheckDeps{Root: root}

	g := checkWorkflows(deps)

	foundMismatch := false
	for _, r := range g.Results {
		if r.Status == Fail && strings.Contains(r.Message, "mismatch") {
			foundMismatch = true
			if r.Remediation == "" {
				t.Error("expected remediation for version mismatch")
			}
		}
	}
	if !foundMismatch {
		t.Error("expected version mismatch failure")
	}
}

func TestCheckWorkflows_VersionMatch(t *testing.T) {
	root := t.TempDir()
	_ = mount.WriteAIVersion(root, "v2.0.0")

	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline-reusable.yml@v2.0.0"), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/release-reusable.yml@v2.0.0"), 0o644)

	deps := CheckDeps{Root: root}

	g := checkWorkflows(deps)

	for _, r := range g.Results {
		if r.Status == Fail {
			t.Errorf("expected all workflows to pass, but %s failed: %s", r.Name, r.Message)
		}
	}
}

func TestCheckAgentFiles_AllPresent(t *testing.T) {
	root := setupHealthyRepo(t)
	deps := CheckDeps{Root: root}

	g := checkAgentFiles(deps)

	for _, r := range g.Results {
		if r.Status == Fail {
			t.Errorf("expected no failures, but %s failed: %s", r.Name, r.Message)
		}
	}
}

func TestCheckAgentFiles_MissingOptional(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# Claude"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Agents"), 0o644)
	// No LOCALRULES.md, no skills/

	deps := CheckDeps{Root: root}
	g := checkAgentFiles(deps)

	warnings := 0
	for _, r := range g.Results {
		if r.Status == Warning {
			warnings++
		}
	}
	if warnings != 2 {
		t.Errorf("expected 2 warnings for missing optional files, got %d", warnings)
	}
}

func TestReport_Render_AllPass(t *testing.T) {
	report := &Report{
		Groups: []Group{
			{
				Name: "Test",
				Results: []CheckResult{
					{Name: "a", Status: Pass, Message: "All good"},
				},
			},
		},
	}

	var buf bytes.Buffer
	report.Render(&buf)
	output := buf.String()

	if !strings.Contains(output, "✓") {
		t.Error("expected check mark in output")
	}
	if strings.Contains(output, "✗") {
		t.Error("should not contain fail mark")
	}
	if strings.Contains(output, "failure") {
		t.Error("should not mention failures")
	}
}

func TestReport_Render_WithFailure(t *testing.T) {
	report := &Report{
		Groups: []Group{
			{
				Name: "Test",
				Results: []CheckResult{
					{Name: "a", Status: Fail, Message: "Broken", Remediation: "fix it"},
				},
			},
		},
	}

	var buf bytes.Buffer
	report.Render(&buf)
	output := buf.String()

	if !strings.Contains(output, "✗") {
		t.Error("expected fail mark")
	}
	if !strings.Contains(output, "fix it") {
		t.Error("expected remediation in output")
	}
	if !strings.Contains(output, "1 failure") {
		t.Errorf("expected '1 failure' summary, got:\n%s", output)
	}
}

func TestReport_Render_WithWarning(t *testing.T) {
	report := &Report{
		Groups: []Group{
			{
				Name: "Test",
				Results: []CheckResult{
					{Name: "a", Status: Warning, Message: "Optional missing"},
				},
			},
		},
	}

	var buf bytes.Buffer
	report.Render(&buf)
	output := buf.String()

	if !strings.Contains(output, "⚠") {
		t.Error("expected warning mark")
	}
	if !strings.Contains(output, "1 warning") {
		t.Errorf("expected '1 warning' summary, got:\n%s", output)
	}
}

func TestReport_HasFailures(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "pass", status: Pass, want: false},
		{name: "warning", status: Warning, want: false},
		{name: "fail", status: Fail, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := &Report{
				Groups: []Group{{Results: []CheckResult{{Status: tc.status}}}},
			}
			if got := report.HasFailures(); got != tc.want {
				t.Errorf("HasFailures() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReport_HasWarnings(t *testing.T) {
	report := &Report{
		Groups: []Group{{Results: []CheckResult{{Status: Warning}}}},
	}
	if !report.HasWarnings() {
		t.Error("expected HasWarnings() true")
	}
}
