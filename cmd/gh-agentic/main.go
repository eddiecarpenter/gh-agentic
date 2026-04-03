// Command gh-agentic is a GitHub CLI extension for managing agentic development environments.
// Install via: gh extension install eddiecarpenter/gh-agentic
// Upgrade via: gh extension upgrade agentic
package main

import (
	"fmt"
	"os"

	"github.com/eddiecarpenter/gh-agentic/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
