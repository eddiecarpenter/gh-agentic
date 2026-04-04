package verify

import "testing"

func TestCheckStatus_String(t *testing.T) {
	tests := []struct {
		name     string
		status   CheckStatus
		expected string
	}{
		{name: "Pass returns pass", status: Pass, expected: "pass"},
		{name: "Warning returns warning", status: Warning, expected: "warning"},
		{name: "Fail returns fail", status: Fail, expected: "fail"},
		{name: "Unknown returns unknown", status: CheckStatus(99), expected: "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.status.String(); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestCheckResult_Construction(t *testing.T) {
	result := CheckResult{
		Name:    "CLAUDE.md exists",
		Status:  Pass,
		Message: "",
	}
	if result.Name != "CLAUDE.md exists" {
		t.Errorf("expected name %q, got %q", "CLAUDE.md exists", result.Name)
	}
	if result.Status != Pass {
		t.Errorf("expected status Pass, got %v", result.Status)
	}
	if result.Message != "" {
		t.Errorf("expected empty message, got %q", result.Message)
	}
}

func TestCheckResult_FailWithMessage(t *testing.T) {
	result := CheckResult{
		Name:    "README.md exists",
		Status:  Fail,
		Message: "file not found",
	}
	if result.Status != Fail {
		t.Errorf("expected status Fail, got %v", result.Status)
	}
	if result.Message != "file not found" {
		t.Errorf("expected message %q, got %q", "file not found", result.Message)
	}
}
