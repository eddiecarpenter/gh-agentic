package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repoRootFromCli is the path from this test's package directory to the
// repository root. Mirrors the pattern in tool_skill_sync_test.go which
// reads "../../skills/...".
const repoRootFromCli = "../.."

// legacyRefs is the set of legacy identifiers from the pre-#622 PAT-based
// authentication model. After Feature #622, every workflow file and
// composite action under .github/ must mint a GitHub App installation
// token via actions/create-github-app-token and consume that token
// instead. Re-introducing any of these strings in a workflow or action
// is a regression: it either leaks the dormant goose-agent identity back
// into CI or implies a new code path that bypasses the App.
//
// The list is deliberately small and exact-match. Substring matches into
// adjacent identifiers (e.g. AGENT_USER_PROFILE) are tolerated by ripgrep
// in practice — for the build-time guard we use literal strings.
var legacyRefs = []string{
	"GOOSE_AGENT_PAT",
	"AGENT_USER",
}

// TestNoLegacyAuthRefsInWorkflows is the build-time guard for Feature
// #622's acceptance criterion that "Zero `secrets.GOOSE_AGENT_PAT`
// references remain in any workflow file" and the equivalent for
// `vars.AGENT_USER`. It walks .github/workflows/**/*.yml,
// .github/workflows/**/*.yaml, and .github/actions/**/action.yml
// relative to the repo root and fails — listing every offending
// file:line pair — if any of the legacy identifiers reappear.
//
// The test is the permanent successor to manual `grep -rn` audits. CI
// must not let a regression silently re-introduce the goose-agent PAT
// or the AGENT_USER variable. If you need to refer to these names in
// documentation or commentary, do so outside .github/.
//
// Pattern source: TestGhAgenticToolSkillCoversCLI in this same package
// — same filepath.Walk approach, same accumulate-then-grouped-error
// shape so failures show every offender in a single test run.
func TestNoLegacyAuthRefsInWorkflows(t *testing.T) {
	roots := []string{
		filepath.Join(repoRootFromCli, ".github", "workflows"),
		filepath.Join(repoRootFromCli, ".github", "actions"),
	}

	type hit struct {
		path string
		line int
		ref  string
	}
	var hits []hit

	for _, root := range roots {
		// A repo without one of these directories is fine — skip.
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if !(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for lineNo, line := range strings.Split(string(content), "\n") {
				for _, ref := range legacyRefs {
					if strings.Contains(line, ref) {
						hits = append(hits, hit{path: path, line: lineNo + 1, ref: ref})
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %q: %v", root, err)
		}
	}

	if len(hits) == 0 {
		return
	}

	// Sort for stable output — failures should look the same on every
	// run regardless of filesystem walk order.
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].path != hits[j].path {
			return hits[i].path < hits[j].path
		}
		if hits[i].line != hits[j].line {
			return hits[i].line < hits[j].line
		}
		return hits[i].ref < hits[j].ref
	})

	var msg strings.Builder
	msg.WriteString("\n")
	msg.WriteString("Legacy authentication identifier(s) re-introduced in CI configuration.\n")
	msg.WriteString("After Feature #622, no .github/workflows or .github/actions file may reference\n")
	msg.WriteString("the following identifiers — every CI step must consume a GitHub App\n")
	msg.WriteString("installation token minted by actions/create-github-app-token.\n\n")
	msg.WriteString("Forbidden identifiers: " + strings.Join(legacyRefs, ", ") + "\n\n")
	msg.WriteString("Offending occurrences:\n")
	for _, h := range hits {
		msg.WriteString("  " + h.path + ":" + itoa(h.line) + " — " + h.ref + "\n")
	}
	msg.WriteString("\nRemediation: replace each occurrence with steps.app-token.outputs.token\n")
	msg.WriteString("(see docs/github-app-setup.md and the existing job patterns in\n")
	msg.WriteString(".github/workflows/agentic-pipeline.yml).\n")
	t.Errorf("%s", msg.String())
}

// TestNoLegacyAuthRefsInWorkflows_NegativePath proves the matcher
// actually catches a forbidden identifier. Mirrors the negative-path
// test pattern in tool_skill_sync_test.go: synthesise an in-memory
// scenario containing a known-bad string and assert the same matcher
// reports it. Without this, a silent matcher regression (e.g. a typo
// in legacyRefs, a Contains-vs-Equal mistake) would let the guard pass
// vacuously.
func TestNoLegacyAuthRefsInWorkflows_NegativePath(t *testing.T) {
	// Build a synthetic line that contains one of the forbidden refs.
	// Run the same containment loop and confirm the hit is recorded.
	syntheticContent := "      token: ${{ secrets.GOOSE_AGENT_PAT }}\n"
	var found bool
	for _, line := range strings.Split(syntheticContent, "\n") {
		for _, ref := range legacyRefs {
			if strings.Contains(line, ref) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("matcher failed to flag a synthetic GOOSE_AGENT_PAT line; legacyRefs=%v", legacyRefs)
	}

	// Confirm a clean line is not flagged — guards against an
	// always-true matcher.
	cleanContent := "      token: ${{ steps.app-token.outputs.token }}\n"
	for _, line := range strings.Split(cleanContent, "\n") {
		for _, ref := range legacyRefs {
			if strings.Contains(line, ref) {
				t.Errorf("matcher flagged a clean line %q for ref %q — false positive", line, ref)
			}
		}
	}
}

// itoa converts a positive int to its decimal string form. Avoids the
// strconv import for a single call site in the failure message — the
// guard test is intentionally dependency-free.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(b[pos:])
}
