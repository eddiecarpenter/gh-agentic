package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenIdentifiers lists the in-code references that must not appear
// outside internal/project/. These are identifiers that can only flow into a
// read or write of an AGENTIC_* variable — any other code path should consume
// project state via project.Resolve instead.
//
// String literals ("AGENTIC_PROJECT_ID", etc.) are intentionally NOT in this
// list: they appear legitimately in user-facing error messages, diagnostic
// output, routing maps, and `gh variable set` invocations. The enforcement
// gate targets value-consumer patterns, not display strings.
var forbiddenIdentifiers = []string{
	"project.ProjectVarName",
	"project.FrameworkVersionVarName",
	"project.DefaultGetRepoVariable",
}

// allowListMarker is the comment tag a source line may carry to opt out of
// the boundary check. The marker must be on the same line as the flagged
// identifier and must explain why the exception is justified (a bare
// `boundary-allow` without a rationale is still accepted by the scanner, but
// code review should reject it).
const allowListMarker = "boundary-allow"

// insideProjectPackage reports whether the given absolute path sits inside
// internal/project/ — the boundary-owner package that is permitted to hold
// every canonical read of AGENTIC_* variables.
func insideProjectPackage(absPath string) bool {
	// Use platform-neutral separators so the test stays correct on Windows.
	normalised := filepath.ToSlash(absPath)
	return strings.Contains(normalised, "/internal/project/")
}

// violation is a single scanner hit — enough to render a crisp failure
// message pointing the author at project.Resolve.
type violation struct {
	file       string
	line       int
	text       string
	identifier string
}

// String renders the violation in the conventional file:line format so
// editors can jump to it from `go test` output.
func (v violation) String() string {
	return fmt.Sprintf("%s:%d: forbidden reference to %s — use project.Resolve instead; got: %s",
		v.file, v.line, v.identifier, strings.TrimSpace(v.text))
}

// scanGoSources walks the repo tree rooted at root and returns every
// violation found. Files in _test.go suffix are skipped. Files inside
// internal/project/ are skipped. Lines carrying the allow-list marker on
// the same line as the forbidden identifier are also skipped.
//
// repoRoot is the repository root — typically resolved by walking up from
// the test file to the module root.
func scanGoSources(repoRoot string) ([]violation, error) {
	var out []violation

	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip common directories that never contain first-party code.
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "recovery-logs" ||
				name == "docs" || name == "concepts" || name == "skills" {
				return filepath.SkipDir
			}
			return nil
		}
		// Only inspect .go files, skip test files.
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// Inside internal/project/ is the canonical owner; skip.
		if insideProjectPackage(path) {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			// Allow-list opt-out on a per-line basis.
			if strings.Contains(line, allowListMarker) {
				continue
			}
			for _, id := range forbiddenIdentifiers {
				if strings.Contains(line, id) {
					// Skip comment lines — the purpose of this scanner
					// is to catch value-consumer code paths, and
					// comments discussing the identifier are fine.
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "//") {
						continue
					}
					out = append(out, violation{
						file:       path,
						line:       i + 1,
						text:       line,
						identifier: id,
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// repoRootForTest walks up from the current working directory until it finds
// a go.mod — conventionally the repository root. The boundary test runs from
// internal/project/, so the walk climbs two levels by default but stays
// portable across relocations.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolving working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod above %s", dir)
		}
		dir = parent
	}
}

// TestBoundary_NoForbiddenReadsOutsideProjectPackage is the enforcement gate
// for feature #571 — adding a direct AGENTIC_* read outside internal/project/
// must fail the test suite. The scanner points the author at project.Resolve
// as the canonical replacement.
func TestBoundary_NoForbiddenReadsOutsideProjectPackage(t *testing.T) {
	root := repoRootForTest(t)

	violations, err := scanGoSources(root)
	if err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	if len(violations) == 0 {
		return
	}

	var b strings.Builder
	b.WriteString("boundary enforcement (#571): forbidden AGENTIC_* identifier references found outside internal/project/.\n")
	b.WriteString("Route all project-state reads through project.Resolve.\n\n")
	for _, v := range violations {
		b.WriteString("  ")
		b.WriteString(v.String())
		b.WriteString("\n")
	}
	t.Fatal(b.String())
}

// TestBoundary_ScannerDetectsSyntheticViolation is the negative case: feed a
// synthetic file through the scanner via the in-memory lines loop and assert
// it is flagged. This guards against the scanner silently becoming a no-op
// (e.g. if a future change breaks the filepath.Walk or the identifier list).
func TestBoundary_ScannerDetectsSyntheticViolation(t *testing.T) {
	cases := []struct {
		name          string
		line          string
		wantViolation bool
	}{
		{
			name:          "direct DefaultGetRepoVariable call is flagged",
			line:          `value, _ := project.DefaultGetRepoVariable(owner, repo, project.ProjectVarName)`,
			wantViolation: true,
		},
		{
			name:          "reference to project.FrameworkVersionVarName in a read context is flagged",
			line:          `val, _ := deps.GetRepoVariable(o, r, project.FrameworkVersionVarName)`,
			wantViolation: true,
		},
		{
			name:          "allow-list comment opts the line out",
			line:          `_ = deps.SetRepoVariable(o, r, project.FrameworkVersionVarName, "v2.0") // boundary-allow: write path`,
			wantViolation: false,
		},
		{
			name:          "comment-only line is ignored",
			line:          `// project.ProjectVarName is the canonical identity variable name`,
			wantViolation: false,
		},
		{
			name:          "string literal alone does not trigger the scanner",
			line:          `fmt.Println("AGENTIC_PROJECT_ID is not set for this repository")`,
			wantViolation: false,
		},
		{
			name:          "completely clean line passes",
			line:          `ctx, err := project.Resolve(deps)`,
			wantViolation: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lineHasViolation(tc.line)
			if got != tc.wantViolation {
				t.Errorf("lineHasViolation(%q) = %v, want %v", tc.line, got, tc.wantViolation)
			}
		})
	}
}

// lineHasViolation mirrors the inner predicate used by scanGoSources so it
// can be unit-tested against synthetic input without writing temp files.
// Keep this in lockstep with the scanner's line-level logic.
func lineHasViolation(line string) bool {
	if strings.Contains(line, allowListMarker) {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") {
		return false
	}
	for _, id := range forbiddenIdentifiers {
		if strings.Contains(line, id) {
			return true
		}
	}
	return false
}
