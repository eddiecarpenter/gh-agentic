package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// --------------------------------------------------------------------------
// Fixture — canned federated project with two repos, three requirements,
// four features, six tasks, and one blocked item. Used across the
// integration tests to prove the full stack works end-to-end against a
// realistic-shaped project state.
// --------------------------------------------------------------------------

// fixedTime is the deterministic timestamp used everywhere so --raw output
// is byte-identical across runs.
var fixedTime = time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)

// fixtureIssues returns the canned project board used by every integration
// test in this file. The shape deliberately covers: requirements and
// features across two owning repos, closed entries, a blocked feature, a
// requirement with a linked feature, and type labels on every issue.
func fixtureIssues() []projectstatus.ProjectIssue {
	return []projectstatus.ProjectIssue{
		// Requirements ------------------------------------------------------
		{Number: 447, Title: "feat: project lifecycle management", Body: "Business need.", Stage: projectstatus.StageBacklog, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 457, Title: "feat: single-pane pipeline status view", Body: "Closes #447", Stage: projectstatus.StageScoping, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 466, Title: "feat: ask-user reference", Body: "Done body.", Stage: projectstatus.StageDone, Type: "requirement", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		// Features ----------------------------------------------------------
		{Number: 483, Title: "feat: ask-user skill", Body: "Closes #466", Stage: projectstatus.StageDone, Type: "feature", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 492, Title: "feat: status command", Body: "## User Story\nCloses #457", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 511, Title: "feat: domain-X event handler", Body: "Blocked-by: foo/domain-x#507", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 520, Title: "feat: placeholder", Body: "", Stage: projectstatus.StageBacklog, Type: "feature", State: "open", OwningRepo: "foo/domain-x", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		// Tasks (sub-issues of #492 scoped feature) -------------------------
		{Number: 494, Title: "Scaffold", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 495, Title: "projectstatus types", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 496, Title: "Requirements list", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 497, Title: "Requirement detail", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 498, Title: "Features list", Type: "task", State: "closed", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
		{Number: 499, Title: "Feature detail", Type: "task", State: "open", Stage: projectstatus.StageBacklog, OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: fixedTime, LastTransitionedAt: fixedTime},
	}
}

// fixtureTasks returns the sub-issue map consumed by FetchFeature.
func fixtureTasks() map[int][]projectstatus.TaskRef {
	return map[int][]projectstatus.TaskRef{
		492: {
			{Number: 494, Title: "Scaffold", Closed: true},
			{Number: 495, Title: "projectstatus types", Closed: true},
			{Number: 496, Title: "Requirements list", Closed: true},
			{Number: 497, Title: "Requirement detail", Closed: true},
			{Number: 498, Title: "Features list", Closed: true},
			{Number: 499, Title: "Feature detail", Closed: false},
		},
	}
}

// fixtureBranches returns the branch map for FetchBranch.
func fixtureBranches() map[string]*projectstatus.BranchState {
	return map[string]*projectstatus.BranchState{
		"feature/483": {Name: "feature/483", Exists: true, Merged: true},
		"feature/492": {Name: "feature/492", Exists: true, Merged: false},
	}
}

// fixturePRs returns the PR map for FetchPR.
func fixturePRs() map[string]*projectstatus.PRState {
	return map[string]*projectstatus.PRState{
		"feature/483": {Number: 491, State: "merged", Reviewers: []string{"eddiecarpenter"}},
		"feature/492": {Number: 777, State: "open", Reviewers: []string{"eddie"}},
	}
}

// buildFixtureDeps assembles a statusDeps that serves the canned fixtures.
func buildFixtureDeps() statusDeps {
	return statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "PVT_fixture", nil },
		psDeps: projectstatus.Deps{
			FetchProjectIssues: func(string) ([]projectstatus.ProjectIssue, error) { return fixtureIssues(), nil },
			FetchLinkedRepos: func(string) ([]project.LinkedRepo, error) {
				return []project.LinkedRepo{
					{NameWithOwner: "eddiecarpenter/gh-agentic"},
					{NameWithOwner: "foo/domain-x"},
				}, nil
			},
			FetchSubIssues: func(_, _ string, n int) ([]projectstatus.TaskRef, error) {
				return fixtureTasks()[n], nil
			},
			FetchParentIssue: func(_, _ string, n int) (*projectstatus.RequirementSummary, error) {
				return nil, nil
			},
			FetchBranch: func(_, _, name string) (*projectstatus.BranchState, error) {
				if b, ok := fixtureBranches()[name]; ok {
					return b, nil
				}
				return &projectstatus.BranchState{Name: name, Exists: false}, nil
			},
			FetchPR: func(_, _, name string) (*projectstatus.PRState, error) {
				return fixturePRs()[name], nil
			},
		},
		busy: testutil.NoopBusy,
	}
}

// --------------------------------------------------------------------------
// Integration tests — run the full stack for every sub-command.
// The --json schema integration tests were removed by feature #589 along
// with the --json flag itself; --raw byte-equality is exercised by the
// per-shape tests in status_requirements_test.go, status_features_test.go,
// status_requirement_test.go, status_feature_test.go, and pipeline_cmd_test.go.
// --------------------------------------------------------------------------

// TestIntegration_RawStabilityAcrossInvocations verifies byte-identical
// --raw output across two runs of each sub-command — the hardest
// guarantee for downstream agent consumers.
func TestIntegration_RawStabilityAcrossInvocations(t *testing.T) {
	runs := []struct {
		name string
		run  func(w *bytes.Buffer) error
	}{
		{"requirements list", func(w *bytes.Buffer) error {
			return runStatusRequirements(w, io.Discard, statusListFlags{raw: true}, buildFixtureDeps())
		}},
		{"requirement detail", func(w *bytes.Buffer) error {
			return runStatusRequirement(w, io.Discard, 457, statusDetailFlags{raw: true}, buildFixtureDeps())
		}},
		{"features list", func(w *bytes.Buffer) error {
			return runStatusFeatures(w, io.Discard, statusListFlags{raw: true}, buildFixtureDeps())
		}},
		{"feature detail", func(w *bytes.Buffer) error {
			return runStatusFeature(w, io.Discard, 492, statusDetailFlags{raw: true}, buildFixtureDeps())
		}},
	}
	for _, r := range runs {
		t.Run(r.name, func(t *testing.T) {
			bufA := &bytes.Buffer{}
			bufB := &bytes.Buffer{}
			if err := r.run(bufA); err != nil {
				t.Fatalf("%s run A: %v", r.name, err)
			}
			if err := r.run(bufB); err != nil {
				t.Fatalf("%s run B: %v", r.name, err)
			}
			if bufA.String() != bufB.String() {
				t.Errorf("%s: --raw output is not byte-identical across runs", r.name)
			}
		})
	}
}

// TestIntegration_HumanOutputHasFederatedRepoColumn verifies the features
// list in a federated project surfaces the REPO column with the expected
// cells.
func TestIntegration_HumanOutputHasFederatedRepoColumn(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"REPO", "foo/domain-x", "(this repo)", "#492", "#511"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in federated human output; got:\n%s", want, out)
		}
	}
}

// TestIntegration_ThisRepoNarrowing verifies --this-repo filters to the
// current repo on both list commands.
func TestIntegration_ThisRepoNarrowing(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{thisRepo: true}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "#520") {
		t.Errorf("--this-repo should exclude foreign feature #520; got:\n%s", out)
	}
	if !strings.Contains(out, "#492") {
		t.Errorf("--this-repo should include local feature #492; got:\n%s", out)
	}
}

// TestIntegration_BlockedAnnotationEndToEnd verifies the blocked annotation
// appears in list views and the Blocked line appears in detail.
func TestIntegration_BlockedAnnotationEndToEnd(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := runStatusFeatures(buf, io.Discard, statusListFlags{}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	if !strings.Contains(buf.String(), "[blocked by foo/domain-x#507]") {
		t.Errorf("expected blocked annotation on #511; got:\n%s", buf.String())
	}

	detailBuf := &bytes.Buffer{}
	if err := runStatusFeature(detailBuf, io.Discard, 511, statusDetailFlags{}, buildFixtureDeps()); err != nil {
		t.Fatalf("runStatusFeature: %v", err)
	}
	if !strings.Contains(detailBuf.String(), "Blocked: foo/domain-x#507") {
		t.Errorf("expected Blocked: line in detail; got:\n%s", detailBuf.String())
	}
}

// TestIntegration_ErrorPaths exercises the classified error scenarios through
// the command handlers to confirm each produces the documented message.
func TestIntegration_ErrorPaths(t *testing.T) {
	t.Run("missing project id", func(t *testing.T) {
		sd := statusDeps{
			currentRepo:      func() (string, error) { return "owner/repo", nil },
			resolveProjectID: func(string) (string, error) { return "", nil },
			psDeps:           projectstatus.Deps{},
			busy:             testutil.NoopBusy,
		}
		err := runStatusRequirements(&bytes.Buffer{}, io.Discard, statusListFlags{}, sd)
		if !errors.Is(err, projectstatus.ErrProjectNotConfigured) {
			t.Errorf("expected ErrProjectNotConfigured; got %v", err)
		}
	})

	t.Run("unknown issue number", func(t *testing.T) {
		err := runStatusRequirement(&bytes.Buffer{}, io.Discard, 9999, statusDetailFlags{}, buildFixtureDeps())
		if !errors.Is(err, projectstatus.ErrIssueNotFound) {
			t.Errorf("expected ErrIssueNotFound; got %v", err)
		}
	})

	t.Run("wrong type requirement->feature", func(t *testing.T) {
		err := runStatusFeature(&bytes.Buffer{}, io.Discard, 457, statusDetailFlags{}, buildFixtureDeps())
		var wt *projectstatus.ErrWrongType
		if !errors.As(err, &wt) {
			t.Fatalf("expected ErrWrongType; got %v", err)
		}
		if wt.ActualType != "requirement" || wt.WantedType != "feature" {
			t.Errorf("unexpected wrong-type fields: %+v", wt)
		}
	})

	t.Run("wrong type feature->requirement", func(t *testing.T) {
		err := runStatusRequirement(&bytes.Buffer{}, io.Discard, 492, statusDetailFlags{}, buildFixtureDeps())
		var wt *projectstatus.ErrWrongType
		if !errors.As(err, &wt) {
			t.Fatalf("expected ErrWrongType; got %v", err)
		}
	})
}

// TestIntegration_BareStatusPrintsHelp verifies `status` with no sub-command
// prints help with all four leaf sub-commands listed.
func TestIntegration_BareStatusPrintsHelp(t *testing.T) {
	cmd := newStatusCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bare status: %v", err)
	}
	for _, tok := range []string{"requirements", "requirement", "features", "feature"} {
		if !strings.Contains(buf.String(), tok) {
			t.Errorf("help missing %q; got:\n%s", tok, buf.String())
		}
	}
}
