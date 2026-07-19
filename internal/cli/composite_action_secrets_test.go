package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// secretsContextRe matches a GitHub Actions expression that references the
// `secrets` context, e.g. ${{ secrets.PROJECT_PAT }} (with any surrounding
// whitespace). Composite actions CANNOT access the secrets context — the
// template validator rejects the whole action at load time with
// "Unrecognized named-value: 'secrets'", which fails every job that uses the
// action. Secrets must be passed into a composite action as `inputs` and
// referenced with ${{ inputs.* }} instead.
var secretsContextRe = regexp.MustCompile(`\$\{\{\s*secrets\.`)

// TestCompositeActionsDoNotReferenceSecrets is the build-time guard for the
// v3.1.0 regression (#912): a `${{ secrets.PROJECT_PAT }}` literal embedded in
// a `run:` echo string inside setup-goose-env's composite action was evaluated
// by the template parser at load time, and since composite actions have no
// secrets context the action failed to load — breaking every pipeline stage
// (all of them call setup-goose-env).
//
// It walks .github/actions/**/action.yml relative to the repo root and fails —
// listing every offender file:line — if any composite action references the
// secrets context anywhere (expression position OR inside a run string; both
// are parsed the same way and both fail).
func TestCompositeActionsDoNotReferenceSecrets(t *testing.T) {
	actionsDir := filepath.Join(repoRootFromCli, ".github", "actions")

	type hit struct {
		path string
		line int
		text string
	}
	var hits []hit

	err := filepath.WalkDir(actionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "action.yml" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for lineNo, line := range strings.Split(string(content), "\n") {
			if secretsContextRe.MatchString(line) {
				hits = append(hits, hit{path: path, line: lineNo + 1, text: strings.TrimSpace(line)})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %q: %v", actionsDir, err)
	}

	if len(hits) == 0 {
		return
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].path != hits[j].path {
			return hits[i].path < hits[j].path
		}
		return hits[i].line < hits[j].line
	})

	var msg strings.Builder
	msg.WriteString("\n")
	msg.WriteString("Composite action(s) reference the `secrets` context.\n\n")
	msg.WriteString("Composite actions (.github/actions/**/action.yml) have NO secrets context.\n")
	msg.WriteString("A ${{ secrets.* }} expression — even inside a run: string, which the\n")
	msg.WriteString("template parser evaluates the same way — makes the action fail to load\n")
	msg.WriteString("with \"Unrecognized named-value: 'secrets'\", breaking every job that uses it.\n\n")
	msg.WriteString("Pass the secret in as an `input` and reference ${{ inputs.* }} instead;\n")
	msg.WriteString("wire the secret at the workflow call site.\n\n")
	msg.WriteString("Offending occurrences:\n")
	for _, h := range hits {
		msg.WriteString("  " + h.path + ":" + itoa(h.line) + " — " + h.text + "\n")
	}
	t.Errorf("%s", msg.String())
}
