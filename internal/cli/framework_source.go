package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// refuseIfFrameworkSource returns an error if root is the gh-agentic
// framework source itself. Commands that do not apply on the framework
// source (mount, upgrade, init, project *) invoke this at the start of
// their RunE and propagate the error to halt before any side effect.
//
// Detection is a single lstat of .agents — cheap. See
// internal/project/source.go for the signal.
//
// The error message is the canonical refusal shape shared by every
// guarded command. It states what was detected, why the command cannot
// run, and which commands ARE supported on the framework source, so the
// user is not left guessing.
//
// When cmd is non-nil, the helper sets SilenceUsage on it so cobra does
// not dump the command's Help block before the refusal message —
// that would bury the actual reason the command was refused under 30
// lines of usage text. The flag is set only on refusal; normal argument
// errors still get the usage dump.
func refuseIfFrameworkSource(cmd *cobra.Command, root, commandName string) error {
	if !project.IsFrameworkSource(root) {
		return nil
	}
	if cmd != nil {
		cmd.SilenceUsage = true
	}
	return fmt.Errorf(
		"command refused: .agents is a symlink — this repo is the gh-agentic framework source, not a consumer\n"+
			"`gh agentic %s` does not apply here.\n"+
			"supported commands on the framework source: status, info, auth, check, repair",
		commandName,
	)
}
