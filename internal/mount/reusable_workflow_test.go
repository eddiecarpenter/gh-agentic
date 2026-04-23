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
// The fix is to nested-checkout eddiecarpenter/gh-agentic into
// `.agentic-tools/` and reference the actions via that path. This test
// enforces the rule statically so the regression cannot recur silently.
//
// If this test fails, the reusable workflow either:
//   - re-introduced a `./.github/actions/...` path (must use
//     `./.agentic-tools/.github/actions/...` instead), or
//   - dropped the nested checkout of eddiecarpenter/gh-agentic that
//     populates `.agentic-tools/` before the first action `uses`.
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

	// The fix requires a nested checkout of gh-agentic at `.agentic-tools/`.
	// If present, the file must show both the nested checkout step and at
	// least one `uses: ./.agentic-tools/...` consumer. Guard both halves so
	// that a future edit cannot silently drop the checkout and leave a
	// dangling relative path.
	hasNestedCheckout := strings.Contains(content, "repository: eddiecarpenter/gh-agentic") &&
		strings.Contains(content, "path: .agentic-tools")
	hasNestedConsumer := strings.Contains(content, "./.agentic-tools/.github/actions/")

	if hasNestedConsumer && !hasNestedCheckout {
		t.Errorf(
			"%s references ./.agentic-tools/... actions but does not nested-checkout "+
				"eddiecarpenter/gh-agentic into path: .agentic-tools. Add the checkout step "+
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
