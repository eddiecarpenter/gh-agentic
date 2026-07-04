package mount

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSafetyNetHonoursComplianceBlockedMarker — the compliance-verify
// safety-net step force-swaps a stuck `in-verification` Feature back to
// `in-development` when it suspects the skill aborted mid-FAIL. But
// compliance-verify Output D / Output F (e.g. STACK_GATE_UNDEFINED,
// toolchain absent) are DELIBERATE terminal blocks: the skill posts a
// `<!-- compliance-blocked:v1 -->` comment and intentionally keeps the
// Feature at in-verification without counting a cycle. The safety net
// must detect that marker and take no action — otherwise a re-firing
// dev-session produces an infinite in-development ↔ in-verification loop
// (observed live: OpenBSS/openbss run 27793325499).
//
// This test pins that the marker check exists and runs BEFORE the
// destructive cycle/exit-1 logic.
func TestSafetyNetHonoursComplianceBlockedMarker(t *testing.T) {
	wfPath := reusableWorkflowPath(t)
	data, err := os.ReadFile(wfPath)
	if err != nil {
		t.Fatalf("read %s: %v", wfPath, err)
	}
	var wf map[string]any
	if err := yaml.Unmarshal(data, &wf); err != nil {
		t.Fatalf("parse workflow: %v", err)
	}

	run := safetyNetRunScript(t, wf)

	// 1. The marker must be referenced at all.
	const marker = "compliance-blocked:v1"
	if !strings.Contains(run, marker) {
		t.Fatalf("safety-net step does not reference %q — deliberate blocks would be misclassified as aborts.\nScript:\n%s", marker, run)
	}

	// 2. The marker check must guard an early exit 0 that happens BEFORE
	//    the destructive `--add-label "in-development"` cycle.
	markerIdx := strings.Index(run, marker)
	cycleIdx := strings.Index(run, `--add-label "in-development"`)
	if cycleIdx < 0 {
		t.Fatalf("safety-net step no longer adds in-development — test assumptions are stale, review manually.\nScript:\n%s", run)
	}
	if markerIdx > cycleIdx {
		t.Errorf("compliance-blocked:v1 check appears AFTER the in-development cycle (marker@%d, cycle@%d) — the block guard must run first.\nScript:\n%s", markerIdx, cycleIdx, run)
	}

	// 3. There must be an early `exit 0` between the marker check and the
	//    cycle, so a blocked Feature is not force-swapped.
	between := run[markerIdx:cycleIdx]
	if !strings.Contains(between, "exit 0") {
		t.Errorf("no `exit 0` between the compliance-blocked:v1 check and the in-development cycle — a deliberate block would still be cycled.\nSegment:\n%s", between)
	}
}

// safetyNetRunScript locates the compliance-verify safety-net step and
// returns its `run:` script body.
func safetyNetRunScript(t *testing.T, wf map[string]any) string {
	t.Helper()
	jobs, ok := wf["jobs"].(map[string]any)
	if !ok {
		t.Fatalf("workflow has no jobs map")
	}
	job, ok := jobs["compliance-verify"].(map[string]any)
	if !ok {
		t.Fatalf("compliance-verify job missing")
	}
	steps, ok := job["steps"].([]any)
	if !ok {
		t.Fatalf("compliance-verify job has no steps")
	}
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		name, _ := step["name"].(string)
		if strings.Contains(name, "Safety net") && strings.Contains(name, "in-verification") {
			run, _ := step["run"].(string)
			if run == "" {
				t.Fatalf("safety-net step %q has empty run:", name)
			}
			return run
		}
	}
	t.Fatalf("safety-net step not found in compliance-verify job")
	return ""
}
