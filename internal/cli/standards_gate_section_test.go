package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// gateSectionHeading is the exact heading that compliance-verify's Section
// A.0 (the mandatory build+test gate) parses out of each detected stack's
// standards file. The verifier reads the commands under this heading to run
// the gate; a standards file that lacks it raises STACK_GATE_UNDEFINED and
// the verifier blocks — a framework-level gap, not a code FAIL.
//
// See skills/compliance-verify/SKILL.md Section A.0 outcome #5.
const gateSectionHeading = "## Verification Gate (build + test)"

// languageStandardsRequiringGate is the set of per-language standards files
// that describe an executable build+test toolchain and therefore MUST define
// the Verification Gate section. These are the stacks a Feature's diff can be
// detected as touching; compliance-verify needs a gate contract for each.
//
// Non-language standards files (e.g. a generic "documentation.md" if one is
// added) are intentionally excluded — they have no build+test gate to run.
var languageStandardsRequiringGate = []string{
	"go.md",
	"java.md",
	"typescript.md",
	"react.md",
}

// TestStandardsFilesDefineVerificationGate is the build-time guard for the
// framework gap that left Feature #8 (OpenBSS Branding) stuck: standards/java.md
// shipped without a `## Verification Gate (build + test)` section, so
// compliance-verify raised STACK_GATE_UNDEFINED on every Java-touching Feature
// and the workflow safety-net cycled the issue between in-verification and
// in-development indefinitely.
//
// Every language standards file that defines an executable toolchain must carry
// the gate heading. This test reads the real standards/*.md files at the repo
// root (../.. from this package, mirroring TestNoLegacyAuthRefsInWorkflows) and
// fails — listing every offender — if any is missing the section.
func TestStandardsFilesDefineVerificationGate(t *testing.T) {
	standardsDir := filepath.Join(repoRootFromCli, "standards")

	var missing []string
	for _, name := range languageStandardsRequiringGate {
		path := filepath.Join(standardsDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %q: %v — a language standards file required to define the "+
				"Verification Gate is absent or unreadable", path, err)
		}
		if !strings.Contains(string(content), gateSectionHeading) {
			missing = append(missing, name)
		}
	}

	if len(missing) == 0 {
		return
	}

	sort.Strings(missing)
	var msg strings.Builder
	msg.WriteString("\n")
	msg.WriteString("Language standards file(s) missing the Verification Gate section.\n\n")
	msg.WriteString("compliance-verify Section A.0 parses the build+test commands out of the\n")
	msg.WriteString("heading:\n\n")
	msg.WriteString("    " + gateSectionHeading + "\n\n")
	msg.WriteString("A detected stack whose standards file lacks this heading raises\n")
	msg.WriteString("STACK_GATE_UNDEFINED and the verifier blocks — leaving the Feature stuck.\n\n")
	msg.WriteString("Offending files (under standards/):\n")
	for _, m := range missing {
		msg.WriteString("  " + m + "\n")
	}
	msg.WriteString("\nRemediation: add a `" + gateSectionHeading + "` section mirroring the\n")
	msg.WriteString("one in standards/go.md (commands, manifest pre-check, dev/compliance\n")
	msg.WriteString("hooks, and the toolchain-absent → SKIPPED-with-WARN softening).\n")
	t.Errorf("%s", msg.String())
}
