package verify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckCLAUDEMD_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# CLAUDE.md"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckCLAUDEMD(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckCLAUDEMD_Missing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckCLAUDEMD(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v", result.Status)
	}
}

func TestCheckAGENTSLocalMD_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.local.md"), []byte("# local"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckAGENTSLocalMD(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckAGENTSLocalMD_Missing_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	result := CheckAGENTSLocalMD(root)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v", result.Status)
	}
}

func TestCheckTEMPLATESOURCE_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_SOURCE"), []byte("eddiecarpenter/agentic-development\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckTEMPLATESOURCE(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckTEMPLATESOURCE_Missing_ReturnsWarning(t *testing.T) {
	root := t.TempDir()
	result := CheckTEMPLATESOURCE(root)
	if result.Status != Warning {
		t.Errorf("expected Warning, got %v", result.Status)
	}
}

func TestCheckTEMPLATEVERSION_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "TEMPLATE_VERSION"), []byte("v1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckTEMPLATEVERSION(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckTEMPLATEVERSION_Missing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckTEMPLATEVERSION(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v", result.Status)
	}
}

func TestCheckREPOSMD_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "REPOS.md"), []byte("# REPOS"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckREPOSMD(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckREPOSMD_Missing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckREPOSMD(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v", result.Status)
	}
}

func TestCheckREADMEMD_Present_ReturnsPass(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# README"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := CheckREADMEMD(root)
	if result.Status != Pass {
		t.Errorf("expected Pass, got %v: %s", result.Status, result.Message)
	}
}

func TestCheckREADMEMD_Missing_ReturnsFail(t *testing.T) {
	root := t.TempDir()
	result := CheckREADMEMD(root)
	if result.Status != Fail {
		t.Errorf("expected Fail, got %v", result.Status)
	}
}

// Table-driven test covering all file checks.
func TestFileChecks_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		checkFn      func(string) CheckResult
		missingState CheckStatus
	}{
		{"CLAUDE.md", "CLAUDE.md", CheckCLAUDEMD, Fail},
		{"AGENTS.local.md", "AGENTS.local.md", CheckAGENTSLocalMD, Warning},
		{"TEMPLATE_SOURCE", "TEMPLATE_SOURCE", CheckTEMPLATESOURCE, Warning},
		{"TEMPLATE_VERSION", "TEMPLATE_VERSION", CheckTEMPLATEVERSION, Fail},
		{"REPOS.md", "REPOS.md", CheckREPOSMD, Fail},
		{"README.md", "README.md", CheckREADMEMD, Fail},
	}

	for _, tc := range tests {
		t.Run(tc.name+"_present", func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, tc.filename), []byte("content"), 0o644); err != nil {
				t.Fatal(err)
			}
			result := tc.checkFn(root)
			if result.Status != Pass {
				t.Errorf("expected Pass when file present, got %v: %s", result.Status, result.Message)
			}
		})

		t.Run(tc.name+"_missing", func(t *testing.T) {
			root := t.TempDir()
			result := tc.checkFn(root)
			if result.Status != tc.missingState {
				t.Errorf("expected %v when file missing, got %v: %s", tc.missingState, result.Status, result.Message)
			}
		})
	}
}
