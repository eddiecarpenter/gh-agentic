// Package verify implements the business logic for gh agentic verify.
// It checks an existing agentic environment for correctness and repairs
// what it can automatically.
package verify

// CheckStatus represents the outcome of a single verification check.
type CheckStatus int

const (
	// Pass indicates the check passed successfully.
	Pass CheckStatus = iota
	// Warning indicates a non-critical issue was found.
	Warning
	// Fail indicates a critical issue was found.
	Fail
)

// String returns a human-readable label for the check status.
func (s CheckStatus) String() string {
	switch s {
	case Pass:
		return "pass"
	case Warning:
		return "warning"
	case Fail:
		return "fail"
	default:
		return "unknown"
	}
}

// CheckResult holds the outcome of a single verification check.
type CheckResult struct {
	// Name is the human-readable label for this check.
	Name string
	// Status is the outcome: Pass, Warning, or Fail.
	Status CheckStatus
	// Message provides additional detail about the outcome.
	Message string
}

// CheckFunc is the signature for a verification check function.
// Each check returns a CheckResult describing whether it passed, warned, or failed.
type CheckFunc func() CheckResult

// RepairFunc attempts to repair a failed or warning check and returns an
// updated CheckResult. If repair is not possible, it returns the original result.
type RepairFunc func(CheckResult) *CheckResult
