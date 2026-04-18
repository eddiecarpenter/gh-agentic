package projectstatus

import (
	"errors"
	"strings"
	"testing"
)

// TestFetchBlocker_NativeSourceHit verifies the native GraphQL path is
// preferred when it returns a blocker.
func TestFetchBlocker_NativeSourceHit(t *testing.T) {
	deps := Deps{
		FetchBlocker: func(owner, repo string, number int) (*BlockedInfo, error) {
			return &BlockedInfo{BlockingRef: "foo/bar#99", Reason: "native blocker"}, nil
		},
	}
	// Body also contains a convention marker — native must win.
	got, err := FetchBlocker(deps, "o", "r", 1, "Blocked-by: other/repo#5\n")
	if err != nil {
		t.Fatalf("FetchBlocker: %v", err)
	}
	if got == nil {
		t.Fatalf("expected blocker; got nil")
	}
	if got.BlockingRef != "foo/bar#99" {
		t.Errorf("expected native ref 'foo/bar#99'; got %q", got.BlockingRef)
	}
	if got.Reason != "native blocker" {
		t.Errorf("expected native reason; got %q", got.Reason)
	}
}

// TestFetchBlocker_ConventionFallbackWhenNativeAbsent verifies the body
// convention fires when the native source returns nil.
func TestFetchBlocker_ConventionFallbackWhenNativeAbsent(t *testing.T) {
	deps := Deps{
		FetchBlocker: func(owner, repo string, number int) (*BlockedInfo, error) {
			return nil, nil
		},
	}
	got, err := FetchBlocker(deps, "o", "r", 1, "Some body\nBlocked-by: o/r#42\n")
	if err != nil {
		t.Fatalf("FetchBlocker: %v", err)
	}
	if got == nil || got.BlockingRef != "o/r#42" {
		t.Errorf("expected convention ref 'o/r#42'; got %+v", got)
	}
}

// TestFetchBlocker_BareConventionDefaultsToOwningRepo verifies `Blocked-by: #N`
// is normalised to `owner/repo#N`.
func TestFetchBlocker_BareConventionDefaultsToOwningRepo(t *testing.T) {
	got, err := FetchBlocker(Deps{}, "eddiecarpenter", "gh-agentic", 1, "Blocked-by: #507")
	if err != nil {
		t.Fatalf("FetchBlocker: %v", err)
	}
	if got == nil || got.BlockingRef != "eddiecarpenter/gh-agentic#507" {
		t.Errorf("expected normalised ref; got %+v", got)
	}
}

// TestFetchBlocker_CrossRepoConventionPreservesOwner verifies a qualified
// `Blocked-by:` line is surfaced verbatim.
func TestFetchBlocker_CrossRepoConventionPreservesOwner(t *testing.T) {
	got, err := FetchBlocker(Deps{}, "eddiecarpenter", "gh-agentic", 1, "Blocked-by: foo/bar#99")
	if err != nil {
		t.Fatalf("FetchBlocker: %v", err)
	}
	if got == nil || got.BlockingRef != "foo/bar#99" {
		t.Errorf("expected cross-repo ref 'foo/bar#99'; got %+v", got)
	}
}

// TestFetchBlocker_BothAbsentReturnsNil verifies graceful degradation when
// neither source reports a blocker.
func TestFetchBlocker_BothAbsentReturnsNil(t *testing.T) {
	got, err := FetchBlocker(Deps{}, "o", "r", 1, "This body has no blocker marker at all.")
	if err != nil {
		t.Fatalf("FetchBlocker: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil blocker; got %+v", got)
	}
}

// TestFetchBlocker_MalformedConventionIgnored verifies a mid-prose mention
// of "blocked" does not trigger — only the anchored Blocked-by: form does.
func TestFetchBlocker_MalformedConventionIgnored(t *testing.T) {
	// Prose-like content that mentions "blocked" should never be parsed.
	cases := []string{
		"this issue was blocked by an earlier PR",           // casual prose
		"Note: we think it's blocked-by the network team",   // almost matching
		"Blocked-by",                                        // no value
		"Blocked-by:",                                       // blank value
		"Blocked-by: #",                                     // no number
		"Blocked-by: #abc",                                  // non-numeric number
	}
	for _, body := range cases {
		got, err := FetchBlocker(Deps{}, "o", "r", 1, body)
		if err != nil {
			t.Fatalf("FetchBlocker(%q): %v", body, err)
		}
		if got != nil {
			t.Errorf("prose body %q should not yield a blocker; got %+v", body, got)
		}
	}
}

// TestFetchBlocker_NativeErrorPropagates verifies the native lookup failure
// is surfaced — the CLI layer classifies it.
func TestFetchBlocker_NativeErrorPropagates(t *testing.T) {
	deps := Deps{
		FetchBlocker: func(owner, repo string, number int) (*BlockedInfo, error) {
			return nil, errors.New("boom")
		},
	}
	_, err := FetchBlocker(deps, "o", "r", 1, "Blocked-by: o/r#1")
	if err == nil {
		t.Errorf("expected error; got nil")
	}
}

// TestFetchBlocker_NoProseParsing enforces — via a direct grep over the
// convention pattern — that only the structured form matches.
func TestFetchBlocker_NoProseParsing(t *testing.T) {
	// Confirm the regex requires the anchor and exact prefix.
	pattern := blockedByConventionPattern.String()
	for _, must := range []string{"^", "Blocked-by:", "(?mi)"} {
		if !strings.Contains(pattern, must) {
			t.Errorf("convention pattern must contain %q; got %q", must, pattern)
		}
	}
}

// TestFetchRequirements_PopulatesBlockedViaConvention checks the end-to-end
// wiring from FetchRequirements into the body-convention parser. A
// requirement body carrying `Blocked-by: foo/bar#99` surfaces as a blocked
// annotation on the returned Requirement without requiring a native source.
func TestFetchRequirements_PopulatesBlockedViaConvention(t *testing.T) {
	issues := []ProjectIssue{
		{Number: 1, Title: "r1", Type: "requirement", State: "open", Stage: StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", Body: "Blocked-by: foo/bar#99"},
		{Number: 2, Title: "r2", Type: "requirement", State: "open", Stage: StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", Body: "no blocker here"},
	}
	deps := Deps{
		FetchProjectIssues: func(string) ([]ProjectIssue, error) { return issues, nil },
	}
	reqs, err := FetchRequirements(deps, "PROJ", false)
	if err != nil {
		t.Fatalf("FetchRequirements: %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requirements; got %d", len(reqs))
	}
	if reqs[0].Blocked == nil || reqs[0].Blocked.BlockingRef != "foo/bar#99" {
		t.Errorf("r1 should be blocked; got %+v", reqs[0].Blocked)
	}
	if reqs[1].Blocked != nil {
		t.Errorf("r2 should not be blocked; got %+v", reqs[1].Blocked)
	}
}

// TestFetchFeatures_PopulatesBlockedViaNative mirrors the requirements test
// for features + native source.
func TestFetchFeatures_PopulatesBlockedViaNative(t *testing.T) {
	issues := []ProjectIssue{
		{Number: 1, Title: "f1", Type: "feature", State: "open", Stage: StageInDevelopment, OwningRepo: "eddiecarpenter/gh-agentic"},
	}
	deps := Deps{
		FetchProjectIssues: func(string) ([]ProjectIssue, error) { return issues, nil },
		FetchBlocker: func(owner, repo string, number int) (*BlockedInfo, error) {
			if number == 1 {
				return &BlockedInfo{BlockingRef: "o/r#5", Reason: "upstream"}, nil
			}
			return nil, nil
		},
	}
	feats, err := FetchFeatures(deps, "PROJ", false)
	if err != nil {
		t.Fatalf("FetchFeatures: %v", err)
	}
	if feats[0].Blocked == nil || feats[0].Blocked.BlockingRef != "o/r#5" {
		t.Errorf("f1 should have native blocker; got %+v", feats[0].Blocked)
	}
}
