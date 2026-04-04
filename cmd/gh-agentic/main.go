// Command gh-agentic is a GitHub CLI extension for managing agentic software delivery environments.
// Install via: gh extension install eddiecarpenter/gh-agentic
// Upgrade via: gh extension upgrade agentic
package main

import (
	"fmt"
	"os"

	"github.com/eddiecarpenter/gh-agentic/internal/cli"
)

// version is set at build time by GoReleaser via ldflags.
// Local dev builds report "dev".
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		if err != cli.ErrSilent {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
