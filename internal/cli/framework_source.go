package cli

import (
	"fmt"

	"github.com/eddiecarpenter/gh-agentic/internal/project"
)

// refuseIfFrameworkSource returns an error if root is the gh-agentic
// framework source itself. Commands that do not apply on the framework
// source (mount, upgrade, init, project *) invoke this at the start of
// their RunE and propagate the error to halt before any side effect.
//
// Detection is a single lstat of .ai — cheap. See
// internal/project/source.go for the signal.
//
// The error message is the canonical refusal shape shared by every
// guarded command. It states what was detected, why the command cannot
// run, and which commands ARE supported on the framework source, so the
// user is not left guessing.
func refuseIfFrameworkSource(root, commandName string) error {
	if !project.IsFrameworkSource(root) {
		return nil
	}
	return fmt.Errorf(
		"command refused: .ai is a symlink — this repo is the gh-agentic framework source, not a consumer\n"+
			"`gh agentic %s` does not apply here.\n"+
			"supported commands on the framework source: status, info, auth, check, repair",
		commandName,
	)
}
