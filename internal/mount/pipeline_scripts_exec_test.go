package mount

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Layer 3 — execute the deterministic run: blocks against stubbed
// external binaries (gh, curl) and assert their behaviour.
//
// We cannot exercise the Goose-running steps (they shell out to
// `goose run` which calls Claude) nor the steps that depend on a
// real GitHub API surface for non-deterministic data. But the glue
// around them — branch-name parsing, label-decision logic, paginated
// curl branch lookup, parent-requirement extraction — is pure logic
// and deserves execution coverage so a typo doesn't survive to a
// live pipeline failure.

// scriptByName extracts a single run: block matching the given
// job-id and step-name from agentic-pipeline.yml. Fails the test if
// no match.
func scriptByName(t *testing.T, jobID, stepName string) string {
	t.Helper()
	path := reusableWorkflowPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	jobs := doc["jobs"].(map[string]any)
	job, ok := jobs[jobID].(map[string]any)
	if !ok {
		t.Fatalf("no job %q in workflow", jobID)
	}
	steps := job["steps"].([]any)
	for _, s := range steps {
		m := s.(map[string]any)
		name, _ := m["name"].(string)
		if name == stepName {
			run, _ := m["run"].(string)
			if run == "" {
				t.Fatalf("step %q has no run: block", stepName)
			}
			return run
		}
	}
	t.Fatalf("no step named %q in job %q", stepName, jobID)
	return ""
}

// substituteExpr replaces every ${{ key }} occurrence with the value
// from subs. Keys must match the inner trimmed text of the expression
// (e.g. "github.event.issue.number"). Unmatched expressions are left
// in place — the caller should provide every value the script reads
// or the script will crash.
func substituteExpr(script string, subs map[string]string) string {
	for key, val := range subs {
		marker := "${{ " + key + " }}"
		script = strings.ReplaceAll(script, marker, val)
	}
	return script
}

// runScript executes a bash script with the given env and a PATH
// rooted at stubDir. Returns stdout, stderr, exit code.
//
// The PATH includes stubDir first (so stubs win), then /usr/bin and /bin
// for universal POSIX tools, then any extra directories needed for tools
// whose feature set diverges across platforms (e.g. grep -P needs GNU
// grep, which is at /opt/homebrew/bin on macOS).
func runScript(t *testing.T, script, stubDir string, env map[string]string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("bash", "-c", script)
	// Empty environment + curated stubs only — no pollution from the
	// developer's shell.
	// On macOS, homebrew GNU tool variants (notably GNU grep with -P
	// support) live at /opt/homebrew/bin. We put them BEFORE /usr/bin so
	// our test environment behaves like a Linux runner — the workflow
	// targets Linux, so testing against Linux-compatible tools is the
	// honest test. On Linux these directories are usually absent;
	// including them is harmless.
	pathDirs := []string{stubDir, "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin"}
	cmd.Env = []string{
		"PATH=" + strings.Join(pathDirs, ":"),
		"HOME=" + t.TempDir(),
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("script execution error: %v", err)
		}
	}
	return stdout.String(), stderr.String(), code
}

// writeStub creates an executable shell script at stubDir/name with
// the given body.
func writeStub(t *testing.T, stubDir, name, body string) {
	t.Helper()
	path := filepath.Join(stubDir, name)
	if err := os.WriteFile(path, []byte("#!/bin/bash\n"+body), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", path, err)
	}
}

// stubDir creates a temp dir for stubs and a GITHUB_OUTPUT file.
func makeStubDir(t *testing.T) (string, string) {
	t.Helper()
	stubDir := t.TempDir()
	githubOutput := filepath.Join(t.TempDir(), "github_output")
	if err := os.WriteFile(githubOutput, nil, 0o644); err != nil {
		t.Fatalf("create GITHUB_OUTPUT: %v", err)
	}
	return stubDir, githubOutput
}

// readOutput reads the GITHUB_OUTPUT file into a map.
func readOutput(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read GITHUB_OUTPUT: %v", err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "="); i > 0 {
			out[line[:i]] = line[i+1:]
		}
	}
	return out
}

// -------------------------------------------------------------------
// Test: feature-complete "Extract feature issue number"
//
// Pure sed parsing — no external dependencies. The only thing that
// can go wrong is a regex typo.
// -------------------------------------------------------------------

func TestExtractFeatureIssueNumber(t *testing.T) {
	script := scriptByName(t, "feature-complete", "Extract feature issue number")

	cases := []struct {
		name    string
		branch  string
		want    string
	}{
		{"simple", "feature/42-add-login", "42"},
		{"multi-word", "feature/123-some-complex-description", "123"},
		{"single-digit", "feature/7-x", "7"},
		{"large-number", "feature/99999-x", "99999"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubDir, ghOut := makeStubDir(t)
			env := map[string]string{
				"PR_HEAD_REF":   tc.branch,
				"GITHUB_OUTPUT": ghOut,
			}
			_, stderr, code := runScript(t, script, stubDir, env)
			if code != 0 {
				t.Fatalf("script exited %d: %s", code, stderr)
			}
			out := readOutput(t, ghOut)
			if got := out["issue_number"]; got != tc.want {
				t.Errorf("issue_number = %q, want %q", got, tc.want)
			}
		})
	}
}

// -------------------------------------------------------------------
// Test: dev-session "Resolve feature branch"
//
// Calls curl against the GitHub API and parses the JSON branch list
// with jq. Mock curl to return a fixed JSON payload; verify the script
// extracts the right branch name.
// -------------------------------------------------------------------

func TestResolveFeatureBranch_HappyPath(t *testing.T) {
	script := scriptByName(t, "dev-session", "Resolve feature branch")

	stubDir, ghOut := makeStubDir(t)
	// curl stub returns a JSON branch list when page=1; empty page=2.
	writeStub(t, stubDir, "curl", `
# Last arg is the URL. Extract the page query parameter.
URL="${@: -1}"
case "$URL" in
  *\&page=1)
    cat <<'JSON'
[
  {"name": "main"},
  {"name": "feature/41-other-thing"},
  {"name": "feature/42-add-login"},
  {"name": "develop"}
]
JSON
    ;;
  *\&page=2)
    echo '[]'
    ;;
esac
`)
	// jq is real (universal command).

	env := map[string]string{
		"GH_TOKEN":      "stub-token",
		"REPO":          "owner/repo",
		"ISSUE":         "42",
		"GITHUB_OUTPUT": ghOut,
	}
	stdout, stderr, code := runScript(t, script, stubDir, env)
	if code != 0 {
		t.Fatalf("script exited %d: stdout=%s stderr=%s", code, stdout, stderr)
	}
	out := readOutput(t, ghOut)
	if got, want := out["name"], "feature/42-add-login"; got != want {
		t.Errorf("name = %q, want %q (stdout=%q)", got, want, stdout)
	}
}

func TestResolveFeatureBranch_NoMatch(t *testing.T) {
	script := scriptByName(t, "dev-session", "Resolve feature branch")

	stubDir, ghOut := makeStubDir(t)
	writeStub(t, stubDir, "curl", `
URL="${@: -1}"
case "$URL" in
  *\&page=1)
    cat <<'JSON'
[
  {"name": "main"},
  {"name": "feature/41-other-thing"},
  {"name": "develop"}
]
JSON
    ;;
  *\&page=2)
    echo '[]'
    ;;
esac
`)

	env := map[string]string{
		"GH_TOKEN":      "stub-token",
		"REPO":          "owner/repo",
		"ISSUE":         "999", // no branch matches
		"GITHUB_OUTPUT": ghOut,
	}
	stdout, stderr, code := runScript(t, script, stubDir, env)
	if code == 0 {
		t.Errorf("expected non-zero exit when no branch found, got 0")
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "No branch matching feature/999-* found") {
		t.Errorf("expected ::error:: message about missing branch, got stdout=%q stderr=%q", stdout, stderr)
	}
}

// Verifies the paginated loop: branch found on page 2 of 2.
func TestResolveFeatureBranch_PaginatedHit(t *testing.T) {
	script := scriptByName(t, "dev-session", "Resolve feature branch")

	stubDir, ghOut := makeStubDir(t)
	writeStub(t, stubDir, "curl", `
URL="${@: -1}"
case "$URL" in
  *\&page=1)
    # 100 branches, none matching → triggers pagination.
    echo -n '['
    for i in $(seq 1 99); do
      printf '{"name":"feat-%s"},' "$i"
    done
    printf '{"name":"feat-100"}'
    echo ']'
    ;;
  *\&page=2)
    echo '[{"name":"feature/77-found-it"}]'
    ;;
esac
`)

	env := map[string]string{
		"GH_TOKEN":      "stub-token",
		"REPO":          "owner/repo",
		"ISSUE":         "77",
		"GITHUB_OUTPUT": ghOut,
	}
	stdout, stderr, code := runScript(t, script, stubDir, env)
	if code != 0 {
		t.Fatalf("script exited %d: stdout=%s stderr=%s", code, stdout, stderr)
	}
	out := readOutput(t, ghOut)
	if got, want := out["name"], "feature/77-found-it"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
}

// -------------------------------------------------------------------
// Test: feature-complete "Parse parent requirement"
//
// Grep -P parses "Closes #N" / "Closes owner/repo#N" /
// "Closes part of owner/repo#N" from the issue body. The regex is
// non-trivial; this test pins the supported syntaxes.
// -------------------------------------------------------------------

// hostHasGNUGrep returns true if `grep -P` is supported on the host —
// production runners (Linux) always have it; macOS without homebrew GNU
// grep does not.
func hostHasGNUGrep() bool {
	cmd := exec.Command("grep", "-oP", `\d+`)
	cmd.Stdin = strings.NewReader("x 1")
	return cmd.Run() == nil
}

func TestParseParentRequirement(t *testing.T) {
	if !hostHasGNUGrep() {
		t.Skip("grep -P (PCRE) not supported on host — install GNU grep " +
			"to exercise this test locally. The workflow targets Linux runners " +
			"which always have GNU grep; this is purely a developer-machine skip.")
	}
	script := scriptByName(t, "feature-complete", "Parse parent requirement")

	cases := []struct {
		name        string
		issueBody   string
		ghLabels    string // label list returned by `gh issue view PARENT --json labels`
		wantParent  string
	}{
		{
			name:       "closes-N",
			issueBody:  "Closes #42\n\nSome other text",
			ghLabels:   "requirement\nbacklog",
			wantParent: "42",
		},
		{
			name:       "closes-cross-repo",
			issueBody:  "Closes eddiecarpenter/gh-agentic#123",
			ghLabels:   "requirement",
			wantParent: "123",
		},
		{
			name:       "closes-part-of",
			issueBody:  "Closes part of eddiecarpenter/gh-agentic#777",
			ghLabels:   "requirement",
			wantParent: "777",
		},
		{
			name:       "no-closes",
			issueBody:  "This issue is unrelated to any requirement.",
			ghLabels:   "",
			wantParent: "", // step exits cleanly without writing requirement_number
		},
		{
			name:       "not-a-requirement-label",
			issueBody:  "Closes #42",
			ghLabels:   "bug\nfeature", // no "requirement" label on parent
			wantParent: "",              // step exits cleanly without writing requirement_number
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubDir, ghOut := makeStubDir(t)
			// gh stub: handles both `gh issue view N --json body` and
			// `gh issue view N --json labels`.
			writeStub(t, stubDir, "gh", `
# Look for --json body or --json labels.
ARGS="$@"
if echo "$ARGS" | grep -q -- '--json body'; then
  cat <<'BODY'
`+tc.issueBody+`
BODY
elif echo "$ARGS" | grep -q -- '--json labels'; then
  printf '%s\n' '`+tc.ghLabels+`'
else
  echo "stub gh: unhandled args: $ARGS" >&2
  exit 1
fi
`)

			env := map[string]string{
				"GH_TOKEN":      "stub-token",
				"REPO":          "owner/repo",
				"AGENTIC_REPO":  "owner/repo",
				"GITHUB_OUTPUT": ghOut,
			}
			// The step references ${{ steps.feature.outputs.issue_number }}.
			// Substitute that with "1" (the feature issue number, doesn't matter
			// for this test).
			sub := substituteExpr(script, map[string]string{
				"steps.feature.outputs.issue_number": "1",
			})
			stdout, stderr, code := runScript(t, sub, stubDir, env)
			if code != 0 {
				t.Fatalf("script exited %d: stdout=%s stderr=%s", code, stdout, stderr)
			}
			out := readOutput(t, ghOut)
			if got := out["requirement_number"]; got != tc.wantParent {
				t.Errorf("requirement_number = %q, want %q", got, tc.wantParent)
			}
		})
	}
}

// -------------------------------------------------------------------
// Test: dev-session "Apply in-verification label" — skip path
//
// When the dev session produced no commits (git rev-list --count = 0),
// the step must NOT apply the label and must write skipped=true.
// -------------------------------------------------------------------

func TestApplyInVerification_SkipWhenNoCommits(t *testing.T) {
	script := scriptByName(t, "dev-session", "Apply in-verification label")

	stubDir, ghOut := makeStubDir(t)
	// git rev-list returns 0
	writeStub(t, stubDir, "git", `
if [ "$1" = "rev-list" ]; then echo 0; fi
`)
	// gh should NEVER be called in skip path
	writeStub(t, stubDir, "gh", `
echo "ERROR: gh was called in skip path with: $@" >&2
exit 99
`)

	env := map[string]string{
		"GH_TOKEN":      "stub-token",
		"GITHUB_OUTPUT": ghOut,
	}
	// substitute ${{ github.event.issue.number }} → 42
	sub := substituteExpr(script, map[string]string{
		"github.event.issue.number": "42",
	})
	stdout, stderr, code := runScript(t, sub, stubDir, env)
	if code != 0 {
		t.Fatalf("script exited %d: stdout=%s stderr=%s", code, stdout, stderr)
	}
	out := readOutput(t, ghOut)
	if got, want := out["skipped"], "true"; got != want {
		t.Errorf("skipped = %q, want %q", got, want)
	}
	if !strings.Contains(stdout, "No commits") {
		t.Errorf("expected 'No commits' message, got stdout=%q", stdout)
	}
}

// Happy path: commits exist, gh is invoked with the expected args, skipped=false.
func TestApplyInVerification_HappyPath(t *testing.T) {
	script := scriptByName(t, "dev-session", "Apply in-verification label")

	stubDir, ghOut := makeStubDir(t)
	writeStub(t, stubDir, "git", `
if [ "$1" = "rev-list" ]; then echo 3; fi
`)
	// Capture gh args to a file for assertion.
	ghLog := filepath.Join(t.TempDir(), "gh.log")
	writeStub(t, stubDir, "gh", `echo "$@" >> `+ghLog+`
`)

	env := map[string]string{
		"GH_TOKEN":      "stub-token",
		"GITHUB_OUTPUT": ghOut,
	}
	sub := substituteExpr(script, map[string]string{
		"github.event.issue.number": "42",
	})
	stdout, stderr, code := runScript(t, sub, stubDir, env)
	if code != 0 {
		t.Fatalf("script exited %d: stdout=%s stderr=%s", code, stdout, stderr)
	}
	out := readOutput(t, ghOut)
	if got, want := out["skipped"], "false"; got != want {
		t.Errorf("skipped = %q, want %q", got, want)
	}
	logData, _ := os.ReadFile(ghLog)
	logStr := string(logData)
	if !strings.Contains(logStr, "issue edit 42") {
		t.Errorf("gh log did not include 'issue edit 42': %s", logStr)
	}
	if !strings.Contains(logStr, "--remove-label in-development") {
		t.Errorf("gh log did not include '--remove-label in-development': %s", logStr)
	}
	if !strings.Contains(logStr, "--add-label in-verification") {
		t.Errorf("gh log did not include '--add-label in-verification': %s", logStr)
	}
}
