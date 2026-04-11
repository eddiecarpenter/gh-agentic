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

func TestRunVerify_RepairFixesFail(t *testing.T) {
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
		t.Errorf("expected 'fixed' in output, got: %s", output)
	}
	if !strings.Contains(output, "All checks passed") {
		t.Errorf("expected 'All checks passed' after repair, got: %s", output)
	}
}

func TestRunVerify_RepairFixesWarning(t *testing.T) {
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
	if !strings.Contains(output, "All checks passed") {
		t.Errorf("expected 'All checks passed' after repair, got: %s", output)
	}
}

func TestRunVerify_RepairFailsToFix(t *testing.T) {
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

func TestRunVerify_MixedResults(t *testing.T) {
	var buf bytes.Buffer
	checks := []CheckFunc{
		func() CheckResult { return CheckResult{Name: "ok-check", Status: Pass} },
		func() CheckResult {
			return CheckResult{Name: "warn-check", Status: Warning, Message: "caution"}
		},
		func() CheckResult {
			return CheckResult{Name: "fail-check", Status: Fail, Message: "broken"}
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
	if !strings.Contains(output, "2 passed") {
		t.Errorf("expected '2 passed' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 repaired") {
		t.Errorf("expected '1 repaired' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 failed") {
		t.Errorf("expected '1 failed' in summary, got: %s", output)
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

func TestRunVerify_RepairPhaseStillShowsFinalState(t *testing.T) {
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
	// The initial check result should appear before the repair section.
	failIdx := strings.Index(output, "fixable")
	repairIdx := strings.Index(output, "Repairing")
	finalIdx := strings.Index(output, "Final state")

	if failIdx < 0 || repairIdx < 0 || finalIdx < 0 {
		t.Fatalf("expected initial result, repair section, and final state in output, got: %s", output)
	}
	if failIdx >= repairIdx {
		t.Errorf("initial result should appear before repair section: fail@%d, repair@%d",
			failIdx, repairIdx)
	}
	if repairIdx >= finalIdx {
		t.Errorf("repair section should appear before final state: repair@%d, final@%d",
			repairIdx, finalIdx)
	}
}
