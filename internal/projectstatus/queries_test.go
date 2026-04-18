package projectstatus

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// jsonMarshal wraps encoding/json.Marshal in a locally-named shim to avoid
// colliding with test helpers that use `json.Unmarshal` inline.
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// fakeDeps builds a Deps populated with deterministic fakes for tests.
// Each callback is wired to return data derived from the provided issues
// slice unless overridden by the caller.
type fakeDeps struct {
	issues        []ProjectIssue
	subIssues     map[int][]TaskRef
	parentIssue   map[int]*RequirementSummary
	branches      map[string]*BranchState
	prs           map[string]*PRState
	linkedRepos   []project.LinkedRepo
	issuesErr     error
	issueFetchErr error
}

func (f fakeDeps) Deps() Deps {
	return Deps{
		FetchLinkedRepos: func(projectID string) ([]project.LinkedRepo, error) {
			return f.linkedRepos, nil
		},
		FetchProjectIssues: func(projectID string) ([]ProjectIssue, error) {
			if f.issuesErr != nil {
				return nil, f.issuesErr
			}
			return f.issues, nil
		},
		FetchIssue: func(owner, repo string, number int) (*ProjectIssue, error) {
			if f.issueFetchErr != nil {
				return nil, f.issueFetchErr
			}
			for i := range f.issues {
				if f.issues[i].Number == number {
					cp := f.issues[i]
					return &cp, nil
				}
			}
			return nil, ErrIssueNotFound
		},
		FetchSubIssues: func(owner, repo string, number int) ([]TaskRef, error) {
			return f.subIssues[number], nil
		},
		FetchParentIssue: func(owner, repo string, number int) (*RequirementSummary, error) {
			return f.parentIssue[number], nil
		},
		FetchBranch: func(owner, repo, name string) (*BranchState, error) {
			if br, ok := f.branches[name]; ok {
				return br, nil
			}
			return &BranchState{Name: name, Exists: false}, nil
		},
		FetchPR: func(owner, repo, name string) (*PRState, error) {
			return f.prs[name], nil
		},
	}
}

// sampleIssues returns a deterministic, unsorted fixture of project items.
func sampleIssues() []ProjectIssue {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	return []ProjectIssue{
		// Intentional unsorted order — FetchRequirements/FetchFeatures must sort.
		{Number: 492, Title: "feat: status", Body: "Closes #457", Stage: StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 457, Title: "requirement: status view", Body: "Body of req 457", Stage: StageScoping, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 466, Title: "requirement: ask-user", Body: "Body", Stage: StageDone, Type: "requirement", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 483, Title: "feat: ask-user", Body: "Closes #466", Stage: StageDone, Type: "feature", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		{Number: 447, Title: "requirement: project lifecycle", Body: "Body", Stage: StageBacklog, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		// A task — must be excluded from requirement/feature lists.
		{Number: 999, Title: "task: example", Stage: StageBacklog, Type: "task", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
		// An issue with no type label — must be excluded.
		{Number: 10, Title: "untyped", Stage: StageUnknown, Type: "", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
}

// TestFetchRequirements_ExcludesDoneByDefault verifies default list view
// omits closed / done requirements and sorts the result ascending by number.
func TestFetchRequirements_ExcludesDoneByDefault(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	got, err := FetchRequirements(f.Deps(), "PROJ", false)
	if err != nil {
		t.Fatalf("FetchRequirements: %v", err)
	}

	var numbers []int
	for _, r := range got {
		numbers = append(numbers, r.Number)
	}
	want := []int{447, 457}
	if !equalInts(numbers, want) {
		t.Errorf("FetchRequirements (default) numbers = %v, want %v", numbers, want)
	}
}

// TestFetchRequirements_IncludeDone verifies --include-done pulls closed /
// done items back in.
func TestFetchRequirements_IncludeDone(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	got, err := FetchRequirements(f.Deps(), "PROJ", true)
	if err != nil {
		t.Fatalf("FetchRequirements: %v", err)
	}
	var numbers []int
	for _, r := range got {
		numbers = append(numbers, r.Number)
	}
	want := []int{447, 457, 466}
	if !equalInts(numbers, want) {
		t.Errorf("FetchRequirements (include-done) numbers = %v, want %v", numbers, want)
	}
}

// TestFetchRequirements_ReturnsDeterministicOrder runs the fetch twice and
// verifies identical slices — guards against map-iteration or append-order
// regressions.
func TestFetchRequirements_ReturnsDeterministicOrder(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	first, err := FetchRequirements(f.Deps(), "PROJ", true)
	if err != nil {
		t.Fatalf("FetchRequirements: %v", err)
	}
	second, err := FetchRequirements(f.Deps(), "PROJ", true)
	if err != nil {
		t.Fatalf("FetchRequirements: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("len mismatch: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Number != second[i].Number {
			t.Errorf("order diverged at %d: %d vs %d", i, first[i].Number, second[i].Number)
		}
	}
}

// TestFetchFeatures_ExcludesDoneByDefault mirrors the requirements test.
func TestFetchFeatures_ExcludesDoneByDefault(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	got, err := FetchFeatures(f.Deps(), "PROJ", false)
	if err != nil {
		t.Fatalf("FetchFeatures: %v", err)
	}
	var numbers []int
	for _, ft := range got {
		numbers = append(numbers, ft.Number)
	}
	want := []int{492}
	if !equalInts(numbers, want) {
		t.Errorf("FetchFeatures (default) numbers = %v, want %v", numbers, want)
	}
}

// TestFetchFeatures_PopulatesTaskCounts verifies that FetchFeatures writes
// TasksTotal and TasksDone on every feature it returns — zero tasks, partial
// completion, and full completion all produce the expected counts.
func TestFetchFeatures_PopulatesTaskCounts(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []ProjectIssue{
		{Number: 100, Title: "zero-task feature", Type: "feature", State: "open", Stage: StageBacklog, OwningRepo: "o/r", CreatedAt: now, LastTransitionedAt: now},
		{Number: 200, Title: "partial feature", Type: "feature", State: "open", Stage: StageInDevelopment, OwningRepo: "o/r", CreatedAt: now, LastTransitionedAt: now},
		{Number: 300, Title: "complete feature", Type: "feature", State: "open", Stage: StageInReview, OwningRepo: "o/r", CreatedAt: now, LastTransitionedAt: now},
	}
	subs := map[int][]TaskRef{
		100: {},
		200: {
			{Number: 201, Title: "t1", Closed: true},
			{Number: 202, Title: "t2", Closed: false},
			{Number: 203, Title: "t3", Closed: true},
		},
		300: {
			{Number: 301, Title: "t1", Closed: true},
			{Number: 302, Title: "t2", Closed: true},
		},
	}
	f := fakeDeps{issues: issues, subIssues: subs}

	got, err := FetchFeatures(f.Deps(), "PROJ", false)
	if err != nil {
		t.Fatalf("FetchFeatures: %v", err)
	}
	byNumber := map[int]Feature{}
	for _, ft := range got {
		byNumber[ft.Number] = ft
	}

	cases := []struct {
		number      int
		wantTotal   int
		wantDone    int
		description string
	}{
		{number: 100, wantTotal: 0, wantDone: 0, description: "zero tasks"},
		{number: 200, wantTotal: 3, wantDone: 2, description: "partial completion"},
		{number: 300, wantTotal: 2, wantDone: 2, description: "full completion"},
	}
	for _, tc := range cases {
		ft, ok := byNumber[tc.number]
		if !ok {
			t.Errorf("%s: feature #%d missing from result", tc.description, tc.number)
			continue
		}
		if ft.TasksTotal != tc.wantTotal {
			t.Errorf("%s (#%d): TasksTotal = %d, want %d", tc.description, tc.number, ft.TasksTotal, tc.wantTotal)
		}
		if ft.TasksDone != tc.wantDone {
			t.Errorf("%s (#%d): TasksDone = %d, want %d", tc.description, tc.number, ft.TasksDone, tc.wantDone)
		}
	}
}

// TestFetchFeatures_TaskCountsZeroWhenDepNotWired verifies that a FetchFeatures
// call still succeeds when FetchSubIssues is not wired — the counts simply
// remain zero and the feature list is otherwise unaffected.
func TestFetchFeatures_TaskCountsZeroWhenDepNotWired(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []ProjectIssue{
		{Number: 100, Title: "f", Type: "feature", State: "open", Stage: StageBacklog, OwningRepo: "o/r", CreatedAt: now, LastTransitionedAt: now},
	}
	// A Deps without FetchSubIssues wired.
	deps := Deps{
		FetchProjectIssues: func(string) ([]ProjectIssue, error) { return issues, nil },
	}
	got, err := FetchFeatures(deps, "PROJ", false)
	if err != nil {
		t.Fatalf("FetchFeatures: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(got))
	}
	if got[0].TasksTotal != 0 || got[0].TasksDone != 0 {
		t.Errorf("expected zero counts when dep not wired; got Total=%d Done=%d", got[0].TasksTotal, got[0].TasksDone)
	}
}

// TestFetchFeatures_TaskCountsExcludedFromJSON verifies that the new internal
// fields are not serialised by json.Marshal — the locked schema for --json
// continues to emit exactly the documented keys.
func TestFetchFeatures_TaskCountsExcludedFromJSON(t *testing.T) {
	feat := Feature{
		Number:     1,
		Title:      "f",
		TasksTotal: 5,
		TasksDone:  3,
	}
	blob, err := jsonMarshalFeature(feat)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, disallowed := range []string{"tasks_total", "tasks_done", "TasksTotal", "TasksDone"} {
		if containsIdentifier(blob, disallowed) {
			t.Errorf("JSON payload must not contain %q; got:\n%s", disallowed, blob)
		}
	}
}

// TestFetchFeature_TaskCountsMirrorTasksSlice verifies the detail path also
// populates the internal counts from the already-fetched Tasks slice.
func TestFetchFeature_TaskCountsMirrorTasksSlice(t *testing.T) {
	f := fakeDeps{
		issues: sampleIssues(),
		subIssues: map[int][]TaskRef{
			492: {
				{Number: 494, Title: "a", Closed: true},
				{Number: 495, Title: "b", Closed: false},
				{Number: 496, Title: "c", Closed: true},
			},
		},
	}
	feat, err := FetchFeature(f.Deps(), "PROJ", 492)
	if err != nil {
		t.Fatalf("FetchFeature: %v", err)
	}
	if feat.TasksTotal != 3 {
		t.Errorf("TasksTotal = %d, want 3", feat.TasksTotal)
	}
	if feat.TasksDone != 2 {
		t.Errorf("TasksDone = %d, want 2", feat.TasksDone)
	}
}

// TestFetchFeatures_IncludeDone returns every feature ordered ascending.
func TestFetchFeatures_IncludeDone(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	got, err := FetchFeatures(f.Deps(), "PROJ", true)
	if err != nil {
		t.Fatalf("FetchFeatures: %v", err)
	}
	var numbers []int
	for _, ft := range got {
		numbers = append(numbers, ft.Number)
	}
	want := []int{483, 492}
	if !equalInts(numbers, want) {
		t.Errorf("FetchFeatures (include-done) numbers = %v, want %v", numbers, want)
	}
}

// TestFetchRequirement_NotFound verifies missing issues return ErrIssueNotFound.
func TestFetchRequirement_NotFound(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	_, err := FetchRequirement(f.Deps(), "PROJ", 9999)
	if !errors.Is(err, ErrIssueNotFound) {
		t.Errorf("FetchRequirement(missing) = %v, want ErrIssueNotFound", err)
	}
}

// TestFetchRequirement_WrongType verifies feature numbers passed to requirement
// detail return *ErrWrongType with the correct fields.
func TestFetchRequirement_WrongType(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	_, err := FetchRequirement(f.Deps(), "PROJ", 492)

	var wt *ErrWrongType
	if !errors.As(err, &wt) {
		t.Fatalf("FetchRequirement(feature) err = %v, want *ErrWrongType", err)
	}
	if wt.Number != 492 {
		t.Errorf("Number = %d, want 492", wt.Number)
	}
	if wt.ActualType != "feature" {
		t.Errorf("ActualType = %q, want %q", wt.ActualType, "feature")
	}
	if wt.WantedType != "requirement" {
		t.Errorf("WantedType = %q, want %q", wt.WantedType, "requirement")
	}
}

// TestFetchRequirement_LinkedFeaturesViaClosesMarker verifies that linked
// features are discovered by scanning feature bodies for `Closes #N` and
// that branch / PR state is embedded.
func TestFetchRequirement_LinkedFeaturesViaClosesMarker(t *testing.T) {
	f := fakeDeps{
		issues: sampleIssues(),
		branches: map[string]*BranchState{
			"feature/492": {Name: "feature/492", Exists: true, Merged: false},
		},
		prs: map[string]*PRState{
			"feature/492": {Number: 555, State: "open"},
		},
	}
	req, err := FetchRequirement(f.Deps(), "PROJ", 457)
	if err != nil {
		t.Fatalf("FetchRequirement: %v", err)
	}
	if len(req.LinkedFeatures) != 1 {
		t.Fatalf("expected 1 linked feature, got %d", len(req.LinkedFeatures))
	}
	lf := req.LinkedFeatures[0]
	if lf.Number != 492 {
		t.Errorf("Number = %d, want 492", lf.Number)
	}
	if lf.BranchOneLiner != "feature/492" {
		t.Errorf("BranchOneLiner = %q, want %q", lf.BranchOneLiner, "feature/492")
	}
	if lf.PR == nil || lf.PR.Number != 555 {
		t.Errorf("PR = %+v, want 555", lf.PR)
	}
}

// TestFetchFeature_NotFound verifies missing issues return ErrIssueNotFound.
func TestFetchFeature_NotFound(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	_, err := FetchFeature(f.Deps(), "PROJ", 9999)
	if !errors.Is(err, ErrIssueNotFound) {
		t.Errorf("FetchFeature(missing) = %v, want ErrIssueNotFound", err)
	}
}

// TestFetchFeature_WrongType verifies requirement numbers passed to feature
// detail return *ErrWrongType.
func TestFetchFeature_WrongType(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	_, err := FetchFeature(f.Deps(), "PROJ", 457)

	var wt *ErrWrongType
	if !errors.As(err, &wt) {
		t.Fatalf("FetchFeature(requirement) err = %v, want *ErrWrongType", err)
	}
	if wt.ActualType != "requirement" {
		t.Errorf("ActualType = %q, want %q", wt.ActualType, "requirement")
	}
	if wt.WantedType != "feature" {
		t.Errorf("WantedType = %q, want %q", wt.WantedType, "feature")
	}
}

// TestFetchFeature_PopulatesTasksBranchPR verifies the assembled Feature has
// tasks, branch state, and PR state populated from Deps.
func TestFetchFeature_PopulatesTasksBranchPR(t *testing.T) {
	f := fakeDeps{
		issues: sampleIssues(),
		subIssues: map[int][]TaskRef{
			492: {
				{Number: 494, Title: "Scaffold", Closed: true},
				{Number: 495, Title: "Types", Closed: false},
			},
		},
		branches: map[string]*BranchState{
			"feature/492": {Name: "feature/492", Exists: true, Merged: false},
		},
		prs: map[string]*PRState{
			"feature/492": {Number: 777, State: "open", Reviewers: []string{"eddie"}},
		},
	}
	feat, err := FetchFeature(f.Deps(), "PROJ", 492)
	if err != nil {
		t.Fatalf("FetchFeature: %v", err)
	}
	if len(feat.Tasks) != 2 {
		t.Errorf("Tasks len = %d, want 2", len(feat.Tasks))
	}
	if feat.Branch == nil || !feat.Branch.Exists {
		t.Errorf("Branch = %+v, want exists", feat.Branch)
	}
	if feat.PR == nil || feat.PR.Number != 777 {
		t.Errorf("PR = %+v, want #777", feat.PR)
	}
}

// TestFetchParentRequirement_PrefersNativeLink verifies the native
// trackedInIssues parent is used when present — the Closes fallback is not
// invoked.
func TestFetchParentRequirement_PrefersNativeLink(t *testing.T) {
	f := fakeDeps{
		issues: sampleIssues(),
		parentIssue: map[int]*RequirementSummary{
			492: {Number: 457, Title: "requirement: status view", OwningRepo: "eddiecarpenter/gh-agentic"},
		},
	}
	parent, err := FetchParentRequirement(f.Deps(), "eddiecarpenter", "gh-agentic", 492, "Closes #999")
	if err != nil {
		t.Fatalf("FetchParentRequirement: %v", err)
	}
	if parent == nil {
		t.Fatalf("parent == nil; expected native link to win")
	}
	// Native parent's Number is 457 regardless of the body's Closes marker.
	if parent.Number != 457 {
		t.Errorf("parent.Number = %d, want 457 (native)", parent.Number)
	}
}

// TestFetchParentRequirement_FallsBackToClosesMarker verifies that when no
// native parent is available, the feature body is parsed for `Closes #N`.
func TestFetchParentRequirement_FallsBackToClosesMarker(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	parent, err := FetchParentRequirement(f.Deps(), "eddiecarpenter", "gh-agentic", 492, "Parent Requirement\n\nCloses #457")
	if err != nil {
		t.Fatalf("FetchParentRequirement: %v", err)
	}
	if parent == nil {
		t.Fatalf("parent == nil; expected Closes fallback to match")
	}
	if parent.Number != 457 {
		t.Errorf("parent.Number = %d, want 457", parent.Number)
	}
}

// TestFetchParentRequirement_NeitherPresent verifies graceful degradation to
// (nil, nil) when neither the native link nor a Closes marker is present.
func TestFetchParentRequirement_NeitherPresent(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	parent, err := FetchParentRequirement(f.Deps(), "eddiecarpenter", "gh-agentic", 492, "body with no closes marker")
	if err != nil {
		t.Fatalf("FetchParentRequirement: %v", err)
	}
	if parent != nil {
		t.Errorf("parent = %+v, want nil", parent)
	}
}

// TestFetchParentRequirement_ClosesMarkerPointsToFeature verifies that when
// the Closes marker points at a non-requirement issue, the fallback treats it
// as absent (returns nil) rather than returning a Feature pretending to be a
// RequirementSummary.
func TestFetchParentRequirement_ClosesMarkerPointsToFeature(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	parent, err := FetchParentRequirement(f.Deps(), "eddiecarpenter", "gh-agentic", 492, "Closes #483")
	if err != nil {
		t.Fatalf("FetchParentRequirement: %v", err)
	}
	if parent != nil {
		t.Errorf("parent = %+v, want nil when Closes points at a feature", parent)
	}
}

// TestFetchParentRequirement_QualifiedClosesMarker covers the
// `Closes owner/repo#N` form.
func TestFetchParentRequirement_QualifiedClosesMarker(t *testing.T) {
	f := fakeDeps{issues: sampleIssues()}
	parent, err := FetchParentRequirement(f.Deps(), "other-owner", "other-repo", 492, "Closes eddiecarpenter/gh-agentic#457")
	if err != nil {
		t.Fatalf("FetchParentRequirement: %v", err)
	}
	if parent == nil || parent.Number != 457 {
		t.Errorf("parent = %+v, want #457", parent)
	}
}

// TestFetchTasksForFeature_DelegatesToDeps checks the pass-through is wired.
func TestFetchTasksForFeature_DelegatesToDeps(t *testing.T) {
	f := fakeDeps{
		subIssues: map[int][]TaskRef{
			492: {{Number: 494, Title: "t", Closed: true}},
		},
	}
	tasks, err := FetchTasksForFeature(f.Deps(), "o", "r", 492)
	if err != nil {
		t.Fatalf("FetchTasksForFeature: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Number != 494 {
		t.Errorf("tasks = %+v, want one with number 494", tasks)
	}
}

// TestFetchTasksForFeature_NotWired returns an error when the dependency is
// absent — prevents silent zero-results.
func TestFetchTasksForFeature_NotWired(t *testing.T) {
	_, err := FetchTasksForFeature(Deps{}, "o", "r", 1)
	if err == nil {
		t.Errorf("expected error when FetchSubIssues is nil, got nil")
	}
}

// TestResolveFederatedRepos_ReturnsSortedNameWithOwner verifies the linked
// repos are surfaced as owner/name strings in sorted order.
func TestResolveFederatedRepos_ReturnsSortedNameWithOwner(t *testing.T) {
	f := fakeDeps{
		linkedRepos: []project.LinkedRepo{
			{NameWithOwner: "z-owner/z-repo"},
			{NameWithOwner: "a-owner/a-repo"},
		},
	}
	repos, err := ResolveFederatedRepos(f.Deps(), "PROJ")
	if err != nil {
		t.Fatalf("ResolveFederatedRepos: %v", err)
	}
	want := []string{"a-owner/a-repo", "z-owner/z-repo"}
	if !equalStrings(repos, want) {
		t.Errorf("ResolveFederatedRepos = %v, want %v", repos, want)
	}
}

// TestFetchProjectIssuesError_PropagatesFromFetchRequirements verifies that an
// error from the underlying fetch is surfaced, not swallowed.
func TestFetchProjectIssuesError_PropagatesFromFetchRequirements(t *testing.T) {
	f := fakeDeps{issuesErr: fmt.Errorf("boom")}
	_, err := FetchRequirements(f.Deps(), "PROJ", false)
	if err == nil {
		t.Fatal("expected error from FetchRequirements, got nil")
	}
}

// TestBodyReferencesRequirement_CrossRepoGuard verifies a bare `Closes #N`
// only matches within the same owning repo — protects against cross-repo
// number collisions.
func TestBodyReferencesRequirement_CrossRepoGuard(t *testing.T) {
	if bodyReferencesRequirement("Closes #457", 457, "a/b", "c/d") {
		t.Errorf("bare Closes #457 should not cross from a/b to c/d")
	}
	if !bodyReferencesRequirement("Closes #457", 457, "a/b", "a/b") {
		t.Errorf("bare Closes #457 in same repo should match")
	}
	if !bodyReferencesRequirement("Closes a/b#457", 457, "c/d", "a/b") {
		t.Errorf("qualified Closes a/b#457 should match regardless of feature repo")
	}
}

// jsonMarshalFeature marshals a Feature using encoding/json — used by the
// schema-stability test to confirm internal fields are not emitted.
func jsonMarshalFeature(f Feature) (string, error) {
	b, err := jsonMarshal(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// containsIdentifier reports whether s contains target as a substring. Named
// for readability in the schema-stability test.
func containsIdentifier(s, target string) bool {
	return len(target) > 0 && indexOf(s, target) >= 0
}

// indexOf wraps strings.Index via a local shim so the test file does not
// need to import strings inline for a single call.
func indexOf(s, sep string) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

// equalInts is a small helper for slice comparison in tests.
func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// equalStrings is a small helper for slice comparison in tests.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
