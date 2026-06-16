package doctor

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestRequiredLabelsCoverPipelineReferences scans the reusable pipeline
// workflow (`.github/workflows/agentic-pipeline.yml`) and every framework
// skill (`skills/**/SKILL.md`) for label references and asserts that every
// label name found is declared in `requiredPipelineLabels`.
//
// This is the drift-detection gate: when a workflow step or skill adds a
// new label transition without updating `requiredPipelineLabels`, this
// test fails. The doctor would otherwise silently report "all labels OK"
// on a repo where the new label is missing — exactly the class of bug
// that hit yapper with the missing `in-review` label.
//
// Scope of the scan:
//
//   - In agentic-pipeline.yml: any --add-label / --remove-label / --label
//     argument to `gh issue edit`, and any `'<label>'` literal inside an
//     `if:` expression that gates on `github.event.label.name`.
//   - In skills/**/SKILL.md: any `add=[...]` / `remove=[...]` argument
//     passed to apply-label / set-issue-status, and any quoted label
//     name in a "*-label" / "label:" instruction context.
//
// Scope of the assertion:
//
//   - Every label name discovered is checked against the
//     `requiredPipelineLabels` set. A discovered label not in the set is
//     a failure.
//   - The converse (a required label that is never referenced) is NOT
//     asserted — we deliberately allow the list to be a superset, since
//     some labels (e.g. concurrency beacons) are applied by the
//     foreground-recovery skill and may not appear in a grep here.
func TestRequiredLabelsCoverPipelineReferences(t *testing.T) {
	repoRoot := repoRoot(t)

	declared := map[string]bool{}
	for _, lbl := range requiredPipelineLabels {
		declared[lbl.Name] = true
	}

	referenced := map[string]bool{}
	scanFile(t, filepath.Join(repoRoot, ".github", "workflows", "agentic-pipeline.yml"), referenced)
	skillsDir := filepath.Join(repoRoot, "skills")
	if err := filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			scanFile(t, path, referenced)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", skillsDir, err)
	}

	if len(referenced) == 0 {
		t.Fatalf("no label references discovered — scanner is broken")
	}

	var missing []string
	for label := range referenced {
		if !declared[label] {
			missing = append(missing, label)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf(
			"the following label(s) are referenced by the pipeline workflow "+
				"or a framework skill but are NOT declared in "+
				"requiredPipelineLabels (internal/doctor/checks.go):\n  %s\n\n"+
				"`gh agentic check`/`repair` will fail to detect or create "+
				"these labels on a domain repo. Add a LabelDef for each "+
				"missing label to requiredPipelineLabels.",
			strings.Join(missing, ", "),
		)
	}
}

// scanFile extracts every label name from a file and populates `out`.
func scanFile(t *testing.T, path string, out map[string]bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, label := range extractLabels(string(data)) {
		out[label] = true
	}
}

// labelExtractionPatterns are the regex patterns that identify a label
// name in a workflow / skill file. Each pattern's capturing group #1 is
// the label name.
var labelExtractionPatterns = []*regexp.Regexp{
	// gh issue edit … --add-label X / --remove-label X / --label X
	regexp.MustCompile(`--(?:add-label|remove-label|label)\s+["']?([a-z][a-z0-9-]*)["']?`),
	// apply-label add=[X, Y, ...] or remove=[X, Y, ...]
	regexp.MustCompile(`(?:add|remove)\s*=\s*\[([^\]]+)\]`),
	// Workflow `if:` gates: == 'label-name'
	regexp.MustCompile(`==\s*['"]([a-z][a-z0-9-]+)['"]`),
	// contains(…, 'label-name')
	regexp.MustCompile(`contains\([^,]+,\s*['"]([a-z][a-z0-9-]+)['"]`),
}

// labelLikePattern matches a quoted label-like token. Used inside an
// add=[…] / remove=[…] capture to pull out the individual names.
var labelLikePattern = regexp.MustCompile(`["']([a-z][a-z0-9-]+)["']`)

// extractLabels runs every pattern against the input and returns the
// set of label names found. Aggressively filters: a token is only a
// label if it (a) matches the canonical naming convention
// (`^[a-z][a-z0-9-]+$` with at least one hyphen) OR (b) is in the
// known-single-word allowlist below.
func extractLabels(content string) []string {
	// Tokens that look like labels but are NOT (false positives the
	// patterns above otherwise pick up).
	deny := map[string]bool{
		// Generic GH event values
		"labeled":           true,
		"closed":            true,
		"opened":            true,
		"submitted":         true,
		"commented":         true,
		"merged":            true,
		"approved":          true,
		"changes-requested": true,
		"changes_requested": true,
		// Event payload field references / type values
		"issues":              true,
		"issue":               true,
		"pull_request":        true,
		"workflow_dispatch":   true,
		"workflow_call":       true,
		"pull_request_review": true,
		"pr-review-session":   true,
		"pull-request":        true,
		"label":               true,
		"labels":              true,
		// Other false positives observed
		"feature/": true, // `head.ref` filter
		"true":     true,
		"false":    true,
		"bot":      true,
		"user":     true,
		"v1":       true,
	}

	// Whitelisted single-word labels (no hyphen) that should be
	// captured. Without this, the canonical-naming filter would
	// drop them.
	singleWordLabels := map[string]bool{
		"backlog":     true,
		"scoping":     true,
		"requirement": true,
		"feature":     true,
		"task":        true,
		"designed":    true,
		"done":        true,
	}

	seen := map[string]bool{}
	for _, re := range labelExtractionPatterns {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			// Handle the add=[…] case: split on commas, extract each
			// quoted token.
			if strings.HasPrefix(re.String(), `(?:add|remove)\s*=`) {
				inner := m[1]
				for _, sub := range labelLikePattern.FindAllStringSubmatch(inner, -1) {
					if len(sub) >= 2 {
						candidate(sub[1], deny, singleWordLabels, seen)
					}
				}
				continue
			}
			candidate(m[1], deny, singleWordLabels, seen)
		}
	}

	out := make([]string, 0, len(seen))
	for label := range seen {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

// candidate applies the naming convention + allowlist filters and
// inserts the token into `seen` if it qualifies.
func candidate(tok string, deny, singleWord, seen map[string]bool) {
	if deny[tok] {
		return
	}
	if !labelNamePattern.MatchString(tok) {
		return
	}
	// Drop single-word non-allowlisted tokens (almost always false
	// positives — event values, field names).
	if !strings.Contains(tok, "-") && !singleWord[tok] {
		return
	}
	seen[tok] = true
}

// labelNamePattern matches the canonical label name shape:
// lowercase letter start, lowercase + digits + hyphen.
var labelNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]+$`)

// repoRoot walks up from the test's working directory until it finds
// a directory containing go.mod, and returns that absolute path.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate go.mod from test directory")
	return ""
}
