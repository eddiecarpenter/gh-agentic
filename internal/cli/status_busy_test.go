package cli

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
)

// recordingBusy returns a ui.BusyFunc that captures the labels each call
// was invoked with. Tests assert on the recorded labels without needing a
// TTY fake.
type recordingBusy struct {
	mu     sync.Mutex
	labels []string
}

func (r *recordingBusy) Run(_ io.Writer, label string, fn func() error) error {
	r.mu.Lock()
	r.labels = append(r.labels, label)
	r.mu.Unlock()
	return fn()
}

func (r *recordingBusy) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.labels))
	copy(out, r.labels)
	return out
}

// TestRunStatusRequirements_InvokesBusyWrapper verifies runStatusRequirements
// threads its fetch through deps.busy with the documented label.
func TestRunStatusRequirements_InvokesBusyWrapper(t *testing.T) {
	rec := &recordingBusy{}
	sd := fakeStatusDeps(sampleRequirementIssues())
	sd.busy = rec.Run

	buf := &bytes.Buffer{}
	if err := runStatusRequirements(buf, io.Discard, statusListFlags{}, sd); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	labels := rec.snapshot()
	if len(labels) != 1 || !strings.Contains(labels[0], "Fetching requirements") {
		t.Errorf("expected one busy invocation with 'Fetching requirements' label; got %v", labels)
	}
}

// TestRunStatusRequirement_InvokesBusyWrapper verifies the detail handler
// builds a per-issue label naming the requested number.
func TestRunStatusRequirement_InvokesBusyWrapper(t *testing.T) {
	rec := &recordingBusy{}
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 466, Title: "r", Stage: projectstatus.StageDone, Type: "requirement", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := requirementDetailFixture(issues, nil, nil)
	sd.busy = rec.Run

	if err := runStatusRequirement(&bytes.Buffer{}, io.Discard, 466, statusDetailFlags{}, sd); err != nil {
		t.Fatalf("runStatusRequirement: %v", err)
	}
	labels := rec.snapshot()
	if len(labels) != 1 || !strings.Contains(labels[0], "Fetching requirement #466") {
		t.Errorf("expected one busy invocation with 'Fetching requirement #466' label; got %v", labels)
	}
}

// TestRunStatusFeatures_InvokesBusyWrapper mirrors the requirements check.
func TestRunStatusFeatures_InvokesBusyWrapper(t *testing.T) {
	rec := &recordingBusy{}
	sd := fakeFeaturesDeps(sampleFeatureIssues(), nil)
	sd.busy = rec.Run

	if err := runStatusFeatures(&bytes.Buffer{}, io.Discard, statusListFlags{}, sd); err != nil {
		t.Fatalf("runStatusFeatures: %v", err)
	}
	labels := rec.snapshot()
	if len(labels) != 1 || !strings.Contains(labels[0], "Fetching features") {
		t.Errorf("expected one busy invocation with 'Fetching features' label; got %v", labels)
	}
}

// TestRunStatusFeature_InvokesBusyWrapper verifies the feature detail
// handler names the specific issue number in its busy label.
func TestRunStatusFeature_InvokesBusyWrapper(t *testing.T) {
	rec := &recordingBusy{}
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	issues := []projectstatus.ProjectIssue{
		{Number: 492, Title: "feat: status", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic", CreatedAt: now, LastTransitionedAt: now},
	}
	sd := featureDetailFixture(issues, nil, nil, nil, nil)
	sd.busy = rec.Run

	if err := runStatusFeature(&bytes.Buffer{}, io.Discard, 492, statusDetailFlags{}, sd); err != nil {
		t.Fatalf("runStatusFeature: %v", err)
	}
	labels := rec.snapshot()
	if len(labels) != 1 || !strings.Contains(labels[0], "Fetching feature #492") {
		t.Errorf("expected one busy invocation with 'Fetching feature #492' label; got %v", labels)
	}
}

// TestStatusCommands_NoSpinnerBytesOnNonTTYStderr verifies that running a
// status handler through the real ui.BusyRun against a bytes.Buffer (which
// is not an *os.File, so the suppression check triggers) produces zero
// spinner bytes on the stderr writer. Protects the --raw-piped-to-pipe
// workflow from spinner noise — explicit regression test for AC-11.
func TestStatusCommands_NoSpinnerBytesOnNonTTYStderr(t *testing.T) {
	sd := fakeStatusDeps(sampleRequirementIssues())
	// Restore the production busy so we exercise the real suppression logic.
	sd.busy = realBusyForTest()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if err := runStatusRequirements(stdout, stderr, statusListFlags{}, sd); err != nil {
		t.Fatalf("runStatusRequirements: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr on non-TTY; got %q", stderr.String())
	}
}

// realBusyForTest returns the production ui.BusyRun wired through without
// a test-owned override. Isolating the import here keeps the test file's
// intent explicit: "use the real thing" versus the injected fake used
// elsewhere in the suite.
func realBusyForTest() func(io.Writer, string, func() error) error {
	return defaultStatusDeps().busy
}
