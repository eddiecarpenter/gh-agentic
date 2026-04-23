package mount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReusableWorkflowHasNoCallerRelativeActionPaths guards against a
// regression introduced in v2.2.2, where agentic-pipeline.yml
// referenced local composite actions via `./.github/actions/...`. When the
// reusable workflow is called from a domain repo, those paths resolve
// against the domain repo's workspace — which does not contain the
// actions — and every domain pipeline failed at the "Install tools" step.
//
// The fix is to populate `.agentic-tools/` with eddiecarpenter/gh-agentic
// and reference the actions via that path. Two mechanisms populate it:
//
//  1. Legacy (pre-#622): an explicit `actions/checkout` step with
//     `repository: eddiecarpenter/gh-agentic` + `path: .agentic-tools`.
//  2. Current (post-#622): `.agentic-tools/` is tracked as a git
//     submodule pointing at gh-agentic, and the primary checkout step
//     uses `submodules: recursive` to populate it.
//
// This test accepts either mechanism — it only fails if `./.agentic-tools/...`
// is referenced AND neither population mechanism is present.
//
// If this test fails, the reusable workflow either:
//   - re-introduced a `./.github/actions/...` path (must use
//     `./.agentic-tools/.github/actions/...` instead), or
//   - dropped both the nested checkout of eddiecarpenter/gh-agentic
//     and the `submodules: recursive` option on the primary checkout.
func TestReusableWorkflowHasNoCallerRelativeActionPaths(t *testing.T) {
	path := reusableWorkflowPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(data)

	// Any `uses: ./.github/actions/...` line is a bug — the reusable
	// workflow runs in the caller's workspace and those actions live in
	// eddiecarpenter/gh-agentic, not in the caller. Scan `uses:` lines
	// only so that comments and documentation strings can reference the
	// broken pattern without tripping the test.
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "uses:") && strings.Contains(trimmed, "./.github/actions/") {
			t.Errorf(
				"%s line %d references ./.github/actions/... in a uses: step — these paths resolve "+
					"in the caller's workspace and will fail in every domain repo. Use "+
					"./.agentic-tools/.github/actions/... and nested-checkout eddiecarpenter/gh-agentic "+
					"at the resolved framework version.\n  line: %s",
				filepath.Base(path), i+1, trimmed,
			)
		}
	}

	// `.agentic-tools/` must be populated by one of two mechanisms before
	// any `./.agentic-tools/...` consumer runs:
	//   1. Legacy: an explicit nested checkout step with
	//      `repository: eddiecarpenter/gh-agentic` + `path: .agentic-tools`.
	//   2. Current: a primary checkout with `submodules: recursive`, which
	//      populates the `.agentic-tools/` submodule (added in PR #620,
	//      consolidated by Feature #622).
	// Either is sufficient. Guard the consumer side so a future edit
	// cannot drop both mechanisms and leave a dangling relative path.
	hasNestedCheckout := strings.Contains(content, "repository: eddiecarpenter/gh-agentic") &&
		strings.Contains(content, "path: .agentic-tools")
	hasSubmoduleCheckout := strings.Contains(content, "submodules: recursive")
	hasNestedConsumer := strings.Contains(content, "./.agentic-tools/.github/actions/")

	if hasNestedConsumer && !(hasNestedCheckout || hasSubmoduleCheckout) {
		t.Errorf(
			"%s references ./.agentic-tools/... actions but neither (a) nested-checkouts "+
				"eddiecarpenter/gh-agentic into path: .agentic-tools nor (b) uses "+
				"submodules: recursive on the primary checkout. Add one mechanism "+
				"before the first consumer.",
			filepath.Base(path),
		)
	}

	if hasNestedCheckout && !hasNestedConsumer {
		t.Errorf(
			"%s nested-checkouts eddiecarpenter/gh-agentic into .agentic-tools/ but no step "+
				"consumes it. The checkout is dead weight — either wire up the actions via "+
				"./.agentic-tools/.github/actions/... or remove the checkout step.",
			filepath.Base(path),
		)
	}
}

// TestCompositeActionsHaveNoInterCompositeUses guards against a related
// class of bug. GitHub Actions resolves `uses: ./path` inside a composite
// action relative to the **caller's workspace root**, not the composite's
// own directory. When a composite references another composite by
// `./.github/actions/...` and is consumed via nested checkout from a
// different repo (the model in `agentic-pipeline.yml`), the path
// does not resolve. Inline shared logic instead of composing composites.
//
// The v2.2.3 release hit this: `install-ai-tools` referenced
// `./.github/actions/install-system-deps`, which then failed in every
// domain pipeline that consumed the framework via nested checkout. The
// fix inlined `install-system-deps` and removed the directory. This test
// freezes that policy so no one reintroduces the pattern.
func TestCompositeActionsHaveNoInterCompositeUses(t *testing.T) {
	actionsDir := compositeActionsDir(t)
	entries, err := os.ReadDir(actionsDir)
	if err != nil {
		t.Fatalf("read %s: %v", actionsDir, err)
	}
	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		actionYAML := filepath.Join(actionsDir, e.Name(), "action.yml")
		data, err := os.ReadFile(actionYAML)
		if err != nil {
			continue // not all directories are composite actions
		}
		found++
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "uses:") && strings.Contains(trimmed, "./.github/actions/") {
				t.Errorf(
					"%s line %d references another composite via ./.github/actions/... — "+
						"GitHub resolves that path against the caller's workspace, which breaks "+
						"when consumed via nested checkout. Inline the referenced composite's "+
						"steps into this one instead.\n  line: %s",
					filepath.Join(e.Name(), "action.yml"), i+1, trimmed,
				)
			}
		}
	}
	if found == 0 {
		t.Fatalf("no composite actions discovered under %s — test is a no-op", actionsDir)
	}
}

// reusableWorkflowPath returns the absolute path to the reusable pipeline
// workflow at the repo root. Tests in internal/mount/ run with CWD =
// internal/mount, so the walk is short.
func reusableWorkflowPath(t *testing.T) string {
	t.Helper()
	return repoRelativePath(t, ".github", "workflows", "agentic-pipeline.yml")
}

// compositeActionsDir returns the absolute path to .github/actions/ at
// the repo root.
func compositeActionsDir(t *testing.T) string {
	t.Helper()
	return repoRelativePath(t, ".github", "actions")
}

// repoRelativePath walks up from the test working directory looking for a
// path that exists relative to each ancestor, returning the absolute path
// on the first hit. Used to locate repo-root artefacts from inside a
// package test binary where Go sets CWD to the package directory.
func repoRelativePath(t *testing.T, segments ...string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	rel := filepath.Join(segments...)
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate %s from %s", rel, dir)
	return ""
}
