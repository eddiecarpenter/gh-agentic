package doctor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
)

// allLabelsListOutput returns fake `gh label list` output that contains every
// required pipeline label. Use it in test run fakes so the pipeline-labels
// check passes without injecting missing-label failures into unrelated tests.
func allLabelsListOutput() string {
	var sb strings.Builder
	for _, lbl := range requiredPipelineLabels {
		fmt.Fprintf(&sb, "%s\t%s\t#%s\n", lbl.Name, lbl.Description, lbl.Color)
	}
	return sb.String()
}

// setupAIGitRepo initialises aiDir as a git repo tagged at version,
// simulating a real git clone --depth 1 --branch <version>.
func setupAIGitRepo(t *testing.T, aiDir, version string) {
	t.Helper()
	_ = os.MkdirAll(aiDir, 0o755)
	_ = exec.Command("git", "-C", aiDir, "init").Run()
	_ = exec.Command("git", "-C", aiDir, "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "-C", aiDir, "config", "user.name", "Test").Run()
	_ = os.WriteFile(filepath.Join(aiDir, ".gitkeep"), []byte(""), 0o644)
	_ = exec.Command("git", "-C", aiDir, "add", ".").Run()
	_ = exec.Command("git", "-C", aiDir, "commit", "-m", "init").Run()
	_ = exec.Command("git", "-C", aiDir, "tag", version).Run()
}

// setupHealthyRepo creates a temp directory that passes all framework checks.
func setupHealthyRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Framework files.
	aiDir := filepath.Join(root, ".agents")
	_ = os.MkdirAll(filepath.Join(aiDir, "skills"), 0o755)
	_ = os.MkdirAll(filepath.Join(aiDir, "standards"), 0o755)
	_ = os.WriteFile(filepath.Join(aiDir, "RULEBOOK.md"), []byte("# Rules"), 0o644)
	setupAIGitRepo(t, aiDir, "v2.0.0")

	// The .ai-version flat file is gone (#585); .agents/.git metadata is the
	// only local version source, installed by setupAIGitRepo above.

	// .gitignore — submodule mode does NOT list .agents/, since the submodule
	// is tracked in the parent repo's index. A legacy `.agents/` line would
	// trigger a "legacy shallow-clone state" warning from the doctor.
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644)

	// Agent files.
	_ = os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# CLAUDE.md\n@AGENTS.md"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# AGENTS.md\n@.agents/RULEBOOK.md"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "LOCALRULES.md"), []byte("# Local Rules"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "README.md"), []byte("# Readme"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "skills"), 0o755)

	// Workflows.
	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline.yml@v2.0.0"), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/release.yml@v2.0.0"), 0o644)

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
		// ProjectID is populated by the resolver in production; the
		// doctor trusts it directly.
		ProjectID:         "PVT_healthy",
		FetchProjectTitle: func(id string) (string, error) { return "Healthy", nil },
		Run: func(name string, args ...string) (string, error) {
			// Fake variable/secret/label checks.
			if name == "gh" && len(args) > 0 {
				if args[0] == "variable" && args[1] == "get" {
					return "some-value", nil
				}
				if args[0] == "secret" && args[1] == "list" {
					return "PROJECT_PAT\tUpdated 2026-04-01\nPIPELINE_PAT\tUpdated 2026-04-01\nCLAUDE_CREDENTIALS_JSON\tUpdated 2026-04-01", nil
				}
				if args[0] == "label" && args[1] == "list" {
					return allLabelsListOutput(), nil
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
	for _, expected := range []string{"Repository", "Framework", "Agent files", "Workflows", "Variables & secrets", "Pipeline labels"} {
		if !groupNames[expected] {
			t.Errorf("expected group %q", expected)
		}
	}
}

func TestRunAllChecks_MissingFramework(t *testing.T) {
	root := t.TempDir()
	// Empty repo — no .agents/, no .ai-version.

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

func TestCheckWorkflows_MissingFile_Fail(t *testing.T) {
	// No .github/workflows directory at all — both files missing.
	root := t.TempDir()
	setupAIGitRepo(t, filepath.Join(root, ".agents"), "v2.0.0")
	deps := CheckDeps{Root: root}

	g := checkWorkflows(deps)

	// Both workflow files must produce a Fail (not Warning) with a non-empty
	// Remediation so that RepairPipeline can pick them up.
	for _, wf := range []string{"agentic-pipeline.yml", "release.yml"} {
		found := false
		for _, r := range g.Results {
			if r.Name == wf {
				found = true
				if r.Status != Fail {
					t.Errorf("%s: got status %d, want Fail", wf, r.Status)
				}
				if r.Remediation == "" {
					t.Errorf("%s: missing remediation hint", wf)
				}
				if !strings.Contains(r.Message, "not found") {
					t.Errorf("%s: message %q should contain 'not found'", wf, r.Message)
				}
			}
		}
		if !found {
			t.Errorf("no result for %s", wf)
		}
	}
}

func TestCheckWorkflows_VersionMismatch(t *testing.T) {
	root := t.TempDir()
	setupAIGitRepo(t, filepath.Join(root, ".agents"), "v2.0.0")
	// Version is stored in .agents/.git (installed by setupAIGitRepo); no
	// flat .ai-version file (#585).

	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline.yml@v1.0.0"), 0o644)

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
	setupAIGitRepo(t, filepath.Join(root, ".agents"), "v2.0.0")
	// Version is stored in .agents/.git (installed by setupAIGitRepo); no
	// flat .ai-version file (#585).

	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline.yml@v2.0.0"), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/release.yml@v2.0.0"), 0o644)

	deps := CheckDeps{Root: root}

	g := checkWorkflows(deps)

	for _, r := range g.Results {
		if r.Status == Fail {
			t.Errorf("expected all workflows to pass, but %s failed: %s", r.Name, r.Message)
		}
	}
}

// TestCheckWorkflows_InlinedNoFrameworkRefs documents the regression that
// previously broke `gh agentic check` for repos whose workflows are inlined
// (no `eddiecarpenter/gh-agentic/...@v...` reference). Such workflows do not
// need a framework version tag and must pass.
func TestCheckWorkflows_InlinedNoFrameworkRefs(t *testing.T) {
	root := t.TempDir()
	setupAIGitRepo(t, filepath.Join(root, ".agents"), "v2.1.0")
	// Version stored in .agents/.git; no flat file (#585).

	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	// Inlined workflow with only third-party action refs — no gh-agentic ref.
	inlined := `name: Agentic Pipeline
on: [push]
jobs:
  run:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd  # v6.0.2
`
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"), []byte(inlined), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"), []byte(inlined), 0o644)

	deps := CheckDeps{Root: root}
	g := checkWorkflows(deps)

	for _, r := range g.Results {
		if r.Status != Pass {
			t.Errorf("expected %s to Pass (inlined workflow, no framework ref), got %v: %s",
				r.Name, r.Status, r.Message)
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

// TestCheckAgentFiles_MissingMandatory_NoStaleMount is the AC-8 regression guard
// for the agent-files check. It verifies that when CLAUDE.md or AGENTS.md are
// absent, the Remediation hint does not reference the legacy 'gh agentic mount'
// command. The expected hint is 'gh agentic repair'.
func TestCheckAgentFiles_MissingMandatory_NoStaleMount(t *testing.T) {
	root := t.TempDir()
	// Deliberately omit CLAUDE.md and AGENTS.md so both fail checks fire.

	deps := CheckDeps{Root: root}
	g := checkAgentFiles(deps)

	for _, r := range g.Results {
		if r.Status == Fail && strings.Contains(r.Remediation, "gh agentic mount") {
			t.Errorf("check %q remediation contains stale 'gh agentic mount': %q", r.Name, r.Remediation)
		}
	}
}

// TestCheckWorkflows_VersionMismatch_NoStaleMount is the AC-8 regression guard
// for the workflow-version-mismatch path. It verifies the Remediation hint
// references 'gh agentic repair', not the legacy 'gh agentic mount'.
func TestCheckWorkflows_VersionMismatch_NoStaleMount(t *testing.T) {
	root := t.TempDir()
	setupAIGitRepo(t, filepath.Join(root, ".agents"), "v2.0.0")

	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.MkdirAll(workflowsDir, 0o755)
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline.yml@v1.0.0"), 0o644)

	deps := CheckDeps{Root: root}
	g := checkWorkflows(deps)

	for _, r := range g.Results {
		if r.Status == Fail && strings.Contains(r.Message, "mismatch") {
			if strings.Contains(r.Remediation, "gh agentic mount") {
				t.Errorf("workflow version-mismatch remediation contains stale 'gh agentic mount': %q", r.Remediation)
			}
			if !strings.Contains(r.Remediation, "gh agentic repair") {
				t.Errorf("workflow version-mismatch remediation should reference 'gh agentic repair', got: %q", r.Remediation)
			}
		}
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

// TestCheckProjectReachability_ProjectIDPresentAndReachable verifies the
// Pass case — variable set, GraphQL lookup returns a title.
func TestCheckProjectReachability_ProjectIDPresentAndReachable(t *testing.T) {
	deps := CheckDeps{
		RepoFullName:      "owner/repo",
		ProjectID:         "PVT_abc123",
		FetchProjectTitle: func(id string) (string, error) { return "My Project", nil },
	}
	g := checkProjectReachability(deps)
	if len(g.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(g.Results))
	}
	if g.Results[0].Status != Pass {
		t.Errorf("expected Pass, got %d (%s)", g.Results[0].Status, g.Results[0].Message)
	}
	if !strings.Contains(g.Results[0].Message, "My Project") {
		t.Errorf("expected title in message; got %q", g.Results[0].Message)
	}
}

// TestCheckProjectReachability_ProjectIDPresentButUnreachable verifies Fail
// with the auth-aware remediation.
func TestCheckProjectReachability_ProjectIDPresentButUnreachable(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "owner/repo",
		ProjectID:    "PVT_revoked",
		FetchProjectTitle: func(id string) (string, error) {
			return "", &fakeHTTPError{code: 403, msg: "Forbidden"}
		},
	}
	g := checkProjectReachability(deps)
	if len(g.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(g.Results))
	}
	if g.Results[0].Status != Fail {
		t.Errorf("expected Fail, got %d (%s)", g.Results[0].Status, g.Results[0].Message)
	}
	if !strings.Contains(g.Results[0].Remediation, "gh auth status") {
		t.Errorf("expected 'gh auth status' in remediation; got %q", g.Results[0].Remediation)
	}
}

// TestCheckProjectReachability_ProjectIDAbsent verifies the Fail path with
// the 'gh agentic project join' remediation.
func TestCheckProjectReachability_ProjectIDAbsent(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "owner/repo",
		ProjectID:    "",
		Run: func(name string, args ...string) (string, error) {
			return "", nil // variable lookup returns empty
		},
	}
	g := checkProjectReachability(deps)
	if len(g.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(g.Results))
	}
	if g.Results[0].Status != Fail {
		t.Errorf("expected Fail, got %d (%s)", g.Results[0].Status, g.Results[0].Message)
	}
	if !strings.Contains(g.Results[0].Remediation, "gh agentic project join") {
		t.Errorf("expected 'gh agentic project join' in remediation; got %q", g.Results[0].Remediation)
	}
}

// TestCheckProjectReachability_RunsForAllTopologies verifies the check is
// wired into the topology list for single and federation topologies.
func TestCheckProjectReachability_RunsForAllTopologies(t *testing.T) {
	topologies := []string{"single", "federation"}
	for _, topo := range topologies {
		t.Run(topo, func(t *testing.T) {
			deps := CheckDeps{Topology: topo}
			steps := checksForTopologyWithLabels(deps)
			found := false
			for _, s := range steps {
				if s.label == "Checking project reachability..." {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("topology %q should include project-reachability check", topo)
			}
		})
	}
}

// TestCheckProjectReachability_NoGraphQLClientWarns verifies the Warning
// path when FetchProjectTitle is not wired (defensive — production always
// wires it).
func TestCheckProjectReachability_NoGraphQLClientWarns(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "owner/repo",
		ProjectID:    "PVT_abc123",
		// FetchProjectTitle intentionally nil.
	}
	g := checkProjectReachability(deps)
	if len(g.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(g.Results))
	}
	if g.Results[0].Status != Warning {
		t.Errorf("expected Warning, got %d (%s)", g.Results[0].Status, g.Results[0].Message)
	}
}

// fakeHTTPError is a stand-in for a 4xx/5xx error returned from the GraphQL
// client in tests — implements the error interface, which is all that
// checkProjectReachability cares about.
type fakeHTTPError struct {
	code int
	msg  string
}

func (e *fakeHTTPError) Error() string { return e.msg }

// --- checkLabels tests (issue #686) ---

// TestCheckLabels_AllPresent verifies that when all required pipeline labels
// are returned by gh label list, every CheckResult is Pass.
func TestCheckLabels_AllPresent(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "owner/repo",
		Run: func(name string, args ...string) (string, error) {
			if name == "gh" && len(args) >= 2 && args[0] == "label" && args[1] == "list" {
				return allLabelsListOutput(), nil
			}
			return "", nil
		},
	}
	g := checkLabels(deps)
	if len(g.Results) != len(requiredPipelineLabels) {
		t.Fatalf("expected %d results, got %d: %+v", len(requiredPipelineLabels), len(g.Results), g.Results)
	}
	for _, r := range g.Results {
		if r.Status != Pass {
			t.Errorf("expected Pass for %q, got %v: %s", r.Name, r.Status, r.Message)
		}
	}
}

// TestCheckLabels_SomeMissing verifies that missing labels produce Fail results
// with a gh label create remediation string.
func TestCheckLabels_SomeMissing(t *testing.T) {
	// Only include the first label in the fake output — the rest are missing.
	partial := fmt.Sprintf("%s\t%s\t#%s\n",
		requiredPipelineLabels[0].Name,
		requiredPipelineLabels[0].Description,
		requiredPipelineLabels[0].Color,
	)
	deps := CheckDeps{
		RepoFullName: "owner/repo",
		Run: func(name string, args ...string) (string, error) {
			if name == "gh" && len(args) >= 2 && args[0] == "label" && args[1] == "list" {
				return partial, nil
			}
			return "", nil
		},
	}
	g := checkLabels(deps)
	passes, fails := 0, 0
	for _, r := range g.Results {
		switch r.Status {
		case Pass:
			passes++
		case Fail:
			fails++
			if !strings.HasPrefix(r.Remediation, "gh label create ") {
				t.Errorf("expected 'gh label create' remediation for %q, got %q",
					r.Name, r.Remediation)
			}
		}
	}
	if passes != 1 {
		t.Errorf("expected exactly 1 Pass result, got %d", passes)
	}
	missing := len(requiredPipelineLabels) - 1
	if fails != missing {
		t.Errorf("expected %d Fail results for missing labels, got %d", missing, fails)
	}
}

// TestCheckLabels_RunNil_Warns verifies that when deps.Run is nil the check
// returns a single Warning rather than panicking.
func TestCheckLabels_RunNil_Warns(t *testing.T) {
	deps := CheckDeps{RepoFullName: "owner/repo"} // Run intentionally nil
	g := checkLabels(deps)
	if len(g.Results) != 1 {
		t.Fatalf("expected 1 result for nil Run, got %d: %+v", len(g.Results), g.Results)
	}
	if g.Results[0].Status != Warning {
		t.Errorf("expected Warning for nil Run, got %v: %s", g.Results[0].Status, g.Results[0].Message)
	}
}

// TestCheckLabels_RunFails_Warns verifies that a gh invocation error produces
// a Warning rather than a cascade of Fail results for every label.
func TestCheckLabels_RunFails_Warns(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "owner/repo",
		Run: func(name string, args ...string) (string, error) {
			return "error: 403 Forbidden", fmt.Errorf("exit status 1")
		},
	}
	g := checkLabels(deps)
	if len(g.Results) != 1 || g.Results[0].Status != Warning {
		t.Errorf("expected single Warning on run failure, got %+v", g.Results)
	}
}

// TestCheckLabels_RunsForAllTopologies verifies the label check is wired into
// both supported topology variants.
func TestCheckLabels_RunsForAllTopologies(t *testing.T) {
	topologies := []string{"single", "federation"}
	for _, topo := range topologies {
		t.Run(topo, func(t *testing.T) {
			deps := CheckDeps{Topology: topo}
			steps := checksForTopologyWithLabels(deps)
			found := false
			for _, s := range steps {
				if s.label == "Checking pipeline labels..." {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("topology %q missing pipeline-labels check", topo)
			}
		})
	}
}

// TestCheckLabels_FrameworkSource verifies that the framework-source path also
// includes the pipeline-labels check (the gh-agentic repo itself needs labels).
func TestCheckLabels_FrameworkSource(t *testing.T) {
	deps := CheckDeps{FrameworkSource: true}
	steps := checksForTopologyWithLabels(deps)
	found := false
	for _, s := range steps {
		if s.label == "Checking pipeline labels..." {
			found = true
			break
		}
	}
	if !found {
		t.Error("framework source should include pipeline-labels check")
	}
}

// TestCheckLabels_VerificationLabels_PresentInRequiredSet confirms that
// the two labels added for Feature #746 (in-verification and compliance-verified)
// are registered in requiredPipelineLabels with the canonical attributes.
func TestCheckLabels_VerificationLabels_PresentInRequiredSet(t *testing.T) {
	tests := []struct {
		labelName   string
		wantColor   string
		wantDescSub string
	}{
		{
			labelName:   "in-verification",
			wantColor:   "d93f0b",
			// Substring chosen to survive description-wording refinements;
			// the protective intent is "the description mentions Compliance
			// Verify" — not a specific phrasing.
			wantDescSub: "Compliance Verify",
		},
		{
			labelName:   "compliance-verified",
			wantColor:   "0075ca",
			wantDescSub: "All acceptance criteria verified",
		},
	}
	for _, tc := range tests {
		t.Run(tc.labelName, func(t *testing.T) {
			found := false
			for _, lbl := range requiredPipelineLabels {
				if lbl.Name != tc.labelName {
					continue
				}
				found = true
				if lbl.Color != tc.wantColor {
					t.Errorf("label %q: color = %q, want %q", tc.labelName, lbl.Color, tc.wantColor)
				}
				if !strings.Contains(lbl.Description, tc.wantDescSub) {
					t.Errorf("label %q: description %q missing expected substring %q",
						tc.labelName, lbl.Description, tc.wantDescSub)
				}
			}
			if !found {
				t.Errorf("label %q not found in requiredPipelineLabels", tc.labelName)
			}
		})
	}
}

// TestCheckLabels_VerificationLabels_Missing_FailWithRemediation verifies that
// when in-verification or compliance-verified are absent from the repo, checkLabels
// produces Fail results with a valid gh label create remediation string.
func TestCheckLabels_VerificationLabels_Missing_FailWithRemediation(t *testing.T) {
	for _, labelName := range []string{"in-verification", "compliance-verified"} {
		t.Run(labelName, func(t *testing.T) {
			// Return all labels EXCEPT the one under test.
			var sb strings.Builder
			for _, lbl := range requiredPipelineLabels {
				if lbl.Name == labelName {
					continue
				}
				fmt.Fprintf(&sb, "%s\t%s\t#%s\n", lbl.Name, lbl.Description, lbl.Color)
			}
			partial := sb.String()

			deps := CheckDeps{
				RepoFullName: "owner/repo",
				Run: func(name string, args ...string) (string, error) {
					if name == "gh" && len(args) >= 2 && args[0] == "label" && args[1] == "list" {
						return partial, nil
					}
					return "", nil
				},
			}
			g := checkLabels(deps)

			found := false
			for _, r := range g.Results {
				if r.Name != "label:"+labelName {
					continue
				}
				found = true
				if r.Status != Fail {
					t.Errorf("label %q: expected Fail status, got %v (%s)", labelName, r.Status, r.Message)
				}
				if !strings.HasPrefix(r.Remediation, "gh label create ") {
					t.Errorf("label %q: expected 'gh label create' remediation, got %q", labelName, r.Remediation)
				}
				if !strings.Contains(r.Remediation, labelName) {
					t.Errorf("label %q: remediation %q does not contain label name", labelName, r.Remediation)
				}
			}
			if !found {
				t.Errorf("no CheckResult found with Name 'label:%s'", labelName)
			}
		})
	}
}

// TestContainsLabelName covers the first-token matching semantics that
// prevent false-positives when one label name is a prefix of another.
func TestContainsLabelName(t *testing.T) {
	out := "design-in-progress\tDesign active\t#d93f0b\nin-design\tIn Design\t#e4e669\n"
	tests := []struct {
		name string
		want bool
	}{
		{"design-in-progress", true},
		{"in-design", true},
		{"design", false}, // prefix-only — must not match
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsLabelName(out, tc.name); got != tc.want {
				t.Errorf("containsLabelName(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// --- checkVariable / checkSecret scope-fallback tests (task #531) ---

// ghRun is a minimal fake for deps.Run used in scope-fallback tests. It
// takes a routing table keyed by the scope flag ("--org" or "--repo") and
// returns the output for matching invocations. Any call that doesn't find
// a matching entry returns empty output (simulating "not found").
type ghRun struct {
	// Commands captured for assertions (deep copy of args).
	Calls [][]string
	// Outputs maps "kind|scopeFlag" -> output. kind is the first positional
	// arg from gh ("variable"/"secret") and scopeFlag is "--repo"/"--org".
	// Absence means empty output (not-found) without error.
	Outputs map[string]string
	// Errors maps the same key to an error, overriding Outputs on match.
	Errors map[string]error
}

func (r *ghRun) fn() func(string, ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		if name != "gh" {
			return "", nil
		}
		r.Calls = append(r.Calls, append([]string{}, args...))

		// kind is args[0]: "variable" or "secret".
		if len(args) < 2 {
			return "", nil
		}
		kind := args[0]

		// Scope flag is whichever of --repo/--org appears.
		scopeFlag := ""
		for _, a := range args {
			if a == "--repo" || a == "--org" {
				scopeFlag = a
				break
			}
		}
		key := kind + "|" + scopeFlag
		if err, ok := r.Errors[key]; ok {
			return "", err
		}
		if out, ok := r.Outputs[key]; ok {
			return out, nil
		}
		return "", nil
	}
}

func TestCheckSecret_SecretAtRepo_Pass(t *testing.T) {
	// Feature #824: secrets are always --repo scoped.
	r := &ghRun{
		Outputs: map[string]string{
			"secret|--repo": "PROJECT_PAT\tUpdated 2026-04-01",
		},
	}
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federation",
		Run: r.fn(),
	}
	res := checkSecret(deps, "PROJECT_PAT")
	if res.Status != Pass {
		t.Fatalf("got %d (%s), want Pass", res.Status, res.Message)
	}
	// Must NOT consult org scope.
	for _, c := range r.Calls {
		for _, a := range c {
			if a == "--org" {
				t.Errorf("org scope must never be consulted (calls: %v)", r.Calls)
			}
		}
	}
}

func TestCheckSecret_SecretMissing_FailWithRepoRemediation(t *testing.T) {
	// Feature #824: remediation always points to --repo, not --org.
	r := &ghRun{} // no outputs → repo listing returns empty
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federation",
		Run: r.fn(),
	}
	res := checkSecret(deps, "PROJECT_PAT")
	if res.Status != Fail {
		t.Fatalf("got %d (%s), want Fail", res.Status, res.Message)
	}
	wantHint := "gh secret set PROJECT_PAT --repo acme/domain"
	if res.Remediation != wantHint {
		t.Errorf("remediation: got %q, want %q", res.Remediation, wantHint)
	}
	// Must NOT consult org scope.
	for _, c := range r.Calls {
		for _, a := range c {
			if a == "--org" {
				t.Errorf("org scope must never be consulted (calls: %v)", r.Calls)
			}
		}
	}
}

func TestCheckSecret_Single_SecretAtRepo_Pass(t *testing.T) {
	r := &ghRun{
		Outputs: map[string]string{
			"secret|--repo": "PROJECT_PAT\tUpdated 2026-04-01",
		},
	}
	deps := CheckDeps{
		RepoFullName: "eddie/repo", Owner: "eddie", Topology: "single",
		Run: r.fn(),
	}
	res := checkSecret(deps, "PROJECT_PAT")
	if res.Status != Pass {
		t.Fatalf("got %d (%s), want Pass", res.Status, res.Message)
	}
	// Must NOT consult org scope under single.
	for _, c := range r.Calls {
		for _, a := range c {
			if a == "--org" {
				t.Errorf("single topology must not consult --org (calls: %v)", r.Calls)
			}
		}
	}
}

func TestCheckSecret_Single_SecretAtOrgOnly_Fail(t *testing.T) {
	// Under single we never consult org — the secret might exist there but
	// the check treats it as not-configured for this repo.
	r := &ghRun{
		Outputs: map[string]string{
			"secret|--org": "PROJECT_PAT\tUpdated 2026-04-01",
		},
	}
	deps := CheckDeps{
		RepoFullName: "eddie/repo", Owner: "eddie", Topology: "single",
		Run: r.fn(),
	}
	res := checkSecret(deps, "PROJECT_PAT")
	if res.Status != Fail {
		t.Fatalf("got %d (%s), want Fail", res.Status, res.Message)
	}
	wantHint := "gh secret set PROJECT_PAT --repo eddie/repo"
	if res.Remediation != wantHint {
		t.Errorf("remediation: got %q, want %q", res.Remediation, wantHint)
	}
}

func TestCheckVariable_NeverQueriesOrg(t *testing.T) {
	// Feature #824: org scope is never consulted for any variable.
	r := &ghRun{}
	for _, varName := range []string{"RUNNER_LABEL", "AGENT_PROVIDER", "AGENTIC_PROJECT_ID"} {
		r.Calls = nil
		deps := CheckDeps{
			RepoFullName: "acme/domain", Owner: "acme", Topology: "federation",
			Run: r.fn(),
		}
		_ = checkVariable(deps, varName)
		for _, c := range r.Calls {
			for _, a := range c {
				if a == "--org" {
					t.Errorf("%s: org scope must never be consulted (calls: %v)", varName, r.Calls)
				}
			}
		}
	}
}

// TestCheckVariable_MissingWithDefault_Warns covers the soft-default behaviour:
// AGENT_PROVIDER and AGENT_MODEL have safe defaults (workflows fall back to
// `claude-code`), so a missing value is reported as a Warning with the
// default surfaced — not a hard failure.
func TestCheckVariable_MissingWithDefault_Warns(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "single",
		Run: func(name string, args ...string) (string, error) {
			return "", nil // variable absent
		},
	}
	res := checkVariable(deps, "AGENT_PROVIDER")
	if res.Status != Warning {
		t.Fatalf("status: got %d (%s), want Warning", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "claude-code") {
		t.Errorf("message: got %q, want default 'claude-code' surfaced", res.Message)
	}
	if !strings.HasPrefix(res.Remediation, "gh variable set AGENT_PROVIDER") {
		t.Errorf("remediation should still offer set hint, got %q", res.Remediation)
	}
}

func TestCheckVariable_Missing_FailWithRepoRemediation(t *testing.T) {
	// Feature #824: remediation always points to --repo regardless of topology.
	r := &ghRun{}
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federation",
		Run: r.fn(),
	}
	res := checkVariable(deps, "RUNNER_LABEL")
	if res.Status != Fail {
		t.Fatalf("got %d (%s), want Fail", res.Status, res.Message)
	}
	wantHint := "gh variable set RUNNER_LABEL --repo acme/domain"
	if res.Remediation != wantHint {
		t.Errorf("remediation: got %q, want %q", res.Remediation, wantHint)
	}
}

// TestCheckVariable_PermissionError_Warns verifies that a 403 from gh CLI is
// reported as "unable to check" rather than "not configured", preventing
// false-positive failures when the token lacks Actions:Read (common with GitHub
// App installation tokens in CI runners).
func TestCheckVariable_PermissionError_Warns(t *testing.T) {
	for _, tc := range []struct {
		name   string
		output string
	}{
		{"HTTP 403", "HTTP 403: Resource not accessible by integration"},
		{"insufficient scopes", "error: insufficient scopes"},
		{"resource not accessible", "Resource not accessible"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := CheckDeps{
				RepoFullName: "acme/repo", Owner: "acme", Topology: "single",
				Run: func(name string, args ...string) (string, error) {
					return tc.output, fmt.Errorf("gh: exit status 1")
				},
			}
			res := checkVariable(deps, "RUNNER_LABEL")
			if res.Status != Warning {
				t.Fatalf("status: got %d (%s), want Warning", res.Status, res.Message)
			}
			if !strings.Contains(res.Message, "unable to check") {
				t.Errorf("message: got %q, want 'unable to check'", res.Message)
			}
			if res.Remediation != "" {
				t.Errorf("remediation: want empty for auth error, got %q", res.Remediation)
			}
		})
	}
}

