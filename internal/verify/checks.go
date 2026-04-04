package verify

import (
	"os"
	"path/filepath"
)

// CheckCLAUDEMD verifies that CLAUDE.md exists in the repo root.
// Returns Fail if the file is missing.
func CheckCLAUDEMD(root string) CheckResult {
	path := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "CLAUDE.md exists",
			Status:  Fail,
			Message: "file not found",
		}
	}
	return CheckResult{
		Name:   "CLAUDE.md exists",
		Status: Pass,
	}
}

// CheckAGENTSLocalMD verifies that AGENTS.local.md exists in the repo root.
// Returns Warning if the file is missing (it can be restored as a skeleton).
func CheckAGENTSLocalMD(root string) CheckResult {
	path := filepath.Join(root, "AGENTS.local.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "AGENTS.local.md exists",
			Status:  Warning,
			Message: "file not found — will restore minimal skeleton",
		}
	}
	return CheckResult{
		Name:   "AGENTS.local.md exists",
		Status: Pass,
	}
}

// CheckTEMPLATESOURCE verifies that TEMPLATE_SOURCE exists in the repo root.
// Returns Warning if the file is missing (requires user input to repair).
func CheckTEMPLATESOURCE(root string) CheckResult {
	path := filepath.Join(root, "TEMPLATE_SOURCE")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: "file not found — value must be provided",
		}
	}
	return CheckResult{
		Name:   "TEMPLATE_SOURCE exists",
		Status: Pass,
	}
}

// CheckTEMPLATEVERSION verifies that TEMPLATE_VERSION exists in the repo root.
// Returns Fail if the file is missing.
func CheckTEMPLATEVERSION(root string) CheckResult {
	path := filepath.Join(root, "TEMPLATE_VERSION")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: "file not found",
		}
	}
	return CheckResult{
		Name:   "TEMPLATE_VERSION exists",
		Status: Pass,
	}
}

// CheckREPOSMD verifies that REPOS.md exists in the repo root.
// Returns Fail if the file is missing.
func CheckREPOSMD(root string) CheckResult {
	path := filepath.Join(root, "REPOS.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "REPOS.md exists",
			Status:  Fail,
			Message: "file not found",
		}
	}
	return CheckResult{
		Name:   "REPOS.md exists",
		Status: Pass,
	}
}

// CheckREADMEMD verifies that README.md exists in the repo root.
// Returns Fail if the file is missing.
func CheckREADMEMD(root string) CheckResult {
	path := filepath.Join(root, "README.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "README.md exists",
			Status:  Fail,
			Message: "file not found",
		}
	}
	return CheckResult{
		Name:   "README.md exists",
		Status: Pass,
	}
}
