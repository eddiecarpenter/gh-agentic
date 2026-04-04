package verify

import (
	"bytes"
	"strings"
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
	if !strings.Contains(output, "repaired") {
		t.Errorf("expected 'repaired' in output, got: %s", output)
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
	if !strings.Contains(output, "1 passed") {
		t.Errorf("expected '1 passed' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 repaired") {
		t.Errorf("expected '1 repaired' in summary, got: %s", output)
	}
	if !strings.Contains(output, "1 failed") {
		t.Errorf("expected '1 failed' in summary, got: %s", output)
	}
}
