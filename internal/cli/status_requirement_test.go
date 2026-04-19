package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// requirementDetailFixture builds a fake projectstatus.Deps that returns the
// given issues slice for any project ID, so FetchRequirement walks the same
// composer path that production does.
func requirementDetailFixture(issues []projectstatus.ProjectIssue, branches map[string]*projectstatus.BranchState, prs map[string]*projectstatus.PRState) statusDeps {
	ps := projectstatus.Deps{
		FetchProjectIssues: func(projectID string) ([]projectstatus.ProjectIssue, error) {
			return issues, nil
		},
		FetchBranch: func(owner, repo, name string) (*projectstatus.BranchState, error) {
			if b, ok := branches[name]; ok {
				return b, nil
			}
			return &projectstatus.BranchState{Name: name, Exists: false}, nil
		},
		FetchPR: func(owner, repo, name string) (*projectstatus.PRState, error) {
			return prs[name], nil
		},
	}
	return statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "PROJ_ID", nil },
		psDeps:           ps,
		busy:             testutil.NoopBusy,
	}
}

// TestRunStatusRequirement_DefaultDetailOutput verifies the human layout
// contains the title, stage line, body, separator, and linked features.
func TestRunStatusRequirement_DefaultDetailOutput(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 466, Title: "requirement-title", Body: "## Business need\n\nBody content", Stage: projectstatus.StageDone, Type: "requirement", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 483, Title: "feat: ask-user", Body: "Closes #466", Stage: projectstatus.StageDone, Type: "feature", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	branches := map[string]*projectstatus.BranchState{
		"feature/483": {Name: "feature/483", Exists: true, Merged: true},
	}
	prs := map[string]*projectstatus.PRState{
		"feature/483": {Number: 491, State: "merged", Reviewers: []string{"eddie"}},
	}
	sd := requirementDetailFixture(issues, branches, prs)

	buf := &bytes.Buffer{}
	err := runStatusRequirement(buf, io.Discard, 466, statusDetailFlags{}, sd)
	if err != nil {
		t.Fatalf("runStatusRequirement: %v", err)
	}
	out := buf.String()
	for _, token := range []string{
		"requirement-title",
		"Stage: done",
		"Created: 2026-04-18",
		"Last transition: 2026-04-18",
		"Business need",
		"---",
		"Linked features:",
		"#483",
		"branch: feature/483 (merged)",
		"pr: #491 (merged) — reviewers: eddie",
	} {
		if !strings.Contains(out, token) {
			t.Errorf("expected output to contain %q; got:\n%s", token, out)
		}
	}
}

// TestRunStatusRequirement_NoLinkedFeaturesShowsNone verifies graceful
// rendering when the requirement has no linked features.
func TestRunStatusRequirement_NoLinkedFeaturesShowsNone(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 467, Title: "lonely", Stage: projectstatus.StageBacklog, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := requirementDetailFixture(issues, nil, nil)

	buf := &bytes.Buffer{}
	err := runStatusRequirement(buf, io.Discard, 467, statusDetailFlags{}, sd)
	if err != nil {
		t.Fatalf("runStatusRequirement: %v", err)
	}
	if !strings.Contains(buf.String(), "(none)") {
		t.Errorf("expected '(none)' for zero linked features; got:\n%s", buf.String())
	}
}

// TestRunStatusRequirement_JSONObjectShape verifies --json emits a single
// self-contained object with the locked field names.
func TestRunStatusRequirement_JSONObjectShape(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 466, Title: "requirement-title", Body: "body", Stage: projectstatus.StageDone, Type: "requirement", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := requirementDetailFixture(issues, nil, nil)

	buf := &bytes.Buffer{}
	err := runStatusRequirement(buf, io.Discard, 466, statusDetailFlags{json: true}, sd)
	if err != nil {
		t.Fatalf("runStatusRequirement: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("json decode: %v; raw:\n%s", err, buf.String())
	}
	for _, key := range []string{"number", "title", "body", "stage", "created_at", "last_transitioned_at", "owning_repo", "blocked", "linked_features"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON missing key %q; keys = %v", key, keysOf(parsed))
		}
	}
	// Blocked must be null (absent), not missing.
	if parsed["blocked"] != nil {
		t.Errorf("blocked = %v, want null", parsed["blocked"])
	}
	// linked_features must be [] not null — consumers can iterate uniformly.
	lf, ok := parsed["linked_features"].([]interface{})
	if !ok || lf == nil {
		t.Errorf("linked_features missing or wrong type: %v", parsed["linked_features"])
	}
}

// TestRunStatusRequirement_NotFound verifies a non-existent number surfaces
// a clear error and wraps ErrIssueNotFound.
func TestRunStatusRequirement_NotFound(t *testing.T) {
	sd := requirementDetailFixture(nil, nil, nil)
	err := runStatusRequirement(&bytes.Buffer{}, io.Discard, 9999, statusDetailFlags{}, sd)
	if err == nil {
		t.Fatalf("expected error for missing requirement, got nil")
	}
	if !errors.Is(err, projectstatus.ErrIssueNotFound) {
		t.Errorf("expected errors.Is(err, ErrIssueNotFound); got %v", err)
	}
	if !strings.Contains(err.Error(), "#9999") {
		t.Errorf("error should name the missing number; got %v", err)
	}
}

// TestRunStatusRequirement_WrongType verifies a feature number passed to the
// requirement detail command returns *ErrWrongType with the correct fields.
func TestRunStatusRequirement_WrongType(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 492, Title: "feat: status", Type: "feature", Stage: projectstatus.StageInDevelopment, State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := requirementDetailFixture(issues, nil, nil)
	err := runStatusRequirement(&bytes.Buffer{}, io.Discard, 492, statusDetailFlags{}, sd)

	var wt *projectstatus.ErrWrongType
	if !errors.As(err, &wt) {
		t.Fatalf("expected *projectstatus.ErrWrongType; got %v", err)
	}
	if wt.ActualType != "feature" || wt.WantedType != "requirement" {
		t.Errorf("wrong-type fields: %+v", wt)
	}
}

// TestRunStatusRequirement_BlockedLineRenders verifies the Blocked annotation
// appears in human output when Blocked is non-nil.
func TestRunStatusRequirement_BlockedLineRenders(t *testing.T) {
	r := &projectstatus.Requirement{
		Number:  10,
		Title:   "t",
		Stage:   projectstatus.StageBacklog,
		Blocked: &projectstatus.BlockedInfo{BlockingRef: "foo/bar#99", Reason: "awaiting schema"},
	}
	buf := &bytes.Buffer{}
	if err := writeRequirementDetail(buf, r); err != nil {
		t.Fatalf("writeRequirementDetail: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Blocked: awaiting schema (foo/bar#99)") {
		t.Errorf("expected blocked line; got:\n%s", out)
	}
}

// TestRunStatusRequirement_UnblockedOmitsLine verifies no Blocked line when
// Blocked is nil.
func TestRunStatusRequirement_UnblockedOmitsLine(t *testing.T) {
	r := &projectstatus.Requirement{Number: 10, Title: "t"}
	buf := &bytes.Buffer{}
	if err := writeRequirementDetail(buf, r); err != nil {
		t.Fatalf("writeRequirementDetail: %v", err)
	}
	if strings.Contains(buf.String(), "Blocked:") {
		t.Errorf("did not expect Blocked line; got:\n%s", buf.String())
	}
}

// TestParseIssueNumberArg_TolerantOfHash verifies the CLI accepts `#N` and
// plain `N` equally, and rejects non-numeric input.
func TestParseIssueNumberArg_TolerantOfHash(t *testing.T) {
	cases := []struct {
		in   string
		out  int
		ok   bool
	}{
		{in: "42", out: 42, ok: true},
		{in: "#42", out: 42, ok: true},
		{in: "  42 ", out: 42, ok: true},
		{in: "0", out: 0, ok: false},
		{in: "abc", out: 0, ok: false},
		{in: "", out: 0, ok: false},
		{in: "-5", out: 0, ok: false},
	}
	for _, tc := range cases {
		n, err := parseIssueNumberArg(tc.in)
		if tc.ok {
			if err != nil {
				t.Errorf("parseIssueNumberArg(%q) err = %v, want ok", tc.in, err)
			}
			if n != tc.out {
				t.Errorf("parseIssueNumberArg(%q) = %d, want %d", tc.in, n, tc.out)
			}
		} else {
			if err == nil {
				t.Errorf("parseIssueNumberArg(%q) = %d, want error", tc.in, n)
			}
		}
	}
}

// TestFormatPROneLiner covers the reviewers-present and reviewers-absent
// variants used in the linked-features block.
func TestFormatPROneLiner(t *testing.T) {
	got := formatPROneLiner(&projectstatus.PRState{Number: 42, State: "open"})
	if got != "#42 (open)" {
		t.Errorf("formatPROneLiner (no reviewers) = %q, want %q", got, "#42 (open)")
	}
	got = formatPROneLiner(&projectstatus.PRState{Number: 42, State: "merged", Reviewers: []string{"a", "b"}})
	want := "#42 (merged) — reviewers: a, b"
	if got != want {
		t.Errorf("formatPROneLiner (with reviewers) = %q, want %q", got, want)
	}
}

// TestRunStatusRequirement_RawVerbatimBody verifies the `--raw` renderer
// emits the frontmatter header, the literal `---` separator, and the body
// verbatim — markdown headings, fenced code, and a `→` character all
// survive without escaping. The rendered bytes must match the golden
// (which itself contains the markdown / code / arrow).
func TestRunStatusRequirement_RawVerbatimBody(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	body := "## Business need\n\n" +
		"Today, `gh agentic init` reads the project context from many places. We need\n" +
		"a single chokepoint.\n\n" +
		"```go\nfunc Resolve(ctx context.Context) (*Project, error) {\n    return nil, nil\n}\n```\n\n" +
		"Steps: scope → design → develop → review."
	issues := []projectstatus.ProjectIssue{
		{Number: 569, Title: "Centralised project context resolution", Body: body, Stage: projectstatus.StageScheduled, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		// Two linked features so the linked_features header field is non-empty.
		{Number: 571, Title: "feat: a", Stage: projectstatus.StageBacklog, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", Body: "Closes #569", CreatedAt: now, LastTransitionedAt: now},
		{Number: 572, Title: "feat: b", Stage: projectstatus.StageBacklog, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", Body: "Closes #569", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := requirementDetailFixture(issues, nil, nil)

	buf := &bytes.Buffer{}
	if err := runStatusRequirement(buf, io.Discard, 569, statusDetailFlags{raw: true}, sd); err != nil {
		t.Fatalf("runStatusRequirement --raw: %v", err)
	}
	got := buf.Bytes()

	wantBytes, err := os.ReadFile("testdata/status_raw/requirement_detail.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("--raw output does not match golden\nwant:\n%s\ngot:\n%s", string(wantBytes), string(got))
	}

	// AC-2: body region must contain markdown, fenced code, and the `→`
	// character — and there must be no JSON-style escape sequences (\\n,
	// \\`, etc.) anywhere in the body region.
	parts := strings.SplitN(string(got), "\n---\n", 2)
	if len(parts) != 2 {
		t.Fatalf("expected '---' separator on its own line; got:\n%s", string(got))
	}
	bodyOut := parts[1]
	for _, marker := range []string{"## Business need", "```go", "→"} {
		if !strings.Contains(bodyOut, marker) {
			t.Errorf("body should contain %q verbatim; got:\n%s", marker, bodyOut)
		}
	}
	for _, escape := range []string{`\n`, "\\`", `\u2192`} {
		if strings.Contains(bodyOut, escape) {
			t.Errorf("body should not contain escape sequence %q; got:\n%s", escape, bodyOut)
		}
	}
}

// TestRunStatusRequirement_RawSeparatorAlwaysPresent verifies the `---`
// separator is emitted even when the issue body is empty — agents can
// rely on the header / body split being unconditional.
func TestRunStatusRequirement_RawSeparatorAlwaysPresent(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 600, Title: "empty body", Body: "", Stage: projectstatus.StageBacklog, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := requirementDetailFixture(issues, nil, nil)
	buf := &bytes.Buffer{}
	if err := runStatusRequirement(buf, io.Discard, 600, statusDetailFlags{raw: true}, sd); err != nil {
		t.Fatalf("runStatusRequirement --raw: %v", err)
	}
	if !strings.Contains(buf.String(), "\n---\n") {
		t.Errorf("expected '---' separator even for empty body; got:\n%s", buf.String())
	}
}

// TestRunStatusRequirement_RawVerboseInsertsTimestamps verifies that
// `--raw --verbose` inserts `created_at` and `last_transitioned_at`
// header lines after `owning_repo` while keeping the `---` separator and
// the verbatim body intact.
func TestRunStatusRequirement_RawVerboseInsertsTimestamps(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	body := "## Business need\n\n" +
		"Today, `gh agentic init` reads the project context from many places. We need\n" +
		"a single chokepoint.\n\n" +
		"```go\nfunc Resolve(ctx context.Context) (*Project, error) {\n    return nil, nil\n}\n```\n\n" +
		"Steps: scope → design → develop → review."
	issues := []projectstatus.ProjectIssue{
		{Number: 569, Title: "Centralised project context resolution", Body: body, Stage: projectstatus.StageScheduled, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 571, Title: "feat: a", Stage: projectstatus.StageBacklog, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", Body: "Closes #569", CreatedAt: now, LastTransitionedAt: now},
		{Number: 572, Title: "feat: b", Stage: projectstatus.StageBacklog, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", Body: "Closes #569", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := requirementDetailFixture(issues, nil, nil)

	buf := &bytes.Buffer{}
	if err := runStatusRequirement(buf, io.Discard, 569, statusDetailFlags{raw: true, verbose: true}, sd); err != nil {
		t.Fatalf("runStatusRequirement --raw --verbose: %v", err)
	}
	got := buf.Bytes()

	wantBytes, err := os.ReadFile("testdata/status_raw/requirement_detail_verbose.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("--raw --verbose detail output does not match golden\nwant:\n%s\ngot:\n%s", string(wantBytes), string(got))
	}

	// Header must contain both timestamp keys, both as ISO date.
	for _, marker := range []string{"created_at: 2026-04-18", "last_transitioned_at: 2026-04-18"} {
		if !strings.Contains(string(got), marker) {
			t.Errorf("expected %q in verbose detail output; got:\n%s", marker, string(got))
		}
	}
}

// TestRunStatusRequirement_VerboseWithoutRawIsNoOp verifies the human
// detail view is unchanged when `--verbose` is passed without `--raw`.
func TestRunStatusRequirement_VerboseWithoutRawIsNoOp(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 569, Title: "t", Stage: projectstatus.StageScheduled, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := requirementDetailFixture(issues, nil, nil)
	bare := &bytes.Buffer{}
	if err := runStatusRequirement(bare, io.Discard, 569, statusDetailFlags{}, sd); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	verbose := &bytes.Buffer{}
	if err := runStatusRequirement(verbose, io.Discard, 569, statusDetailFlags{verbose: true}, sd); err != nil {
		t.Fatalf("verbose: %v", err)
	}
	if !bytes.Equal(bare.Bytes(), verbose.Bytes()) {
		t.Errorf("--verbose without --raw must not change detail output")
	}
}

// TestBlockedDetailLine covers the reason / ref / both branches.
func TestBlockedDetailLine(t *testing.T) {
	cases := []struct {
		in  *projectstatus.BlockedInfo
		out string
	}{
		{in: nil, out: ""},
		{in: &projectstatus.BlockedInfo{}, out: "Blocked"},
		{in: &projectstatus.BlockedInfo{BlockingRef: "a/b#1"}, out: "Blocked: a/b#1"},
		{in: &projectstatus.BlockedInfo{Reason: "r"}, out: "Blocked: r"},
		{in: &projectstatus.BlockedInfo{Reason: "r", BlockingRef: "a/b#1"}, out: "Blocked: r (a/b#1)"},
	}
	for _, tc := range cases {
		got := blockedDetailLine(tc.in)
		if got != tc.out {
			t.Errorf("blockedDetailLine(%+v) = %q, want %q", tc.in, got, tc.out)
		}
	}
}
