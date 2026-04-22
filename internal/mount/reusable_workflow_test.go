package mount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReusableWorkflowHasNoCallerRelativeActionPaths guards against a
// regression introduced in v2.2.2, where agentic-pipeline-reusable.yml
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

// reusableWorkflowPath walks up from the test's working directory to the
// repository root and returns the absolute path to
// .github/workflows/agentic-pipeline-reusable.yml. Tests in
// internal/mount/ run with CWD = internal/mount, so the walk is short.
func reusableWorkflowPath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, ".github", "workflows", "agentic-pipeline-reusable.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate .github/workflows/agentic-pipeline-reusable.yml from %s", dir)
	return ""
}
