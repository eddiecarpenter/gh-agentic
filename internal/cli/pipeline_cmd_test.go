package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// pipelineTestDeps returns a statusDeps that serves a flat ProjectIssue
// slice through the projectstatus layer with the current repo fixed to
// eddiecarpenter/gh-agentic. The fetchers upstream filter by type, so a
// single list containing requirements, features, and tasks is enough.
func pipelineTestDeps(issues []projectstatus.ProjectIssue, linked []project.LinkedRepo) statusDeps {
	return statusDeps{
		currentRepo:      func() (string, error) { return "eddiecarpenter/gh-agentic", nil },
		resolveProjectID: func(string) (string, error) { return "PROJ_ID", nil },
		psDeps: projectstatus.Deps{
			FetchProjectIssues: func(string) ([]projectstatus.ProjectIssue, error) {
				return issues, nil
			},
			FetchLinkedRepos: func(string) ([]project.LinkedRepo, error) {
				return linked, nil
			},
		},
		busy: testutil.NoopBusy,
	}
}

// pipelineSampleIssues is a hand-built fixture of non-overlapping
// requirements and features. Three open requirements (447, 457, 467),
// one closed (466); two open features (492, 511), one closed (483).
// 511 lives in the second repo to exercise federation-related flags.
func pipelineSampleIssues() []projectstatus.ProjectIssue {
	return []projectstatus.ProjectIssue{
		// Requirements ----------------------------------------------
		{Number: 447, Title: "feat: project lifecycle management", Stage: projectstatus.StageBacklog, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic"},
		{Number: 457, Title: "feat: single-pane pipeline status view", Stage: projectstatus.StageScoping, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic"},
		{Number: 467, Title: "feat: skill-publishing", Stage: projectstatus.StageBacklog, Type: "requirement", State: "open", OwningRepo: "eddiecarpenter/gh-agentic"},
		{Number: 466, Title: "feat: ask-user", Stage: projectstatus.StageDone, Type: "requirement", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic"},
		// Features --------------------------------------------------
		{Number: 483, Title: "feat: ask-user skill", Stage: projectstatus.StageDone, Type: "feature", State: "closed", OwningRepo: "eddiecarpenter/gh-agentic"},
		{Number: 492, Title: "feat: status command", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "eddiecarpenter/gh-agentic"},
		{Number: 511, Title: "feat: domain-X event handler", Stage: projectstatus.StageInDevelopment, Type: "feature", State: "open", OwningRepo: "foo/domain-x"},
	}
}

// pipelineSampleDeps bundles the fixture and the linked-repo list used by
// every pipeline behavioural test.
func pipelineSampleDeps() statusDeps {
	linked := []project.LinkedRepo{
		{NameWithOwner: "eddiecarpenter/gh-agentic"},
		{NameWithOwner: "foo/domain-x"},
	}
	return pipelineTestDeps(pipelineSampleIssues(), linked)
}

// TestPipelineCmd_RegisteredUnderStatus verifies the `gh agentic status
// pipeline` command appears as a direct child of the `status` command.
// The previous top-level kanban command was promoted in feature #518,
// moved under `status` in feature #549, and renamed to `pipeline` in
// feature #562 so the command name matches its read-only semantics and
// the codebase consistently uses the "pipeline" vocabulary.
func TestPipelineCmd_RegisteredUnderStatus(t *testing.T) {
	root := newRootCmd("test", "test")
	status := findChild(root, "status")
	if status == nil {
		t.Fatalf("status command not registered on root")
	}
	child := findChild(status, "pipeline")
	if child == nil {
		t.Fatalf("pipeline command not registered under status")
	}
	if child.Use != "pipeline" {
		t.Errorf("Use = %q, want %q", child.Use, "pipeline")
	}
}

// TestPipelineCmd_NotOnRoot verifies the pipeline command is not
// registered at the top level of the command tree — it belongs under
// `status` as a sub-command, not as a root-level verb.
func TestPipelineCmd_NotOnRoot(t *testing.T) {
	root := newRootCmd("test", "test")
	if findChild(root, "pipeline") != nil {
		t.Errorf("pipeline must not be registered as a direct child of root")
	}
}

// TestPipelineCmd_LegacyKanbanPathUnknownCommand verifies that invoking
// the legacy top-level path (`gh agentic kanban`) resolves to Cobra's
// "unknown command" error with a non-zero exit — feature #562 scope
// forbids any deprecation alias, so the hard-move contract must hold.
func TestPipelineCmd_LegacyKanbanPathUnknownCommand(t *testing.T) {
	root := newRootCmd("test", "test")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"kanban"})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected Cobra unknown-command error; got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' in error; got %q", err.Error())
	}
}

// TestStatusCmd_HelpListsPipeline verifies the `status --help` output
// advertises the `pipeline` subcommand in its Available Commands list.
// This is the user-facing discovery path under the new invocation.
func TestStatusCmd_HelpListsPipeline(t *testing.T) {
	root := newRootCmd("test", "test")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("status --help: %v", err)
	}
	if !strings.Contains(buf.String(), "pipeline") {
		t.Errorf("status --help must list 'pipeline' as a subcommand; got:\n%s", buf.String())
	}
}

// TestPipelineCmd_HelpListsFlags verifies --help text mentions every
// documented flag so users can discover the command's full surface.
func TestPipelineCmd_HelpListsFlags(t *testing.T) {
	cmd := newPipelineCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pipeline --help: %v", err)
	}
	out := buf.String()
	for _, tok := range []string{
		"--requirements",
		"--features",
		"--horizontal",
		"--vertical",
		"--include-done",
		"--this-repo",
		"--json",
	} {
		if !strings.Contains(out, tok) {
			t.Errorf("help missing flag %q; got:\n%s", tok, out)
		}
	}
}

// TestPipelineCmd_AllFlagsRegistered verifies every flag the scope
// documents is declared on the Cobra command — belt-and-braces over the
// help-output check in case the long-description wording drifts.
func TestPipelineCmd_AllFlagsRegistered(t *testing.T) {
	cmd := newPipelineCmd()
	for _, name := range []string{"requirements", "features", "horizontal", "vertical", "include-done", "this-repo", "json", "raw"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

// TestPipelineCmd_MutuallyExclusiveSelectorError verifies that passing
// both --requirements and --features produces the documented error and
// does not reach the handler body.
func TestPipelineCmd_MutuallyExclusiveSelectorError(t *testing.T) {
	cmd := newPipelineCmdWithDeps(pipelineSampleDeps())
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--requirements", "--features"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected mutually-exclusive error; got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--requirements") || !strings.Contains(msg, "--features") {
		t.Errorf("error should name both selector flags; got %q", msg)
	}
	if !strings.Contains(msg, "mutually exclusive") {
		t.Errorf("error should mention 'mutually exclusive'; got %q", msg)
	}
}

// TestPipelineCmd_PositionalArgsRejected verifies cobra.NoArgs is
// honoured so a stray positional fails fast without invoking the handler.
func TestPipelineCmd_PositionalArgsRejected(t *testing.T) {
	cmd := newPipelineCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"oops"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for positional argument; got nil")
	}
}

// TestPipelineCmd_RootLongDescriptionOmitsPipeline verifies the top-level
// `gh agentic --help` output does not list `pipeline` as a top-level
// command — pipeline belongs under `status`, not at the root. Cobra
// renders every direct child of the root in its Available Commands
// block, so absence there is enough; this test asserts that absence.
func TestPipelineCmd_RootLongDescriptionOmitsPipeline(t *testing.T) {
	root := newRootCmd("test", "test")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("root --help: %v", err)
	}
	out := buf.String()
	// Scan the Available Commands section specifically — the long
	// description may mention "pipeline" as part of describing `status`,
	// so a naive full-text check would be misleading. Cobra's help
	// lists the command name as the first token on each indented line
	// after the "Available Commands:" heading.
	const header = "Available Commands:"
	idx := strings.Index(out, header)
	if idx < 0 {
		t.Fatalf("help missing Available Commands section; got:\n%s", out)
	}
	// Take the block up to the next blank-line-after-commands marker
	// (typically "Flags:"). A simple approach: slice to "Flags:" if
	// present, otherwise to end.
	block := out[idx:]
	if end := strings.Index(block, "\nFlags:"); end >= 0 {
		block = block[:end]
	}
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		// Each command line is "<name>   <description>". Split on
		// whitespace to isolate the name token.
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "pipeline" {
			t.Errorf("root Available Commands must not list 'pipeline'; got block:\n%s", block)
		}
	}
}

// -----------------------------------------------------------------------
// Behavioural tests — the real handler (task #523) delivers these.
// -----------------------------------------------------------------------

// wideTerminalForPipelineTest pins terminalWidth() to a value above the
// feature pipeline's 120-column threshold so the horizontal path is
// exercised in tests. Callers use a `defer restore()` pattern.
func wideTerminalForPipelineTest(t *testing.T) func() {
	t.Helper()
	original := terminalWidth
	terminalWidth = func() int { return 200 }
	return func() { terminalWidth = original }
}

// TestRunPipeline_DefaultStackedOutput verifies a bare invocation renders
// the requirements pipeline first, a blank-line separator, then the
// features pipeline, then a combined totals line — AC-1.
func TestRunPipeline_DefaultStackedOutput(t *testing.T) {
	defer wideTerminalForPipelineTest(t)()

	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
	out := buf.String()

	reqIdx := strings.Index(out, "=== Requirements — Pipeline ===")
	feaIdx := strings.Index(out, "=== Features — Pipeline ===")
	if reqIdx < 0 {
		t.Errorf("expected requirements heading; got:\n%s", out)
	}
	if feaIdx < 0 {
		t.Errorf("expected features heading; got:\n%s", out)
	}
	if reqIdx >= 0 && feaIdx >= 0 && reqIdx >= feaIdx {
		t.Errorf("requirements heading must precede features heading; got reqIdx=%d feaIdx=%d", reqIdx, feaIdx)
	}
	// Combined totals line must carry both entity counts — sample
	// fixture has 3 open requirements (467, 457, 447) and 2 open
	// features (492, 511).
	if !strings.Contains(out, "3 open requirements · 2 open features") {
		t.Errorf("expected combined totals line; got:\n%s", out)
	}
}

// TestRunPipeline_RequirementsSelector verifies --requirements renders
// only the requirements pipeline with the requirements totals line — AC-2.
func TestRunPipeline_RequirementsSelector(t *testing.T) {
	defer wideTerminalForPipelineTest(t)()

	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{requirements: true}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "=== Requirements — Pipeline ===") {
		t.Errorf("expected requirements heading; got:\n%s", out)
	}
	if strings.Contains(out, "=== Features — Pipeline ===") {
		t.Errorf("features heading must not appear with --requirements; got:\n%s", out)
	}
	if !strings.Contains(out, "3 open requirements") {
		t.Errorf("expected requirements totals line; got:\n%s", out)
	}
	if strings.Contains(out, "open features") {
		t.Errorf("features totals should not appear; got:\n%s", out)
	}
}

// TestRunPipeline_FeaturesSelector verifies --features renders only the
// features pipeline with the features totals line — AC-3.
func TestRunPipeline_FeaturesSelector(t *testing.T) {
	defer wideTerminalForPipelineTest(t)()

	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{features: true}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "=== Features — Pipeline ===") {
		t.Errorf("expected features heading; got:\n%s", out)
	}
	if strings.Contains(out, "=== Requirements — Pipeline ===") {
		t.Errorf("requirements heading must not appear with --features; got:\n%s", out)
	}
	if !strings.Contains(out, "2 open features") {
		t.Errorf("expected features totals line; got:\n%s", out)
	}
}

// TestRunPipeline_IncludeDoneAddsDoneColumnBothPipelines verifies
// --include-done appends the `done` column to both pipelines — AC-5.
func TestRunPipeline_IncludeDoneAddsDoneColumnBothPipelines(t *testing.T) {
	// Force vertical rendering so the `## done` heading appears (the
	// horizontal path uses inline column headers on the border line).
	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{includeDone: true, vertical: true}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
	out := buf.String()
	// Both pipelines must surface a `## done` section heading.
	if strings.Count(out, "## done") < 2 {
		t.Errorf("expected 'done' column on both pipelines; got:\n%s", out)
	}
}

// TestRunPipeline_VerticalForcesVertical verifies --vertical disables
// the horizontal table even on wide terminals — AC-5.
func TestRunPipeline_VerticalForcesVertical(t *testing.T) {
	defer wideTerminalForPipelineTest(t)()

	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{vertical: true}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
	out := buf.String()
	// Vertical rendering emits ## <stage> section headings; horizontal
	// does not. Expect at least one on each pipeline.
	if !strings.Contains(out, "## backlog") {
		t.Errorf("expected '## backlog' section heading (vertical); got:\n%s", out)
	}
}

// TestRunPipeline_HorizontalForcesHorizontal verifies --horizontal forces
// the horizontal table even on narrow terminals — AC-5.
func TestRunPipeline_HorizontalForcesHorizontal(t *testing.T) {
	original := terminalWidth
	terminalWidth = func() int { return 60 }
	defer func() { terminalWidth = original }()

	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{horizontal: true}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
	out := buf.String()
	if !strings.ContainsAny(out, "┌+") {
		t.Errorf("expected horizontal table borders; got:\n%s", out)
	}
}

// TestRunPipeline_ThisRepoNarrowsBothLists verifies --this-repo drops
// cross-repo items from both pipelines — AC-5.
func TestRunPipeline_ThisRepoNarrowsBothLists(t *testing.T) {
	defer wideTerminalForPipelineTest(t)()

	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{thisRepo: true}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
	out := buf.String()
	// #511 is the foo/domain-x feature — must be filtered out.
	if strings.Contains(out, "#511") {
		t.Errorf("--this-repo must drop cross-repo #511; got:\n%s", out)
	}
	// #492 is the local feature — must stay.
	if !strings.Contains(out, "#492") {
		t.Errorf("--this-repo must retain local #492; got:\n%s", out)
	}
}

// TestRunPipeline_BothFlagsLayoutConflict verifies --horizontal and
// --vertical together yield a mutually-exclusive error — AC-5.
func TestRunPipeline_BothFlagsLayoutConflict(t *testing.T) {
	err := runPipeline(&bytes.Buffer{}, &bytes.Buffer{}, pipelineFlags{horizontal: true, vertical: true}, pipelineSampleDeps())
	if err == nil {
		t.Fatalf("expected mutually-exclusive error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected message to mention 'mutually exclusive'; got %q", err.Error())
	}
}

// TestRunPipeline_JSONDefaultEnvelopeShape verifies --json with no
// selector emits {requirements, features, totals} — AC-6.
func TestRunPipeline_JSONDefaultEnvelopeShape(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{json: true}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline --json: %v", err)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal: %v; raw:\n%s", err, buf.String())
	}
	for _, key := range []string{"requirements", "features", "totals"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("envelope missing key %q; keys = %v", key, keysOfRaw(parsed))
		}
	}
	// Inner totals must have the three locked fields and no extras.
	var totals map[string]interface{}
	if err := json.Unmarshal(parsed["totals"], &totals); err != nil {
		t.Fatalf("totals: %v", err)
	}
	want := map[string]bool{"open_requirements": true, "open_features": true, "blocked": true}
	for k := range totals {
		if !want[k] {
			t.Errorf("unexpected totals key %q", k)
		}
	}
	for k := range want {
		if _, ok := totals[k]; !ok {
			t.Errorf("totals missing key %q; got %v", k, keysOf(totals))
		}
	}
}

// TestRunPipeline_JSONRequirementsSelectorOmitsFeatures verifies AC-7 for
// the --requirements selector: the `features` key is absent, not null.
func TestRunPipeline_JSONRequirementsSelectorOmitsFeatures(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{json: true, requirements: true}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline --json --requirements: %v", err)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal: %v; raw:\n%s", err, buf.String())
	}
	if _, present := parsed["features"]; present {
		t.Errorf("features key must be absent with --requirements; keys = %v", keysOfRaw(parsed))
	}
	if _, present := parsed["requirements"]; !present {
		t.Errorf("requirements key must be present; keys = %v", keysOfRaw(parsed))
	}
	// Totals should carry open_requirements but not open_features.
	var totals map[string]interface{}
	if err := json.Unmarshal(parsed["totals"], &totals); err != nil {
		t.Fatalf("totals: %v", err)
	}
	if _, present := totals["open_features"]; present {
		t.Errorf("open_features must be absent under --requirements; got %v", keysOf(totals))
	}
	if _, present := totals["open_requirements"]; !present {
		t.Errorf("open_requirements must be present; got %v", keysOf(totals))
	}
}

// TestRunPipeline_JSONFeaturesSelectorOmitsRequirements is the symmetric
// check to TestRunPipeline_JSONRequirementsSelectorOmitsFeatures.
func TestRunPipeline_JSONFeaturesSelectorOmitsRequirements(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runPipeline(buf, &bytes.Buffer{}, pipelineFlags{json: true, features: true}, pipelineSampleDeps())
	if err != nil {
		t.Fatalf("runPipeline --json --features: %v", err)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal: %v; raw:\n%s", err, buf.String())
	}
	if _, present := parsed["requirements"]; present {
		t.Errorf("requirements key must be absent with --features; keys = %v", keysOfRaw(parsed))
	}
	if _, present := parsed["features"]; !present {
		t.Errorf("features key must be present; keys = %v", keysOfRaw(parsed))
	}
	var totals map[string]interface{}
	if err := json.Unmarshal(parsed["totals"], &totals); err != nil {
		t.Fatalf("totals: %v", err)
	}
	if _, present := totals["open_requirements"]; present {
		t.Errorf("open_requirements must be absent under --features; got %v", keysOf(totals))
	}
}

// TestRunPipeline_InvokesBusyWrapper verifies the pipeline handler wraps
// its fetch via deps.busy with the appropriate label — AC-9 / AC-11.
func TestRunPipeline_InvokesBusyWrapper(t *testing.T) {
	rec := &recordingBusy{}
	sd := pipelineSampleDeps()
	sd.busy = rec.Run

	// Default (both) — expect "Fetching pipeline state…".
	if err := runPipeline(&bytes.Buffer{}, &bytes.Buffer{}, pipelineFlags{vertical: true}, sd); err != nil {
		t.Fatalf("runPipeline default: %v", err)
	}
	// --requirements — expect "Fetching requirements…".
	if err := runPipeline(&bytes.Buffer{}, &bytes.Buffer{}, pipelineFlags{vertical: true, requirements: true}, sd); err != nil {
		t.Fatalf("runPipeline requirements: %v", err)
	}
	// --features — expect "Fetching features…".
	if err := runPipeline(&bytes.Buffer{}, &bytes.Buffer{}, pipelineFlags{vertical: true, features: true}, sd); err != nil {
		t.Fatalf("runPipeline features: %v", err)
	}
	labels := rec.snapshot()
	if len(labels) != 3 {
		t.Fatalf("expected 3 busy invocations; got %d: %v", len(labels), labels)
	}
	if !strings.Contains(labels[0], "pipeline state") {
		t.Errorf("first label should mention 'pipeline state'; got %q", labels[0])
	}
	if !strings.Contains(labels[1], "requirements") {
		t.Errorf("second label should mention 'requirements'; got %q", labels[1])
	}
	if !strings.Contains(labels[2], "features") {
		t.Errorf("third label should mention 'features'; got %q", labels[2])
	}
}

// TestRunPipeline_NonTTYStderrProducesNoSpinnerBytes verifies AC-11 at
// the pipeline level — captured stderr contains no spinner glyphs.
func TestRunPipeline_NonTTYStderrProducesNoSpinnerBytes(t *testing.T) {
	sd := pipelineSampleDeps()
	sd.busy = realBusyForTest()

	stderr := &bytes.Buffer{}
	if err := runPipeline(&bytes.Buffer{}, stderr, pipelineFlags{vertical: true}, sd); err != nil {
		t.Fatalf("runPipeline: %v", err)
	}
	if stderr.Len() != 0 {
		t.Errorf("non-TTY stderr must be empty; got %q", stderr.String())
	}
}

// keysOfRaw returns the keys of a map[string]json.RawMessage for
// human-readable failure messages.
func keysOfRaw(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestRunPipeline_RawCombinedShape verifies the bare `pipeline --raw`
// invocation emits both sections separated by a blank line, that each
// section uses the Task 1 list TSV header + rows, and that the rendered
// bytes match the combined golden.
func TestRunPipeline_RawCombinedShape(t *testing.T) {
	sd := pipelineSampleDeps()
	buf := &bytes.Buffer{}
	if err := runPipeline(buf, io.Discard, pipelineFlags{raw: true}, sd); err != nil {
		t.Fatalf("runPipeline --raw: %v", err)
	}

	got := buf.Bytes()
	wantBytes, err := os.ReadFile("testdata/status_raw/pipeline_combined.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("combined --raw output does not match golden\nwant:\n%s\ngot:\n%s", string(wantBytes), string(got))
	}

	// Both section markers must be present and in order.
	out := string(got)
	reqIdx := strings.Index(out, "# requirements")
	feaIdx := strings.Index(out, "# features")
	if reqIdx < 0 || feaIdx < 0 {
		t.Fatalf("expected both '# requirements' and '# features' markers; got:\n%s", out)
	}
	if reqIdx >= feaIdx {
		t.Errorf("requirements section must precede features section")
	}
}

// TestRunPipeline_RawRequirementsSelector verifies the `--requirements`
// selector emits only the `# requirements` section — no `# features`
// marker anywhere in the output.
func TestRunPipeline_RawRequirementsSelector(t *testing.T) {
	sd := pipelineSampleDeps()
	buf := &bytes.Buffer{}
	if err := runPipeline(buf, io.Discard, pipelineFlags{raw: true, requirements: true}, sd); err != nil {
		t.Fatalf("runPipeline --raw --requirements: %v", err)
	}

	got := buf.Bytes()
	wantBytes, err := os.ReadFile("testdata/status_raw/pipeline_requirements.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("requirements --raw output does not match golden\nwant:\n%s\ngot:\n%s", string(wantBytes), string(got))
	}
	if strings.Contains(string(got), "# features") {
		t.Errorf("requirements selector must not emit '# features' marker; got:\n%s", string(got))
	}
}

// TestRunPipeline_RawFeaturesSelector verifies the `--features` selector
// emits only the `# features` section — no `# requirements` marker.
func TestRunPipeline_RawFeaturesSelector(t *testing.T) {
	sd := pipelineSampleDeps()
	buf := &bytes.Buffer{}
	if err := runPipeline(buf, io.Discard, pipelineFlags{raw: true, features: true}, sd); err != nil {
		t.Fatalf("runPipeline --raw --features: %v", err)
	}

	got := buf.Bytes()
	wantBytes, err := os.ReadFile("testdata/status_raw/pipeline_features.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("features --raw output does not match golden\nwant:\n%s\ngot:\n%s", string(wantBytes), string(got))
	}
	if strings.Contains(string(got), "# requirements") {
		t.Errorf("features selector must not emit '# requirements' marker; got:\n%s", string(got))
	}
}

// TestRunPipeline_RawIgnoresLayoutFlags verifies that --horizontal /
// --vertical are no-ops under --raw — the rendered bytes match the
// layout-free combined golden regardless of the layout selector.
func TestRunPipeline_RawIgnoresLayoutFlags(t *testing.T) {
	sd := pipelineSampleDeps()
	wantBytes, err := os.ReadFile("testdata/status_raw/pipeline_combined.raw")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	for _, layout := range []pipelineFlags{
		{raw: true, horizontal: true},
		{raw: true, vertical: true},
	} {
		buf := &bytes.Buffer{}
		if err := runPipeline(buf, io.Discard, layout, sd); err != nil {
			t.Fatalf("runPipeline --raw layout=%+v: %v", layout, err)
		}
		if !bytes.Equal(buf.Bytes(), wantBytes) {
			t.Errorf("layout %+v under --raw should be a no-op; bytes diverged", layout)
		}
	}
}
