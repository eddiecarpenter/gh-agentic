package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestStatusCommands_RejectJSONFlag is the direct AC-1 verification for
// feature #589: every status sub-command must respond to `--json` with
// cobra's `unknown flag: --json` error and a non-zero exit code. The
// `--json` envelope was retired in favour of the agent-oriented `--raw`
// shape; nothing in the CLI must accept the old flag any longer.
//
// Each table row drives the root command end-to-end so we exercise the
// real cobra parse layer rather than the handler — that is where the
// flag rejection actually happens.
func TestStatusCommands_RejectJSONFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "status pipeline", args: []string{"status", "pipeline", "--json"}},
		{name: "status requirements", args: []string{"status", "requirements", "--json"}},
		{name: "status features", args: []string{"status", "features", "--json"}},
		{name: "status requirement", args: []string{"status", "requirement", "457", "--json"}},
		{name: "status feature", args: []string{"status", "feature", "492", "--json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRootCmd("test", "test")
			buf := &bytes.Buffer{}
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs(tc.args)
			err := root.Execute()
			if err == nil {
				t.Fatalf("%s: expected unknown-flag error, got nil; output:\n%s", tc.name, buf.String())
			}
			if !strings.Contains(err.Error(), "unknown flag: --json") {
				t.Errorf("%s: expected 'unknown flag: --json' in error; got %q", tc.name, err.Error())
			}
		})
	}
}
