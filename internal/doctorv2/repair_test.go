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
