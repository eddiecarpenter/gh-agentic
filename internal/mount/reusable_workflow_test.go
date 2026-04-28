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
// The fix is to reference the actions via `./.ai/.github/actions/...`. The
// `.ai/` mount is the single bridge to the framework: it is a git submodule
// pointing at eddiecarpenter/gh-agentic in domain repos, and a `.ai -> .`
// symlink in gh-agentic itself. `submodules: recursive` on the primary
// checkout step populates it.
//
// If this test fails, the reusable workflow either:
//   - re-introduced a `./.github/actions/...` path (must use
//     `./.ai/.github/actions/...` instead),
//   - re-introduced a reference to the legacy `.agentic-tools/` bridge, or
//   - dropped `submodules: recursive` on the primary checkout, leaving
//     `.ai/` unpopulated when consumed from a domain repo.
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
					"./.ai/.github/actions/... instead — the .ai/ submodule (or symlink in "+
					"gh-agentic itself) is the bridge to the framework's composite actions.\n  line: %s",
				filepath.Base(path), i+1, trimmed,
			)
		}
	}

	// The legacy `.agentic-tools/` bridge has been removed in favour of
	// `.ai/`. Any reference to it — in `uses:` paths, comments, or
	// elsewhere — is a regression.
	if strings.Contains(content, ".agentic-tools") {
		t.Errorf(
			"%s references the legacy .agentic-tools/ bridge — this was replaced by .ai/. "+
				"Use ./.ai/.github/actions/... for composite actions.",
			filepath.Base(path),
		)
	}

	// `.ai/` must be populated by `submodules: recursive` on the primary
	// checkout before any `./.ai/.github/actions/...` consumer runs.
	hasSubmoduleCheckout := strings.Contains(content, "submodules: recursive")
	hasNestedConsumer := strings.Contains(content, "./.ai/.github/actions/")

	if hasNestedConsumer && !hasSubmoduleCheckout {
		t.Errorf(
			"%s references ./.ai/.github/actions/... actions but the primary checkout does not "+
				"use `submodules: recursive`. Without it, the .ai/ submodule is empty in domain "+
				"repos and every consuming step fails at startup.",
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
