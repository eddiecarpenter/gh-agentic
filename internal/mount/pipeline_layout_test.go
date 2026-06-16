package mount

import (
	"os/exec"
	"testing"
)

// TestCPLayoutStructure runs the Layer 2 structure check
// (scripts/test-cp-layout.sh) that reproduces the agentic-pipeline's
// CP-rooted / ./project layout with real local clones and asserts every
// filesystem invariant the recipe + execution skills depend on (#873).
//
// It is the regression gate for the control-plane-centralized layout: it
// fails if the recipe/AGENTS.md/docs stop resolving from $AGENTIC_CP_ROOT,
// if the project stops landing at ./project, or if the control-plane
// checkout stops staying clean (the read-only-CP invariant).
//
// The check needs bash + git; it is skipped where either is unavailable.
func TestCPLayoutStructure(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available — skipping CP layout structure check")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available — skipping CP layout structure check")
	}

	script := repoRelativePath(t, "scripts", "test-cp-layout.sh")

	out, err := exec.Command(bash, script).CombinedOutput()
	if err != nil {
		t.Fatalf("CP layout structure check failed (%v):\n%s", err, out)
	}
	t.Logf("CP layout structure check passed:\n%s", out)
}
