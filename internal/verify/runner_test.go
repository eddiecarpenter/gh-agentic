package verify

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestRunVerify_AllPass_ReturnsNil(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult { return CheckResult{Name: "check1", Status: Pass} },
		func() CheckResult { return CheckResult{Name: "check2", Status: Pass} },
	}

	err := RunVerify(&buf, checks, nil)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "✔") {
		t.Errorf("expected ✔ in output, got: %s", output)
	}
	if !strings.Contains(output, "All checks passed") {
		t.Errorf("expected 'All checks passed' in output, got: %s", output)
	}
}

func TestRunVerify_WarningReturnsError(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult { return CheckResult{Name: "check1", Status: Pass} },
		func() CheckResult {
			return CheckResult{Name: "check2", Status: Warning, Message: "missing optional file"}
		},
	}

	err := RunVerify(&buf, checks, nil)
	if err == nil {
		t.Fatal("expected error for unresolved warning, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "⚠") {
		t.Errorf("expected ⚠ in output, got: %s", output)
	}
	if !strings.Contains(output, "1 passed") {
		t.Errorf("expected '1 passed' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 warnings") {
		t.Errorf("expected '1 warnings' in summary, got: %s", output)
	}
}

func TestRunVerify_FailReturnsError(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult {
			return CheckResult{Name: "check1", Status: Fail, Message: "critical issue"}
		},
	}

	err := RunVerify(&buf, checks, nil)
	if err == nil {
		t.Fatal("expected error for failure, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "✖") {
		t.Errorf("expected ✖ in output, got: %s", output)
	}
	if !strings.Contains(output, "1 failed") {
		t.Errorf("expected '1 failed' in summary, got: %s", output)
	}
}

func TestRunVerify_InlineRepair_AllPass_ShowsOKSuffix(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult { return CheckResult{Name: "check1", Status: Pass} },
		func() CheckResult { return CheckResult{Name: "check2", Status: Pass} },
	}

	repairFn := func(r CheckResult) *CheckResult {
		return &r // Should not be called for passing checks.
	}

	err := RunVerify(&buf, checks, repairFn)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	output := buf.String()
	// Each line should have "ok" suffix.
	if !strings.Contains(output, "ok") {
		t.Errorf("expected 'ok' suffix in output, got: %s", output)
	}
	if !strings.Contains(output, "All checks passed") {
		t.Errorf("expected 'All checks passed' in output, got: %s", output)
	}
	// No "Final state" reprint.
	if strings.Contains(output, "Final state") {
		t.Errorf("should not contain 'Final state' in inline repair mode, got: %s", output)
	}
}

func TestRunVerify_InlineRepair_FixesFail_ShowsFixedSuffix(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult {
			return CheckResult{Name: "check1", Status: Fail, Message: "missing file"}
		},
	}

	repairFn := func(r CheckResult) *CheckResult {
		return &CheckResult{Name: r.Name, Status: Pass, Message: "restored"}
	}

	err := RunVerify(&buf, checks, repairFn)
	if err != nil {
		t.Fatalf("expected nil error after repair, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "fixed") {
		t.Errorf("expected 'fixed' suffix in output, got: %s", output)
	}
	// No "Final state" reprint.
	if strings.Contains(output, "Final state") {
		t.Errorf("should not contain 'Final state' in inline repair mode, got: %s", output)
	}
	// No separate "Repairing" phase.
	if strings.Contains(output, "Repairing") {
		t.Errorf("should not contain 'Repairing' in inline repair mode, got: %s", output)
	}
}

func TestRunVerify_InlineRepair_RepairFails_ShowsStillFailing(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult {
			return CheckResult{Name: "check1", Status: Fail, Message: "broken"}
		},
	}

	repairFn := func(r CheckResult) *CheckResult {
		return &CheckResult{Name: r.Name, Status: Fail, Message: "still broken"}
	}

	err := RunVerify(&buf, checks, repairFn)
	if err == nil {
		t.Fatal("expected error when repair fails, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "still failing") {
		t.Errorf("expected 'still failing' suffix in output, got: %s", output)
	}
}

func TestRunVerify_InlineRepair_ManualAction_ShowsActionNeeded(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult {
			return CheckResult{Name: "runner-check", Status: ManualAction, Message: "see instructions"}
		},
	}

	repairFn := func(r CheckResult) *CheckResult {
		return &r // Should not be called for ManualAction.
	}

	err := RunVerify(&buf, checks, repairFn)
	if err != nil {
		t.Fatalf("expected nil error for ManualAction, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "action needed") {
		t.Errorf("expected 'action needed' suffix in output, got: %s", output)
	}
}

func TestRunVerify_InlineRepair_MixedResults_SummaryFormat(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult { return CheckResult{Name: "ok-check", Status: Pass} },
		func() CheckResult {
			return CheckResult{Name: "warn-check", Status: Warning, Message: "caution"}
		},
		func() CheckResult {
			return CheckResult{Name: "fail-check", Status: Fail, Message: "broken"}
		},
		func() CheckResult {
			return CheckResult{Name: "manual-check", Status: ManualAction, Message: "see instructions"}
		},
	}

	// Repair only fixes warnings, not failures.
	repairFn := func(r CheckResult) *CheckResult {
		if r.Status == Warning {
			return &CheckResult{Name: r.Name, Status: Pass}
		}
		return &r
	}

	err := RunVerify(&buf, checks, repairFn)
	if err == nil {
		t.Fatal("expected error for unresolved failure, got nil")
	}

	output := buf.String()
	// New summary format: N ok, M fixed, K still failing, L action needed
	if !strings.Contains(output, "1 ok") {
		t.Errorf("expected '1 ok' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 fixed") {
		t.Errorf("expected '1 fixed' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 still failing") {
		t.Errorf("expected '1 still failing' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 action needed") {
		t.Errorf("expected '1 action needed' in summary, got: %s", output)
	}
	// No "Final state" reprint.
	if strings.Contains(output, "Final state") {
		t.Errorf("should not contain 'Final state' in inline repair mode, got: %s", output)
	}
}

func TestRunVerify_InlineRepair_NoFinalStateReprint(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult {
			return CheckResult{Name: "fixable", Status: Fail, Message: "broken"}
		},
	}

	repairFn := func(r CheckResult) *CheckResult {
		return &CheckResult{Name: r.Name, Status: Pass, Message: "repaired"}
	}

	err := RunVerify(&buf, checks, repairFn)
	if err != nil {
		t.Fatalf("expected nil error after repair, got: %v", err)
	}

	output := buf.String()
	// The check name should appear exactly once (no reprint).
	count := strings.Count(output, "fixable")
	if count != 1 {
		t.Errorf("expected check name 'fixable' to appear once, got %d times in:\n%s", count, output)
	}
	// No "Final state" divider.
	if strings.Contains(output, "Final state") {
		t.Errorf("should not contain 'Final state' in inline repair mode, got: %s", output)
	}
	// No "Repairing" phase text.
	if strings.Contains(output, "Repairing") {
		t.Errorf("should not contain 'Repairing' in inline repair mode, got: %s", output)
	}
}

func TestRunVerify_EmptyChecks_ReturnsNil(t *testing.T) {
	var buf bytes.Buffer
	err := RunVerify(&buf, []CheckFunc{}, nil)
	if err != nil {
		t.Fatalf("expected nil error for empty checks, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "All checks passed") {
		t.Errorf("expected 'All checks passed' for empty checks, got: %s", output)
	}
}

func TestRunVerify_InlineRepair_FixesWarning(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult {
			return CheckResult{Name: "check1", Status: Warning, Message: "issue"}
		},
	}

	repairFn := func(r CheckResult) *CheckResult {
		return &CheckResult{Name: r.Name, Status: Pass, Message: "fixed"}
	}

	err := RunVerify(&buf, checks, repairFn)
	if err != nil {
		t.Fatalf("expected nil error after repair, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "fixed") {
		t.Errorf("expected 'fixed' in output, got: %s", output)
	}
}

// streamWriter captures each Write call as a separate entry so we can verify
// that output is produced progressively (one result per Write batch) rather
// than all at once after all checks complete.
type streamWriter struct {
	mu      sync.Mutex
	entries []string
}

func (sw *streamWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.entries = append(sw.entries, string(p))
	return len(p), nil
}

func TestRunVerify_StreamsResultsProgressively(t *testing.T) {
	// Track the order in which checks execute and results appear.
	var mu sync.Mutex
	var callOrder []string

	sw := &streamWriter{}

	// Each check records when it was called. Because printResult writes to sw
	// immediately after the check returns, we can verify that output for
	// check N appears before check N+1 executes.
	makeCheck := func(name string, status CheckStatus) CheckFunc {
		return func() CheckResult {
			mu.Lock()
			callOrder = append(callOrder, "call:"+name)
			mu.Unlock()
			return CheckResult{Name: name, Status: status}
		}
	}

	checks := []CheckFunc{
		makeCheck("alpha", Pass),
		makeCheck("bravo", Warning),
		makeCheck("charlie", Fail),
	}

	_ = RunVerify(sw, checks, nil)

	// Verify all three checks were called in order.
	if len(callOrder) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(callOrder))
	}
	expected := []string{"call:alpha", "call:bravo", "call:charlie"}
	for i, exp := range expected {
		if callOrder[i] != exp {
			t.Errorf("callOrder[%d] = %q, want %q", i, callOrder[i], exp)
		}
	}

	// Combine all stream entries into one string and verify results appear
	// in the order the checks executed.
	var combined strings.Builder
	for _, e := range sw.entries {
		combined.WriteString(e)
	}
	output := combined.String()

	alphaIdx := strings.Index(output, "alpha")
	bravoIdx := strings.Index(output, "bravo")
	charlieIdx := strings.Index(output, "charlie")

	if alphaIdx < 0 || bravoIdx < 0 || charlieIdx < 0 {
		t.Fatalf("expected all three check names in output, got: %s", output)
	}
	if alphaIdx >= bravoIdx || bravoIdx >= charlieIdx {
		t.Errorf("results not in progressive order: alpha@%d, bravo@%d, charlie@%d",
			alphaIdx, bravoIdx, charlieIdx)
	}
}

func TestRunVerify_InlineRepair_EachCheckPrintedOnce(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult { return CheckResult{Name: "check-alpha", Status: Pass} },
		func() CheckResult {
			return CheckResult{Name: "check-bravo", Status: Fail, Message: "broken"}
		},
		func() CheckResult { return CheckResult{Name: "check-charlie", Status: Pass} },
	}

	repairFn := func(r CheckResult) *CheckResult {
		return &CheckResult{Name: r.Name, Status: Pass, Message: "repaired"}
	}

	err := RunVerify(&buf, checks, repairFn)
	if err != nil {
		t.Fatalf("expected nil error after repair, got: %v", err)
	}

	output := buf.String()
	// Each check name should appear exactly once.
	for _, name := range []string{"check-alpha", "check-bravo", "check-charlie"} {
		count := strings.Count(output, name)
		if count != 1 {
			t.Errorf("expected %q to appear once, got %d in:\n%s", name, count, output)
		}
	}
}

func TestRunVerify_InlineRepair_RepairReturnsManualAction(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult {
			return CheckResult{Name: "check1", Status: Fail, Message: "broken"}
		},
	}

	repairFn := func(r CheckResult) *CheckResult {
		return &CheckResult{Name: r.Name, Status: ManualAction, Message: "needs manual steps"}
	}

	err := RunVerify(&buf, checks, repairFn)
	if err != nil {
		t.Fatalf("expected nil error for ManualAction result, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "action needed") {
		t.Errorf("expected 'action needed' suffix when repair returns ManualAction, got: %s", output)
	}
}
