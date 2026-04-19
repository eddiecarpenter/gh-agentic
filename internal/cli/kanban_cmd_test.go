package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/projectstatus"
	"github.com/eddiecarpenter/gh-agentic/internal/testutil"
)

// kanbanTestDeps returns a statusDeps that serves a flat ProjectIssue
// slice through the projectstatus layer with the current repo fixed to
// eddiecarpenter/gh-agentic. The fetchers upstream filter by type, so a
// single list containing requirements, features, and tasks is enough.
func kanbanTestDeps(issues []projectstatus.ProjectIssue, linked []project.LinkedRepo) statusDeps {
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

// kanbanSampleIssues is a hand-built fixture of non-overlapping
// requirements and features. Three open requirements (447, 457, 467),
// one closed (466); two open features (492, 511), one closed (483).
// 511 lives in the second repo to exercise federation-related flags.
func kanbanSampleIssues() []projectstatus.ProjectIssue {
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

// kanbanSampleDeps bundles the fixture and the linked-repo list used by
// every kanban behavioural test.
func kanbanSampleDeps() statusDeps {
	linked := []project.LinkedRepo{
		{NameWithOwner: "eddiecarpenter/gh-agentic"},
		{NameWithOwner: "foo/domain-x"},
	}
	return kanbanTestDeps(kanbanSampleIssues(), linked)
}

// TestKanbanCmd_RegisteredUnderStatus verifies the `gh agentic status
// kanban` command appears as a direct child of the `status` command. The
// command was promoted to a top-level verb in feature #518 and moved back
// under `status` in feature #549 so all "where are we?" views live behind
// one verb.
func TestKanbanCmd_RegisteredUnderStatus(t *testing.T) {
	root := newRootCmd("test", "test")
	status := findChild(root, "status")
	if status == nil {
		t.Fatalf("status command not registered on root")
	}
	child := findChild(status, "kanban")
	if child == nil {
		t.Fatalf("kanban command not registered under status")
	}
	if child.Use != "kanban" {
		t.Errorf("Use = %q, want %q", child.Use, "kanban")
	}
}

// TestKanbanCmd_NotOnRoot verifies the old top-level `gh agentic kanban`
// registration has been removed. This is the negative-space counterpart to
// TestKanbanCmd_RegisteredUnderStatus — it guards against regressions that
// would silently keep both registrations alive.
func TestKanbanCmd_NotOnRoot(t *testing.T) {
	root := newRootCmd("test", "test")
	if findChild(root, "kanban") != nil {
		t.Errorf("kanban must not be registered as a direct child of root")
	}
}

// TestKanbanCmd_OldPathUnknownCommand verifies that invoking the former
// top-level path (`gh agentic kanban`) now resolves to Cobra's "unknown
// command" error with a non-zero exit — feature #549 scope §Out of Scope
// forbids any deprecation alias, so the hard-move contract must hold.
func TestKanbanCmd_OldPathUnknownCommand(t *testing.T) {
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

// TestStatusCmd_HelpListsKanban verifies the `status --help` output
// advertises the `kanban` subcommand in its Available Commands list. This
// is the user-facing discovery path under the new invocation — AC-5 of
// feature #549.
func TestStatusCmd_HelpListsKanban(t *testing.T) {
	root := newRootCmd("test", "test")
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("status --help: %v", err)
	}
	if !strings.Contains(buf.String(), "kanban") {
		t.Errorf("status --help must list 'kanban' as a subcommand; got:\n%s", buf.String())
	}
}

// TestKanbanCmd_HelpListsFlags verifies --help text mentions every
// documented flag so users can discover the command's full surface.
func TestKanbanCmd_HelpListsFlags(t *testing.T) {
	cmd := newKanbanCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("kanban --help: %v", err)
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

// TestKanbanCmd_AllFlagsRegistered verifies every flag the scope
// documents is declared on the Cobra command — belt-and-braces over the
// help-output check in case the long-description wording drifts.
func TestKanbanCmd_AllFlagsRegistered(t *testing.T) {
	cmd := newKanbanCmd()
	for _, name := range []string{"requirements", "features", "horizontal", "vertical", "include-done", "this-repo", "json"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

// TestKanbanCmd_MutuallyExclusiveSelectorError verifies that passing both
// --requirements and --features produces the documented error and does
// not reach the handler body.
func TestKanbanCmd_MutuallyExclusiveSelectorError(t *testing.T) {
	cmd := newKanbanCmdWithDeps(kanbanSampleDeps())
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

// TestKanbanCmd_PositionalArgsRejected verifies cobra.NoArgs is honoured
// so a stray positional fails fast without invoking the handler.
func TestKanbanCmd_PositionalArgsRejected(t *testing.T) {
	cmd := newKanbanCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"oops"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for positional argument; got nil")
	}
}

// TestKanbanCmd_RootLongDescriptionOmitsKanban verifies the top-level
// `gh agentic --help` output no longer lists `kanban` as a top-level
// command — AC-4 of feature #549. Cobra renders every direct child of the
// root in its Available Commands block, so removing the child registration
// is enough; this test asserts that removal is reflected in help output.
func TestKanbanCmd_RootLongDescriptionOmitsKanban(t *testing.T) {
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
	// description still mentions "kanban sub-view" as part of describing
	// `status`, so a naive full-text check would fail. Cobra's help lists
	// the command name as the first token on each indented line after the
	// "Available Commands:" heading.
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
		if fields[0] == "kanban" {
			t.Errorf("root Available Commands must not list 'kanban'; got block:\n%s", block)
		}
	}
}

// -----------------------------------------------------------------------
// Behavioural tests — the real handler (task #523) delivers these.
// -----------------------------------------------------------------------

// wideTerminalForKanbanTest pins terminalWidth() to a value above the
// feature kanban's 120-column threshold so the horizontal path is
// exercised in tests. Callers use a `defer restore()` pattern.
func wideTerminalForKanbanTest(t *testing.T) func() {
	t.Helper()
	original := terminalWidth
	terminalWidth = func() int { return 200 }
	return func() { terminalWidth = original }
}

// TestRunKanban_DefaultStackedOutput verifies a bare invocation renders
// the requirements kanban first, a blank-line separator, then the
// features kanban, then a combined totals line — AC-1.
func TestRunKanban_DefaultStackedOutput(t *testing.T) {
	defer wideTerminalForKanbanTest(t)()

	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban: %v", err)
	}
	out := buf.String()

	reqIdx := strings.Index(out, "=== Requirements — Kanban ===")
	feaIdx := strings.Index(out, "=== Features — Kanban ===")
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

// TestRunKanban_RequirementsSelector verifies --requirements renders
// only the requirements kanban with the requirements totals line — AC-2.
func TestRunKanban_RequirementsSelector(t *testing.T) {
	defer wideTerminalForKanbanTest(t)()

	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{requirements: true}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "=== Requirements — Kanban ===") {
		t.Errorf("expected requirements heading; got:\n%s", out)
	}
	if strings.Contains(out, "=== Features — Kanban ===") {
		t.Errorf("features heading must not appear with --requirements; got:\n%s", out)
	}
	if !strings.Contains(out, "3 open requirements") {
		t.Errorf("expected requirements totals line; got:\n%s", out)
	}
	if strings.Contains(out, "open features") {
		t.Errorf("features totals should not appear; got:\n%s", out)
	}
}

// TestRunKanban_FeaturesSelector verifies --features renders only the
// features kanban with the features totals line — AC-3.
func TestRunKanban_FeaturesSelector(t *testing.T) {
	defer wideTerminalForKanbanTest(t)()

	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{features: true}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "=== Features — Kanban ===") {
		t.Errorf("expected features heading; got:\n%s", out)
	}
	if strings.Contains(out, "=== Requirements — Kanban ===") {
		t.Errorf("requirements heading must not appear with --features; got:\n%s", out)
	}
	if !strings.Contains(out, "2 open features") {
		t.Errorf("expected features totals line; got:\n%s", out)
	}
}

// TestRunKanban_IncludeDoneAddsDoneColumnBothKanbans verifies
// --include-done appends the `done` column to both kanbans — AC-5.
func TestRunKanban_IncludeDoneAddsDoneColumnBothKanbans(t *testing.T) {
	// Force vertical rendering so the `## done` heading appears (the
	// horizontal path uses inline column headers on the border line).
	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{includeDone: true, vertical: true}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban: %v", err)
	}
	out := buf.String()
	// Both kanbans must surface a `## done` section heading.
	if strings.Count(out, "## done") < 2 {
		t.Errorf("expected 'done' column on both kanbans; got:\n%s", out)
	}
}

// TestRunKanban_VerticalForcesVertical verifies --vertical disables
// the horizontal table even on wide terminals — AC-5.
func TestRunKanban_VerticalForcesVertical(t *testing.T) {
	defer wideTerminalForKanbanTest(t)()

	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{vertical: true}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban: %v", err)
	}
	out := buf.String()
	// Vertical rendering emits ## <stage> section headings; horizontal
	// does not. Expect at least one on each kanban.
	if !strings.Contains(out, "## backlog") {
		t.Errorf("expected '## backlog' section heading (vertical); got:\n%s", out)
	}
}

// TestRunKanban_HorizontalForcesHorizontal verifies --horizontal forces
// the horizontal table even on narrow terminals — AC-5.
func TestRunKanban_HorizontalForcesHorizontal(t *testing.T) {
	original := terminalWidth
	terminalWidth = func() int { return 60 }
	defer func() { terminalWidth = original }()

	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{horizontal: true}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban: %v", err)
	}
	out := buf.String()
	if !strings.ContainsAny(out, "┌+") {
		t.Errorf("expected horizontal table borders; got:\n%s", out)
	}
}

// TestRunKanban_ThisRepoNarrowsBothLists verifies --this-repo drops
// cross-repo items from both kanbans — AC-5.
func TestRunKanban_ThisRepoNarrowsBothLists(t *testing.T) {
	defer wideTerminalForKanbanTest(t)()

	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{thisRepo: true}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban: %v", err)
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

// TestRunKanban_BothFlagsLayoutConflict verifies --horizontal and
// --vertical together yield a mutually-exclusive error — AC-5.
func TestRunKanban_BothFlagsLayoutConflict(t *testing.T) {
	err := runKanban(&bytes.Buffer{}, &bytes.Buffer{}, kanbanFlags{horizontal: true, vertical: true}, kanbanSampleDeps())
	if err == nil {
		t.Fatalf("expected mutually-exclusive error, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected message to mention 'mutually exclusive'; got %q", err.Error())
	}
}

// TestRunKanban_JSONDefaultEnvelopeShape verifies --json with no
// selector emits {requirements, features, totals} — AC-6.
func TestRunKanban_JSONDefaultEnvelopeShape(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{json: true}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban --json: %v", err)
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

// TestRunKanban_JSONRequirementsSelectorOmitsFeatures verifies AC-7 for
// the --requirements selector: the `features` key is absent, not null.
func TestRunKanban_JSONRequirementsSelectorOmitsFeatures(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{json: true, requirements: true}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban --json --requirements: %v", err)
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

// TestRunKanban_JSONFeaturesSelectorOmitsRequirements is the symmetric
// check to TestRunKanban_JSONRequirementsSelectorOmitsFeatures.
func TestRunKanban_JSONFeaturesSelectorOmitsRequirements(t *testing.T) {
	buf := &bytes.Buffer{}
	err := runKanban(buf, &bytes.Buffer{}, kanbanFlags{json: true, features: true}, kanbanSampleDeps())
	if err != nil {
		t.Fatalf("runKanban --json --features: %v", err)
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

// TestRunKanban_InvokesBusyWrapper verifies the kanban handler wraps
// its fetch via deps.busy with the appropriate label — AC-9 / AC-11.
func TestRunKanban_InvokesBusyWrapper(t *testing.T) {
	rec := &recordingBusy{}
	sd := kanbanSampleDeps()
	sd.busy = rec.Run

	// Default (both) — expect "Fetching pipeline state…".
	if err := runKanban(&bytes.Buffer{}, &bytes.Buffer{}, kanbanFlags{vertical: true}, sd); err != nil {
		t.Fatalf("runKanban default: %v", err)
	}
	// --requirements — expect "Fetching requirements…".
	if err := runKanban(&bytes.Buffer{}, &bytes.Buffer{}, kanbanFlags{vertical: true, requirements: true}, sd); err != nil {
		t.Fatalf("runKanban requirements: %v", err)
	}
	// --features — expect "Fetching features…".
	if err := runKanban(&bytes.Buffer{}, &bytes.Buffer{}, kanbanFlags{vertical: true, features: true}, sd); err != nil {
		t.Fatalf("runKanban features: %v", err)
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

// TestRunKanban_NonTTYStderrProducesNoSpinnerBytes verifies AC-11 at
// the kanban level — captured stderr contains no spinner glyphs.
func TestRunKanban_NonTTYStderrProducesNoSpinnerBytes(t *testing.T) {
	sd := kanbanSampleDeps()
	sd.busy = realBusyForTest()

	stderr := &bytes.Buffer{}
	if err := runKanban(&bytes.Buffer{}, stderr, kanbanFlags{vertical: true}, sd); err != nil {
		t.Fatalf("runKanban: %v", err)
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
