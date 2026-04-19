package doctorv2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
)

// fakeRunMissingVarsAndSecret simulates `gh` returning empty values for all
// variables and an empty secret list, so every variable/secret check fails.
func fakeRunMissingVarsAndSecret(name string, args ...string) (string, error) {
	if name == "gh" && len(args) > 1 && args[0] == "variable" && args[1] == "get" {
		return "", nil
	}
	if name == "gh" && len(args) > 1 && args[0] == "secret" && args[1] == "list" {
		return "", nil
	}
	return "", nil
}

// makeRepoNeedingPipelineRepairs builds a repo that:
//   - has the framework mounted at v2.0.0,
//   - has NO .gitignore entry for .ai/,
//   - has workflow files pinned to a stale @v1.0.0 tag,
//   - has no GH variables or secrets configured.
func makeRepoNeedingPipelineRepairs(t *testing.T) string {
	t.Helper()
	root := setupHealthyRepo(t)

	// Break .gitignore.
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("# nothing\n"), 0o644)

	// Break workflow tags (currently @v2.0.0 from setupHealthyRepo).
	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline-reusable.yml@v1.0.0"), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/release-reusable.yml@v1.0.0"), 0o644)

	return root
}

func TestRepairPipeline_FixesGitignoreAndWorkflowTags(t *testing.T) {
	root := makeRepoNeedingPipelineRepairs(t)

	deps := CheckDeps{
		Root:         root,
		RepoFullName: "owner/repo",
		Owner:        "owner",
		RepoName:     "repo",
		OwnerType:    "User",
		Topology:     "single",
		Run:          fakeRunMissingVarsAndSecret,
		ReadCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return []byte(`{"token":"abc"}`), nil
		},
	}

	result := RepairPipeline(deps, nil)

	// Verify .gitignore was repaired on disk.
	gi, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if !strings.Contains(string(gi), ".ai/") {
		t.Errorf(".gitignore does not contain .ai/ after repair:\n%s", string(gi))
	}

	// Verify workflow tags were rewritten to the mounted version (v2.0.0).
	for _, wf := range []string{"agentic-pipeline.yml", "release.yml"} {
		data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", wf))
		if err != nil {
			t.Fatalf("reading %s: %v", wf, err)
		}
		if strings.Contains(string(data), "@v1.0.0") {
			t.Errorf("%s still pinned to @v1.0.0:\n%s", wf, string(data))
		}
		if !strings.Contains(string(data), "@v2.0.0") {
			t.Errorf("%s missing expected @v2.0.0:\n%s", wf, string(data))
		}
	}

	// Two auto-repairs (gitignore + workflows-as-a-batch) expected.
	if result.Repaired != 2 {
		t.Errorf("expected 2 repairs, got %d (lines: %v)", result.Repaired, result.Lines)
	}

	// Variable + secret failures must surface as Unrepaired (not silently dropped).
	if result.Unrepaired == 0 {
		t.Errorf("expected unrepaired failures for missing vars/secrets, got 0")
	}

	// The single workflow fix should produce exactly one workflow line, not two.
	workflowLines := 0
	for _, l := range result.Lines {
		if strings.Contains(l, "Workflow version tags updated") {
			workflowLines++
		}
	}
	if workflowLines != 1 {
		t.Errorf("expected one workflow-tags line, got %d", workflowLines)
	}
}

// capturedGH holds every gh invocation a fake run function received, so
// routing tests can assert exactly which scope flag/target was used.
type capturedGH struct {
	called [][]string
}

func (c *capturedGH) run(name string, args ...string) (string, error) {
	if name == "gh" {
		c.called = append(c.called, append([]string{}, args...))
	}
	return "", nil
}

// assertGHScope asserts that the last captured gh invocation used the
// expected scope flag and target. The test relies on the invocation being
// the write (`gh variable set ...` / `gh secret set ...`), so make sure
// nothing else ran afterwards.
func assertGHScope(t *testing.T, c *capturedGH, wantFlag, wantTarget string) {
	t.Helper()
	if len(c.called) == 0 {
		t.Fatalf("no gh invocations captured")
	}
	last := c.called[len(c.called)-1]
	for i := 0; i < len(last)-1; i++ {
		if last[i] == wantFlag {
			if last[i+1] != wantTarget {
				t.Fatalf("scope target mismatch: got %q, want %q (full cmd: %v)", last[i+1], wantTarget, last)
			}
			return
		}
	}
	t.Fatalf("scope flag %q not found in cmd: %v", wantFlag, last)
}

func TestApplyPendingPrompt_Federated_SharedName_RoutesToOrg(t *testing.T) {
	shared := []string{"AGENT_USER", "RUNNER_LABEL", "GOOSE_PROVIDER", "GOOSE_MODEL", "GOOSE_AGENT_PAT", "CLAUDE_CREDENTIALS_JSON"}
	for _, name := range shared {
		t.Run(name, func(t *testing.T) {
			c := &capturedGH{}
			kind := "variable"
			if name == "GOOSE_AGENT_PAT" || name == "CLAUDE_CREDENTIALS_JSON" {
				kind = "secret"
			}
			p := PendingPrompt{
				Name:     name,
				Kind:     kind,
				Topology: "federated-cp",
				Owner:    "acme",
			}
			if err := ApplyPendingPrompt(c.run, "acme/repo", p, "v"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertGHScope(t, c, "--org", "acme")
		})
	}
}

func TestApplyPendingPrompt_Federated_IdentityName_StaysAtRepo(t *testing.T) {
	for _, name := range []string{"AGENTIC_PROJECT_ID", "AGENTIC_TOPOLOGY", "AGENTIC_FRAMEWORK_VERSION"} {
		t.Run(name, func(t *testing.T) {
			c := &capturedGH{}
			p := PendingPrompt{
				Name:     name,
				Kind:     "variable",
				Topology: "federated-cp",
				Owner:    "acme",
			}
			if err := ApplyPendingPrompt(c.run, "acme/repo", p, "v"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertGHScope(t, c, "--repo", "acme/repo")
		})
	}
}

func TestApplyPendingPrompt_Single_AllNames_StayAtRepo(t *testing.T) {
	all := []string{
		"AGENT_USER", "RUNNER_LABEL", "GOOSE_PROVIDER", "GOOSE_MODEL",
		"GOOSE_AGENT_PAT", "CLAUDE_CREDENTIALS_JSON",
		"AGENTIC_PROJECT_ID", "AGENTIC_TOPOLOGY", "AGENTIC_FRAMEWORK_VERSION",
	}
	for _, name := range all {
		t.Run(name, func(t *testing.T) {
			c := &capturedGH{}
			kind := "variable"
			if name == "GOOSE_AGENT_PAT" || name == "CLAUDE_CREDENTIALS_JSON" {
				kind = "secret"
			}
			p := PendingPrompt{
				Name:     name,
				Kind:     kind,
				Topology: "single",
				Owner:    "eddie",
			}
			if err := ApplyPendingPrompt(c.run, "eddie/repo", p, "v"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertGHScope(t, c, "--repo", "eddie/repo")
		})
	}
}

func TestRepairPipeline_PopulatesTopologyAndOwnerOnPendingPrompts(t *testing.T) {
	// RepairPipeline must carry topology/owner onto PendingPrompt so the
	// later ApplyPendingPrompt call routes correctly.
	root := setupHealthyRepo(t)

	deps := CheckDeps{
		Root:         root,
		RepoFullName: "acme/cp",
		Owner:        "acme",
		RepoName:     "cp",
		OwnerType:    "Organization",
		Topology:     "federated-cp",
		Run:          fakeRunMissingVarsAndSecret,
		ReadCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return []byte(`{"token":"abc"}`), nil
		},
	}

	result := RepairPipeline(deps, nil)
	if len(result.PendingPrompts) == 0 {
		t.Fatal("expected pending prompts for missing shared values under federated-cp")
	}
	for _, p := range result.PendingPrompts {
		if p.Topology != "federated-cp" {
			t.Errorf("prompt %q topology: got %q, want %q", p.Name, p.Topology, "federated-cp")
		}
		if p.Owner != "acme" {
			t.Errorf("prompt %q owner: got %q, want %q", p.Name, p.Owner, "acme")
		}
	}
}

func TestRepairPipeline_NoFailures(t *testing.T) {
	root := setupHealthyRepo(t)

	deps := CheckDeps{
		Root:         root,
		RepoFullName: "owner/repo",
		Owner:        "owner",
		RepoName:     "repo",
		OwnerType:    "User",
		Topology:     "single",
		Run: func(name string, args ...string) (string, error) {
			if name == "gh" && len(args) > 1 && args[0] == "variable" && args[1] == "get" {
				return "configured", nil
			}
			if name == "gh" && len(args) > 1 && args[0] == "secret" && args[1] == "list" {
				return "GOOSE_AGENT_PAT\tUpdated 2026-04-01", nil
			}
			return "", nil
		},
		ReadCreds: func(run auth.RunCommandFunc) ([]byte, error) {
			return []byte(`{"token":"abc"}`), nil
		},
	}

	result := RepairPipeline(deps, nil)

	if result.Repaired != 0 || result.Unrepaired != 0 {
		t.Errorf("expected zero repairs and zero failures on healthy repo, got %+v", result)
	}
	if len(result.Lines) != 0 {
		t.Errorf("expected no output lines on healthy repo, got %v", result.Lines)
	}
}
