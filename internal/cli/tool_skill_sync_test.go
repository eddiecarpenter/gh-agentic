package cli

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// skillFilePath is the relative path to the skill that documents the gh
// agentic CLI surface. The lock-step test reads this file once and checks
// that every cobra command path and every declared non-hidden flag appears
// somewhere in its body.
const skillFilePath = "../../skills/gh-agentic/SKILL.md"

// TestGhAgenticToolSkillCoversCLI is the build-time enforcement of the
// Tool / Skill Sync rule (LOCALRULES.md). It walks the cobra command tree
// rooted at `gh-agentic` and asserts that every fully-qualified command
// path and every declared non-hidden flag appears in
// `skills/gh-agentic/SKILL.md`. When either side drifts the test fails
// with a readable diff naming the missing entries.
//
// Hidden flags are excluded — they are intercept points, not part of the
// documented surface. The legacy `--kanban` flag on status list commands
// is the canonical example.
//
// Negative-path verification: temporarily add a throwaway flag to any
// cobra command and re-run `go test ./internal/cli/...`. The diff will
// list the new flag under "Missing flags" until the skill is updated.
// Revert before committing.
func TestGhAgenticToolSkillCoversCLI(t *testing.T) {
	skillBytes, err := os.ReadFile(skillFilePath)
	if err != nil {
		t.Fatalf("read skill %q: %v — Tool/Skill Sync test cannot run without the skill body", skillFilePath, err)
	}
	skill := string(skillBytes)

	root := newRootCmd("test", "")
	paths, flags := walkCobraTree(root)

	missingPaths := make([]string, 0)
	for _, p := range paths {
		if !strings.Contains(skill, p) {
			missingPaths = append(missingPaths, p)
		}
	}
	missingFlags := make([]string, 0)
	for _, f := range flags {
		if !strings.Contains(skill, f) {
			missingFlags = append(missingFlags, f)
		}
	}

	if len(missingPaths) == 0 && len(missingFlags) == 0 {
		return
	}

	var msg strings.Builder
	msg.WriteString("\n")
	msg.WriteString("Tool / Skill Sync drift detected.\n")
	msg.WriteString("Update " + skillFilePath + " — every cobra command path and every declared non-hidden flag must appear in the skill body.\n")
	msg.WriteString("This test is the lock-step enforcement defined in LOCALRULES.md (Tool / Skill Sync).\n")
	if len(missingPaths) > 0 {
		msg.WriteString("\nMissing command paths:\n")
		for _, p := range missingPaths {
			msg.WriteString("  - " + p + "\n")
		}
	}
	if len(missingFlags) > 0 {
		msg.WriteString("\nMissing flags:\n")
		for _, f := range missingFlags {
			msg.WriteString("  - " + f + "\n")
		}
	}
	t.Errorf("%s", msg.String())

	// Orphan detection (skill entries that no longer correspond to any
	// command path or flag in the cobra tree) is a follow-up — the cheap
	// substring approach used here cannot disambiguate documentation prose
	// (e.g. "the `--json` flag has been removed") from claims that a
	// command/flag exists. A proper implementation would parse the skill's
	// command sections.
	// TODO(#589 follow-up): implement orphan detection that respects
	// removal-history sentences.
}

// TestGhAgenticToolSkillCoversCLI_NegativePath proves the test logic
// actually catches drift — runs the same matcher against a synthetic
// cobra tree containing a command path and a flag that the skill body
// definitely does not document, and asserts both are reported as
// missing. This is the permanent, deterministic equivalent of
// "temporarily add a throwaway flag to a real cobra command and observe
// the diff" — running it every test invocation keeps the matcher's
// failure path covered without leaving evidence in production code.
func TestGhAgenticToolSkillCoversCLI_NegativePath(t *testing.T) {
	skillBytes, err := os.ReadFile(skillFilePath)
	if err != nil {
		t.Fatalf("read skill %q: %v", skillFilePath, err)
	}
	skill := string(skillBytes)

	// Build a synthetic root with one sub-command that the real skill
	// does not document. The path "neverdocumented-cmd" and the flag
	// "--neverdocumented-flag" are guaranteed not to appear in the
	// skill body, so the matcher must report them as missing.
	synthRoot := &cobra.Command{Use: "synth-root"}
	syntheticChild := &cobra.Command{Use: "neverdocumented-cmd"}
	syntheticChild.Flags().Bool("neverdocumented-flag", false, "fixture flag")
	synthRoot.AddCommand(syntheticChild)

	paths, flags := walkCobraTree(synthRoot)

	missingPaths := make([]string, 0)
	for _, p := range paths {
		if !strings.Contains(skill, p) {
			missingPaths = append(missingPaths, p)
		}
	}
	missingFlags := make([]string, 0)
	for _, f := range flags {
		if !strings.Contains(skill, f) {
			missingFlags = append(missingFlags, f)
		}
	}

	if len(missingPaths) == 0 {
		t.Errorf("matcher failed to flag the synthetic command path; paths walked = %v", paths)
	} else {
		found := false
		for _, p := range missingPaths {
			if p == "neverdocumented-cmd" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missingPaths did not include 'neverdocumented-cmd'; got %v", missingPaths)
		}
	}
	if len(missingFlags) == 0 {
		t.Errorf("matcher failed to flag the synthetic flag; flags walked = %v", flags)
	} else {
		found := false
		for _, f := range missingFlags {
			if f == "--neverdocumented-flag" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missingFlags did not include '--neverdocumented-flag'; got %v", missingFlags)
		}
	}
}

// TestGhAgenticToolSkillCoversCLI_HiddenFlagsExcluded verifies the
// matcher honours the hidden-flag contract — a hidden flag (e.g. the
// legacy `--kanban` migration intercept) must not appear in the
// declared-flag set returned by walkCobraTree. Without this guarantee
// the lock-step test would force agents to document intercepts that
// are intentionally invisible.
func TestGhAgenticToolSkillCoversCLI_HiddenFlagsExcluded(t *testing.T) {
	cmd := &cobra.Command{Use: "intercept-host"}
	cmd.Flags().Bool("visible-flag", false, "")
	cmd.Flags().Bool("hidden-flag", false, "")
	if err := cmd.Flags().MarkHidden("hidden-flag"); err != nil {
		t.Fatalf("MarkHidden: %v", err)
	}
	root := &cobra.Command{Use: "root"}
	root.AddCommand(cmd)

	_, flags := walkCobraTree(root)
	for _, f := range flags {
		if f == "--hidden-flag" {
			t.Errorf("hidden flag leaked into the documented-surface set: %v", flags)
		}
	}
	found := false
	for _, f := range flags {
		if f == "--visible-flag" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("visible flag missing from walk: %v", flags)
	}
}

// walkCobraTree walks every command rooted at root and returns:
//   - the sorted, deduped fully-qualified command paths (root prefix
//     stripped), excluding the root itself, hidden commands, and the
//     `completion` sub-tree (cobra internals);
//   - the sorted, deduped set of `--<flag-name>` strings declared on any
//     visited command, excluding hidden flags.
func walkCobraTree(root *cobra.Command) (paths []string, flags []string) {
	pathSet := make(map[string]struct{})
	flagSet := make(map[string]struct{})

	var visit func(cmd *cobra.Command, prefix []string)
	visit = func(cmd *cobra.Command, prefix []string) {
		// The root itself contributes no path string — it's the `gh agentic`
		// invocation, not a sub-command.
		if cmd != root {
			if cmd.Hidden {
				return
			}
			if cmd.Name() == "" {
				return
			}
			// Skip cobra's auto-generated completion sub-tree even when
			// it ends up on the tree by accident. The disabling of the
			// default completion command in newRootCmd should keep it
			// off, but this guard makes the intent explicit.
			if cmd.Name() == "completion" {
				return
			}
			path := strings.Join(append(prefix, cmd.Name()), " ")
			pathSet[path] = struct{}{}

			cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
				if f.Hidden {
					return
				}
				flagSet["--"+f.Name] = struct{}{}
			})
		}

		// Recurse — pass the new prefix path for children. The root
		// contributes no segment (it would be "gh-agentic" which we strip).
		var childPrefix []string
		if cmd != root {
			childPrefix = append(append([]string{}, prefix...), cmd.Name())
		}
		for _, sub := range cmd.Commands() {
			visit(sub, childPrefix)
		}
	}
	visit(root, nil)

	for p := range pathSet {
		paths = append(paths, p)
	}
	for f := range flagSet {
		flags = append(flags, f)
	}
	sort.Strings(paths)
	sort.Strings(flags)
	return paths, flags
}
