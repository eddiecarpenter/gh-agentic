package doctor

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
)

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
	aiDir := filepath.Join(root, ".ai")
	_ = os.MkdirAll(filepath.Join(aiDir, "skills"), 0o755)
	_ = os.MkdirAll(filepath.Join(aiDir, "standards"), 0o755)
	_ = os.WriteFile(filepath.Join(aiDir, "RULEBOOK.md"), []byte("# Rules"), 0o644)
	setupAIGitRepo(t, aiDir, "v2.0.0")

	// The .ai-version flat file is gone (#585); .ai/.git metadata is the
	// only local version source, installed by setupAIGitRepo above.

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
	setupAIGitRepo(t, filepath.Join(root, ".ai"), "v2.0.0")
	// Version is stored in .ai/.git (installed by setupAIGitRepo); no
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
	setupAIGitRepo(t, filepath.Join(root, ".ai"), "v2.0.0")
	// Version is stored in .ai/.git (installed by setupAIGitRepo); no
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
	setupAIGitRepo(t, filepath.Join(root, ".ai"), "v2.1.0")
	// Version stored in .ai/.git; no flat file (#585).

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
// wired into the topology list for single, federated-cp, and federated-domain.
func TestCheckProjectReachability_RunsForAllTopologies(t *testing.T) {
	topologies := []string{"single", "federated-cp", "federated-domain"}
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

func TestCheckSecret_Federated_SecretAtOrgOnly_Pass(t *testing.T) {
	r := &ghRun{
		Outputs: map[string]string{
			"secret|--org": "GOOSE_AGENT_PAT\tUpdated 2026-04-01",
			// repo listing returns nothing
		},
	}
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federated-domain",
		Run: r.fn(),
	}
	res := checkSecret(deps, "GOOSE_AGENT_PAT")
	if res.Status != Pass {
		t.Fatalf("got %d (%s), want Pass — calls: %v", res.Status, res.Message, r.Calls)
	}
}

func TestCheckSecret_Federated_SecretAtRepoOnly_Pass(t *testing.T) {
	// Pass-and-overlap is fine here; shadow is a separate check (#532).
	r := &ghRun{
		Outputs: map[string]string{
			"secret|--repo": "GOOSE_AGENT_PAT\tUpdated 2026-04-01",
			// org listing returns nothing
		},
	}
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federated-domain",
		Run: r.fn(),
	}
	res := checkSecret(deps, "GOOSE_AGENT_PAT")
	if res.Status != Pass {
		t.Fatalf("got %d (%s), want Pass", res.Status, res.Message)
	}
}

func TestCheckSecret_Federated_SecretAtNeither_FailWithOrgRemediation(t *testing.T) {
	r := &ghRun{} // no outputs → both scopes return empty
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federated-domain",
		Run: r.fn(),
	}
	res := checkSecret(deps, "GOOSE_AGENT_PAT")
	if res.Status != Fail {
		t.Fatalf("got %d (%s), want Fail", res.Status, res.Message)
	}
	wantHint := "gh secret set GOOSE_AGENT_PAT --org acme"
	if res.Remediation != wantHint {
		t.Errorf("remediation: got %q, want %q", res.Remediation, wantHint)
	}
	// Must have consulted both scopes.
	var sawRepo, sawOrg bool
	for _, c := range r.Calls {
		for _, a := range c {
			if a == "--repo" {
				sawRepo = true
			}
			if a == "--org" {
				sawOrg = true
			}
		}
	}
	if !sawRepo || !sawOrg {
		t.Errorf("expected both --repo and --org consulted; got --repo=%v --org=%v (calls: %v)", sawRepo, sawOrg, r.Calls)
	}
}

func TestCheckSecret_Single_SecretAtRepo_Pass(t *testing.T) {
	r := &ghRun{
		Outputs: map[string]string{
			"secret|--repo": "GOOSE_AGENT_PAT\tUpdated 2026-04-01",
		},
	}
	deps := CheckDeps{
		RepoFullName: "eddie/repo", Owner: "eddie", Topology: "single",
		Run: r.fn(),
	}
	res := checkSecret(deps, "GOOSE_AGENT_PAT")
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
			"secret|--org": "GOOSE_AGENT_PAT\tUpdated 2026-04-01",
		},
	}
	deps := CheckDeps{
		RepoFullName: "eddie/repo", Owner: "eddie", Topology: "single",
		Run: r.fn(),
	}
	res := checkSecret(deps, "GOOSE_AGENT_PAT")
	if res.Status != Fail {
		t.Fatalf("got %d (%s), want Fail", res.Status, res.Message)
	}
	wantHint := "gh secret set GOOSE_AGENT_PAT --repo eddie/repo"
	if res.Remediation != wantHint {
		t.Errorf("remediation: got %q, want %q", res.Remediation, wantHint)
	}
}

func TestCheckVariable_Federated_SharedName_QueriesOrg(t *testing.T) {
	r := &ghRun{
		Outputs: map[string]string{
			"variable|--org": "AGENT_USER\tsome-value",
			// repo variable get returns empty (not set)
		},
	}
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federated-domain",
		Run: r.fn(),
	}
	res := checkVariable(deps, "AGENT_USER")
	if res.Status != Pass {
		t.Fatalf("got %d (%s), want Pass", res.Status, res.Message)
	}
}

func TestCheckVariable_Federated_IdentityName_NeverQueriesOrg(t *testing.T) {
	// AGENTIC_PROJECT_ID is an identity name — even under federated it
	// belongs at the repo only. The org list must never be consulted.
	r := &ghRun{}
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federated-domain",
		Run: r.fn(),
	}
	_ = checkVariable(deps, "AGENTIC_PROJECT_ID")
	for _, c := range r.Calls {
		for _, a := range c {
			if a == "--org" {
				t.Errorf("identity name must not trigger --org lookup (calls: %v)", r.Calls)
			}
		}
	}
}

// --- checkShadowVars tests (task #532) ---

// shadowScenario configures a ghRun with specific list outputs for each
// of the four list queries the shadow-vars check issues.
type shadowScenario struct {
	VarRepo string
	VarOrg  string
	SecRepo string
	SecOrg  string
}

func (s shadowScenario) ghRun() *ghRun {
	return &ghRun{
		Outputs: map[string]string{
			"variable|--repo": s.VarRepo,
			"variable|--org":  s.VarOrg,
			"secret|--repo":   s.SecRepo,
			"secret|--org":    s.SecOrg,
		},
	}
}

func TestCheckShadowVars_Federated_NoShadows_Pass(t *testing.T) {
	r := shadowScenario{
		// Shared name present only at org — correct federated placement.
		VarOrg: "AGENT_USER\tvalue",
		SecOrg: "GOOSE_AGENT_PAT\tupdated",
	}.ghRun()
	deps := CheckDeps{
		RepoFullName: "acme/cp", Owner: "acme", Topology: "federated-cp",
		Run: r.fn(),
	}
	g := checkShadowVars(deps)
	if len(g.Results) != 1 || g.Results[0].Status != Pass {
		t.Fatalf("expected single Pass result, got %+v", g.Results)
	}
	if g.Results[0].Data != nil {
		t.Errorf("expected nil Data on pass result, got %v", g.Results[0].Data)
	}
}

func TestCheckShadowVars_Federated_VariableShadow_FailWithDeleteCommand(t *testing.T) {
	r := shadowScenario{
		VarRepo: "AGENT_USER\tshadow",
		VarOrg:  "AGENT_USER\ttrue-value",
	}.ghRun()
	deps := CheckDeps{
		RepoFullName: "acme/cp", Owner: "acme", Topology: "federated-cp",
		Run: r.fn(),
	}
	g := checkShadowVars(deps)
	// Expect: 1 summary Fail + 1 per-item Fail for AGENT_USER variable.
	if len(g.Results) != 2 {
		t.Fatalf("expected 2 results (summary + item), got %d: %+v", len(g.Results), g.Results)
	}
	if g.Results[0].Status != Fail {
		t.Errorf("summary: got %d, want Fail", g.Results[0].Status)
	}
	data, ok := g.Results[0].Data.([]ShadowValue)
	if !ok || len(data) != 1 {
		t.Fatalf("expected []ShadowValue of length 1 on summary, got %v", g.Results[0].Data)
	}
	if data[0].Kind != "variable" || data[0].Name != "AGENT_USER" {
		t.Errorf("structured data: got %+v, want variable/AGENT_USER", data[0])
	}
	wantCmd := "gh variable delete --repo acme/cp AGENT_USER"
	if data[0].DeleteCommand != wantCmd {
		t.Errorf("structured delete command: got %q, want %q", data[0].DeleteCommand, wantCmd)
	}
	if g.Results[1].Remediation != wantCmd {
		t.Errorf("per-item remediation: got %q, want %q", g.Results[1].Remediation, wantCmd)
	}
}

func TestCheckShadowVars_Federated_SecretShadow_FailWithDeleteCommand(t *testing.T) {
	r := shadowScenario{
		SecRepo: "GOOSE_AGENT_PAT\tshadow",
		SecOrg:  "GOOSE_AGENT_PAT\ttrue-value",
	}.ghRun()
	deps := CheckDeps{
		RepoFullName: "acme/cp", Owner: "acme", Topology: "federated-cp",
		Run: r.fn(),
	}
	g := checkShadowVars(deps)
	data, ok := g.Results[0].Data.([]ShadowValue)
	if !ok || len(data) != 1 {
		t.Fatalf("expected 1 shadow, got %v", g.Results[0].Data)
	}
	if data[0].Kind != "secret" || data[0].Name != "GOOSE_AGENT_PAT" {
		t.Errorf("got %+v, want secret/GOOSE_AGENT_PAT", data[0])
	}
	wantCmd := "gh secret delete --repo acme/cp GOOSE_AGENT_PAT"
	if data[0].DeleteCommand != wantCmd {
		t.Errorf("delete command: got %q, want %q", data[0].DeleteCommand, wantCmd)
	}
}

func TestCheckShadowVars_Federated_MixedShadows_AllListed(t *testing.T) {
	r := shadowScenario{
		VarRepo: "AGENT_USER\tx\nRUNNER_LABEL\ty",
		VarOrg:  "AGENT_USER\tx\nRUNNER_LABEL\ty",
		SecRepo: "GOOSE_AGENT_PAT\tz\nCLAUDE_CREDENTIALS_JSON\tw",
		SecOrg:  "GOOSE_AGENT_PAT\tz\nCLAUDE_CREDENTIALS_JSON\tw",
	}.ghRun()
	deps := CheckDeps{
		RepoFullName: "acme/cp", Owner: "acme", Topology: "federated-cp",
		Run: r.fn(),
	}
	g := checkShadowVars(deps)
	data, ok := g.Results[0].Data.([]ShadowValue)
	if !ok {
		t.Fatalf("expected []ShadowValue, got %T", g.Results[0].Data)
	}
	if len(data) != 4 {
		t.Errorf("expected 4 shadows, got %d: %+v", len(data), data)
	}
	// Count variables vs secrets.
	varCount, secCount := 0, 0
	for _, s := range data {
		switch s.Kind {
		case "variable":
			varCount++
		case "secret":
			secCount++
		}
	}
	if varCount != 2 || secCount != 2 {
		t.Errorf("want 2 variable + 2 secret shadows, got var=%d sec=%d", varCount, secCount)
	}
}

func TestCheckShadowVars_Federated_IssuesFourListQueries(t *testing.T) {
	r := shadowScenario{}.ghRun()
	deps := CheckDeps{
		RepoFullName: "acme/cp", Owner: "acme", Topology: "federated-cp",
		Run: r.fn(),
	}
	_ = checkShadowVars(deps)
	// Expect 4 list calls: var --repo, var --org, sec --repo, sec --org.
	want := map[string]int{
		"variable|--repo": 1,
		"variable|--org":  1,
		"secret|--repo":   1,
		"secret|--org":    1,
	}
	got := map[string]int{}
	for _, c := range r.Calls {
		if len(c) < 2 || c[1] != "list" {
			continue
		}
		scopeFlag := ""
		for _, a := range c {
			if a == "--repo" || a == "--org" {
				scopeFlag = a
				break
			}
		}
		got[c[0]+"|"+scopeFlag]++
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("list query %q: got %d, want %d (all calls: %v)", k, got[k], v, r.Calls)
		}
	}
}

func TestCheckShadowVars_Single_NotApplicable_NoOrgQueries(t *testing.T) {
	r := &ghRun{}
	deps := CheckDeps{
		RepoFullName: "eddie/repo", Owner: "eddie", Topology: "single",
		Run: r.fn(),
	}
	g := checkShadowVars(deps)
	if len(g.Results) != 1 || g.Results[0].Status != Pass {
		t.Fatalf("expected single Pass not-applicable result, got %+v", g.Results)
	}
	if !strings.Contains(g.Results[0].Message, "not applicable") {
		t.Errorf("expected 'not applicable' in message, got %q", g.Results[0].Message)
	}
	// Must not consult any scope under single.
	if len(r.Calls) != 0 {
		t.Errorf("single topology must not issue any gh list queries; got %v", r.Calls)
	}
}

func TestFindShadowValues_DeterministicOrder(t *testing.T) {
	// Same name appears as both a variable and a secret shadow — the slice
	// should emit the variable entry before the secret entry (declaration
	// order in the canonical shared list).
	r := shadowScenario{
		VarRepo: "AGENT_USER\tv",
		VarOrg:  "AGENT_USER\tv",
		SecRepo: "AGENT_USER\ts", // Same name, different kind.
		SecOrg:  "AGENT_USER\ts",
	}.ghRun()
	deps := CheckDeps{
		RepoFullName: "acme/cp", Owner: "acme", Topology: "federated-cp",
		Run: r.fn(),
	}
	shadows := FindShadowValues(deps)
	if len(shadows) != 2 {
		t.Fatalf("expected 2 shadows, got %d: %+v", len(shadows), shadows)
	}
	if shadows[0].Kind != "variable" || shadows[1].Kind != "secret" {
		t.Errorf("expected variable before secret, got %+v", shadows)
	}
}

func TestCheckVariable_Federated_SharedAtNeither_FailWithOrgRemediation(t *testing.T) {
	r := &ghRun{}
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federated-domain",
		Run: r.fn(),
	}
	res := checkVariable(deps, "AGENT_USER")
	if res.Status != Fail {
		t.Fatalf("got %d (%s), want Fail", res.Status, res.Message)
	}
	wantHint := "gh variable set AGENT_USER --org acme"
	if res.Remediation != wantHint {
		t.Errorf("remediation: got %q, want %q", res.Remediation, wantHint)
	}
}

// TestCheckTopologyStopgap_NotSet covers the post-#585 norm: the variable
// is absent and the resolver infers topology. The check reports Pass.
func TestCheckTopologyStopgap_NotSet(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federated-domain",
		Run: func(name string, args ...string) (string, error) {
			return "", nil // variable absent
		},
	}
	res := checkTopologyStopgap(deps)
	if res.Status != Pass {
		t.Fatalf("status: got %d (%s), want Pass", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "not set") {
		t.Errorf("message: got %q, want 'not set' mention", res.Message)
	}
}

// TestCheckTopologyStopgap_RedundantFederated covers the stopgap cleanup
// path: AGENTIC_TOPOLOGY=federated agrees with the resolver's federated-*
// inference, so the variable is redundant and can be deleted.
func TestCheckTopologyStopgap_RedundantFederated(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "acme/domain", Owner: "acme", Topology: "federated-domain",
		Run: func(name string, args ...string) (string, error) {
			return "federated\n", nil
		},
	}
	res := checkTopologyStopgap(deps)
	if res.Status != Warning {
		t.Fatalf("status: got %d (%s), want Warning", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "redundant") {
		t.Errorf("message: got %q, want 'redundant' mention", res.Message)
	}
	if !strings.Contains(res.Remediation, "gh variable delete AGENTIC_TOPOLOGY") {
		t.Errorf("remediation: got %q, want delete command", res.Remediation)
	}
}

// TestCheckTopologyStopgap_RedundantSingle covers the same path for single
// topology.
func TestCheckTopologyStopgap_RedundantSingle(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "user/solo", Owner: "user", Topology: "single",
		Run: func(name string, args ...string) (string, error) {
			return "single\n", nil
		},
	}
	res := checkTopologyStopgap(deps)
	if res.Status != Warning {
		t.Fatalf("status: got %d (%s), want Warning", res.Status, res.Message)
	}
}

// TestCheckTopologyStopgap_ExplicitOverride covers the case where the
// variable disagrees with the resolver — treat as an intentional override.
func TestCheckTopologyStopgap_ExplicitOverride(t *testing.T) {
	deps := CheckDeps{
		RepoFullName: "acme/cp", Owner: "acme", Topology: "federated-cp",
		Run: func(name string, args ...string) (string, error) {
			return "single\n", nil // disagrees with resolver
		},
	}
	res := checkTopologyStopgap(deps)
	if res.Status != Pass {
		t.Fatalf("status: got %d (%s), want Pass", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "explicit override") {
		t.Errorf("message: got %q, want 'explicit override' mention", res.Message)
	}
}
