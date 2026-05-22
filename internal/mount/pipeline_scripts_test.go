package mount

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Pipeline run-block execution tests.
//
// Extracts every `run:` block from the reusable workflow + every
// composite action, and exercises them in three layers:
//
//   Layer 1 — bash -n: every script parses as valid bash.
//   Layer 2 — shellcheck: deeper static analysis of every script.
//             Errors fail the test; informational warnings (SC2086 etc.)
//             are surfaced but do not fail.
//   Layer 3 — execution: a subset of deterministic scripts (branch
//             extraction, label-decision, etc.) are run against stubbed
//             gh / curl / git binaries to assert their behaviour.
//
// Layer 3 cannot cover the Goose-running steps (they invoke `goose run`
// against Claude) or the steps that depend on a real GitHub API surface.
// Those are exercised by the live pipeline; the unit-level coverage
// here is for the deterministic glue around them.

// extractRunBlocks walks the workflow YAML and returns every `run:`
// block with a context label (job-or-action / step-name).
func extractRunBlocks(t *testing.T, path string) []runBlock {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	// Generic YAML decoding — we use map[string]any so the function works
	// for both reusable workflows (jobs.X.steps) and composite actions
	// (runs.steps).
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var out []runBlock

	// Reusable-workflow shape: jobs.<id>.steps[]
	if jobs, ok := doc["jobs"].(map[string]any); ok {
		for jobID, job := range jobs {
			steps := stepsOf(job)
			for i, step := range steps {
				if s := runOf(step); s != "" {
					out = append(out, runBlock{
						Context: "jobs/" + jobID + "/" + stepName(step, i),
						Script:  s,
					})
				}
			}
		}
	}

	// Composite-action shape: runs.steps[]
	if runs, ok := doc["runs"].(map[string]any); ok {
		if steps, ok := runs["steps"].([]any); ok {
			for i, step := range steps {
				if s := runOf(step); s != "" {
					out = append(out, runBlock{
						Context: filepath.Base(filepath.Dir(path)) + "/" + stepName(step, i),
						Script:  s,
					})
				}
			}
		}
	}

	return out
}

type runBlock struct {
	Context string
	Script  string
}

func stepsOf(job any) []any {
	m, ok := job.(map[string]any)
	if !ok {
		return nil
	}
	steps, _ := m["steps"].([]any)
	return steps
}

func runOf(step any) string {
	m, ok := step.(map[string]any)
	if !ok {
		return ""
	}
	r, _ := m["run"].(string)
	return r
}

func stepName(step any, idx int) string {
	m, ok := step.(map[string]any)
	if !ok {
		return "step-" + itoa(idx)
	}
	if n, _ := m["name"].(string); n != "" {
		return sanitize(n)
	}
	return "step-" + itoa(idx)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}

// stripGitHubExpressions replaces every ${{ … }} with a shell-safe
// literal so the script becomes valid bash. GitHub Actions evaluates
// these substitutions before the script runs; for syntax checking
// they need to be neutralised first.
func stripGitHubExpressions(script string) string {
	out := strings.Builder{}
	i := 0
	for i < len(script) {
		if i+2 < len(script) && script[i] == '$' && script[i+1] == '{' && script[i+2] == '{' {
			// Find the closing }}
			end := strings.Index(script[i:], "}}")
			if end < 0 {
				// Unclosed — copy the rest verbatim.
				out.WriteString(script[i:])
				break
			}
			// Replace with a placeholder that is bash-safe in expansion
			// and assignment contexts.
			out.WriteString("__GHA_EXPR__")
			i += end + 2
			continue
		}
		out.WriteByte(script[i])
		i++
	}
	return out.String()
}

// TestEveryRunBlockSyntax — Layer 1: every run: block parses as valid bash.
func TestEveryRunBlockSyntax(t *testing.T) {
	paths := allWorkflowAndActionPaths(t)
	totalBlocks := 0
	for _, path := range paths {
		blocks := extractRunBlocks(t, path)
		for _, b := range blocks {
			totalBlocks++
			t.Run(filepath.Base(path)+"/"+b.Context, func(t *testing.T) {
				cleaned := stripGitHubExpressions(b.Script)
				cmd := exec.Command("bash", "-n")
				cmd.Stdin = strings.NewReader(cleaned)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Errorf("bash -n rejected script in %s:\n%s\n--- script ---\n%s",
						b.Context, string(out), cleaned)
				}
			})
		}
	}
	if totalBlocks == 0 {
		t.Fatalf("no run: blocks discovered — extraction is broken")
	}
	t.Logf("checked %d run: blocks across %d files", totalBlocks, len(paths))
}

// TestEveryRunBlockShellcheck — Layer 2: shellcheck errors fail the
// test. Lower-severity warnings (info/style) are accepted because the
// workflow includes many SC2086 informationals that we accept as
// intentional (heredoc interpolation, label name args, etc.).
func TestEveryRunBlockShellcheck(t *testing.T) {
	if _, err := exec.LookPath("shellcheck"); err != nil {
		t.Skip("shellcheck not installed — Layer 2 test skipped")
	}
	paths := allWorkflowAndActionPaths(t)
	for _, path := range paths {
		blocks := extractRunBlocks(t, path)
		for _, b := range blocks {
			t.Run(filepath.Base(path)+"/"+b.Context, func(t *testing.T) {
				cleaned := stripGitHubExpressions(b.Script)
				// --severity=error → only errors are surfaced.
				// --shell=bash → matches the workflow's `shell: bash`.
				cmd := exec.Command("shellcheck",
					"--severity=error",
					"--shell=bash",
					"--external-sources",
					"-")
				cmd.Stdin = strings.NewReader(cleaned)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Errorf("shellcheck found errors in %s:\n%s\n--- script ---\n%s",
						b.Context, string(out), cleaned)
				}
			})
		}
	}
}

// allWorkflowAndActionPaths returns the reusable workflow path plus
// every composite action.yml under .github/actions/.
func allWorkflowAndActionPaths(t *testing.T) []string {
	t.Helper()
	out := []string{reusableWorkflowPath(t)}
	actionsDir := compositeActionsDir(t)
	entries, err := os.ReadDir(actionsDir)
	if err != nil {
		t.Fatalf("read %s: %v", actionsDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ap := filepath.Join(actionsDir, e.Name(), "action.yml")
		if _, err := os.Stat(ap); err == nil {
			out = append(out, ap)
		}
	}
	return out
}
