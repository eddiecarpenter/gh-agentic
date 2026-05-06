package doctor

import (
	"fmt"
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
//   - has a legacy `.agents/` line in .gitignore (must be stripped),
//   - has workflow files pinned to a stale @v1.0.0 tag,
//   - has no GH variables or secrets configured.
func makeRepoNeedingPipelineRepairs(t *testing.T) string {
	t.Helper()
	root := setupHealthyRepo(t)

	// Re-introduce the legacy `.agents/` gitignore entry so the repair has
	// something to fix; setupHealthyRepo no longer produces this line by
	// default in submodule mode.
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("# nothing\n.agents/\n"), 0o644)

	// Break workflow tags (currently @v2.0.0 from setupHealthyRepo).
	workflowsDir := filepath.Join(root, ".github", "workflows")
	_ = os.WriteFile(filepath.Join(workflowsDir, "agentic-pipeline.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/agentic-pipeline.yml@v1.0.0"), 0o644)
	_ = os.WriteFile(filepath.Join(workflowsDir, "release.yml"),
		[]byte("uses: eddiecarpenter/gh-agentic/.github/workflows/release.yml@v1.0.0"), 0o644)

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

	// Verify .gitignore was repaired on disk — the legacy `.agents/` line
	// is now stripped (submodule mount, not gitignored shallow clone).
	gi, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if strings.Contains(string(gi), ".agents/") {
		t.Errorf(".gitignore still contains .agents/ after repair:\n%s", string(gi))
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
	shared := []string{"AGENT_USER", "RUNNER_LABEL", "AGENT_PROVIDER", "AGENT_MODEL", "PROJECT_PAT", "CLAUDE_CREDENTIALS_JSON"}
	for _, name := range shared {
		t.Run(name, func(t *testing.T) {
			c := &capturedGH{}
			kind := "variable"
			if name == "PROJECT_PAT" || name == "CLAUDE_CREDENTIALS_JSON" {
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
		"AGENT_USER", "RUNNER_LABEL", "AGENT_PROVIDER", "AGENT_MODEL",
		"PROJECT_PAT", "CLAUDE_CREDENTIALS_JSON",
		"AGENTIC_PROJECT_ID", "AGENTIC_TOPOLOGY", "AGENTIC_FRAMEWORK_VERSION",
	}
	for _, name := range all {
		t.Run(name, func(t *testing.T) {
			c := &capturedGH{}
			kind := "variable"
			if name == "PROJECT_PAT" || name == "CLAUDE_CREDENTIALS_JSON" {
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

// --- RepairShadowValues tests (task #533) ---

// shadowRunRecorder captures every gh invocation so tests can assert the
// exact commands and order. Errors can be injected per key so a single
// delete failure in the batch can be simulated.
type shadowRunRecorder struct {
	calls [][]string
	errs  map[string]error // key: "kind|name"
}

func (r *shadowRunRecorder) run(name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{}, args...))
	if len(args) >= 3 && args[1] == "delete" {
		key := args[0] + "|" + args[2]
		if err, ok := r.errs[key]; ok {
			return "", err
		}
	}
	return "", nil
}

// recordedConfirm captures a single confirmation invocation and returns a
// pre-arranged yes/no answer.
type recordedConfirm struct {
	called int
	title  string
	body   string
	yes    bool
	err    error
}

func (c *recordedConfirm) run(title, body string) (bool, error) {
	c.called++
	c.title = title
	c.body = body
	return c.yes, c.err
}

func makeShadowItems() []ShadowValue {
	return []ShadowValue{
		{Name: "AGENT_USER", Kind: "variable", DeleteCommand: "gh variable delete --repo acme/cp AGENT_USER"},
		{Name: "PROJECT_PAT", Kind: "secret", DeleteCommand: "gh secret delete --repo acme/cp PROJECT_PAT"},
	}
}

func TestRepairShadowValues_ConfirmYes_AllDeletesSucceed(t *testing.T) {
	items := makeShadowItems()
	run := &shadowRunRecorder{}
	confirm := &recordedConfirm{yes: true}

	res := RepairShadowValues(items, run.run, confirm.run)

	if confirm.called != 1 {
		t.Errorf("confirm called %d times, want exactly 1", confirm.called)
	}
	if res.Repaired != 2 {
		t.Errorf("Repaired = %d, want 2", res.Repaired)
	}
	if res.Unrepaired != 0 {
		t.Errorf("Unrepaired = %d, want 0", res.Unrepaired)
	}
	if len(run.calls) != 2 {
		t.Fatalf("gh called %d times, want 2: %v", len(run.calls), run.calls)
	}
	// Delete commands use the injected runner with the right scope and name.
	wantFirst := []string{"variable", "delete", "AGENT_USER", "--repo", "acme/cp"}
	wantSecond := []string{"secret", "delete", "PROJECT_PAT", "--repo", "acme/cp"}
	if !reflectDeepEqualStrings(run.calls[0], wantFirst) {
		t.Errorf("call 0: got %v, want %v", run.calls[0], wantFirst)
	}
	if !reflectDeepEqualStrings(run.calls[1], wantSecond) {
		t.Errorf("call 1: got %v, want %v", run.calls[1], wantSecond)
	}
}

func TestRepairShadowValues_ConfirmYes_OneFailureDoesNotAbortBatch(t *testing.T) {
	items := makeShadowItems()
	run := &shadowRunRecorder{
		errs: map[string]error{
			"variable|AGENT_USER": fmt.Errorf("boom"),
		},
	}
	confirm := &recordedConfirm{yes: true}

	res := RepairShadowValues(items, run.run, confirm.run)

	// Both items attempted despite the first failing.
	if len(run.calls) != 2 {
		t.Fatalf("want 2 gh invocations, got %d: %v", len(run.calls), run.calls)
	}
	if res.Repaired != 1 {
		t.Errorf("Repaired = %d, want 1 (secret succeeded)", res.Repaired)
	}
	if res.Unrepaired != 1 {
		t.Errorf("Unrepaired = %d, want 1 (variable failed)", res.Unrepaired)
	}
	// The error message must appear in the output.
	var sawError bool
	for _, l := range res.Lines {
		if strings.Contains(l, "boom") {
			sawError = true
		}
	}
	if !sawError {
		t.Errorf("expected 'boom' in output, got:\n%v", res.Lines)
	}
}

func TestRepairShadowValues_ConfirmNo_NoDeletesIssued(t *testing.T) {
	items := makeShadowItems()
	run := &shadowRunRecorder{}
	confirm := &recordedConfirm{yes: false}

	res := RepairShadowValues(items, run.run, confirm.run)

	if len(run.calls) != 0 {
		t.Errorf("expected zero gh invocations on No, got %v", run.calls)
	}
	if res.Repaired != 0 {
		t.Errorf("Repaired = %d, want 0", res.Repaired)
	}
	if res.Unrepaired != 2 {
		t.Errorf("Unrepaired = %d, want 2", res.Unrepaired)
	}
	// Each item surfaces with its original delete command (manual remedy).
	for _, it := range items {
		found := false
		for _, l := range res.Lines {
			if strings.Contains(l, it.DeleteCommand) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("manual command for %q missing from output: %v", it.Name, res.Lines)
		}
	}
}

func TestRepairShadowValues_EmptyItems_NoWork_NoPrompt(t *testing.T) {
	run := &shadowRunRecorder{}
	confirm := &recordedConfirm{yes: true}

	res := RepairShadowValues(nil, run.run, confirm.run)

	if confirm.called != 0 {
		t.Errorf("expected confirm not called on empty items, was called %d", confirm.called)
	}
	if len(run.calls) != 0 {
		t.Errorf("expected zero gh invocations, got %v", run.calls)
	}
	if res.Repaired != 0 || res.Unrepaired != 0 {
		t.Errorf("expected zero counts, got Repaired=%d Unrepaired=%d", res.Repaired, res.Unrepaired)
	}
}

func TestRepairShadowValues_ConfirmTitleInterpolatesCount(t *testing.T) {
	items := makeShadowItems()
	run := &shadowRunRecorder{}
	confirm := &recordedConfirm{yes: false}

	_ = RepairShadowValues(items, run.run, confirm.run)

	// Sentence placeholder "N" is replaced with the item count.
	if !strings.Contains(confirm.title, "Remove these 2 shadow values") {
		t.Errorf("title: got %q, want count 2 interpolated", confirm.title)
	}
	// Body lists each name with its kind.
	for _, it := range items {
		needle := it.Name + " (" + it.Kind + ")"
		if !strings.Contains(confirm.body, needle) {
			t.Errorf("body missing %q; got:\n%s", needle, confirm.body)
		}
	}
}

// reflectDeepEqualStrings is a tiny helper so the repair tests avoid
// pulling reflect into the package namespace.
func reflectDeepEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
		// ProjectID is supplied by the resolver in production; the
		// doctor trusts it directly (task #583 removed the gh-CLI
		// fallback read in checkProjectReachability).
		ProjectID:         "PVT_configured",
		FetchProjectTitle: func(id string) (string, error) { return "Healthy", nil },
		Run: func(name string, args ...string) (string, error) {
			if name == "gh" && len(args) > 1 && args[0] == "variable" && args[1] == "get" {
				return "configured", nil
			}
			if name == "gh" && len(args) > 1 && args[0] == "secret" && args[1] == "list" {
				return "PROJECT_PAT\tUpdated 2026-04-01", nil
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
