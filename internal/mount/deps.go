// Package mount implements the core framework mount logic for v2.
// It downloads and extracts the AI-Native Delivery Framework from a
// versioned release of gh-agentic.
package mount

import (
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/sync"
	"github.com/eddiecarpenter/gh-agentic/internal/tarball"
)

// FrameworkRepo is the default repository for framework releases.
const FrameworkRepo = "eddiecarpenter/gh-agentic"

// FrameworkPrefixes lists the path prefixes within the release tarball that
// constitute the framework. Only these paths are extracted to .ai/.
var FrameworkPrefixes = []string{
	"RULEBOOK.md",
	"skills/",
	"recipes/",
	"standards/",
	"concepts/",
}

// FetchReleasesFunc fetches all releases for a repo.
// Reuses the sync package type for consistency.
type FetchReleasesFunc = sync.FetchReleasesFunc

// FetchTarballFunc downloads a tarball for the given repo and version.
// Reuses the tarball package type.
type FetchTarballFunc = tarball.FetchFunc

// ConfirmFunc prompts the user for confirmation. Returns true if confirmed.
type ConfirmFunc func(prompt string) (bool, error)

// Deps holds injectable dependencies for mount operations.
// Tests can supply fakes; the production path fills in real defaults.
type Deps struct {
	// FetchReleases fetches all releases for a repo slug.
	FetchReleases FetchReleasesFunc

	// FetchTarball downloads a tarball for a given repo and version.
	FetchTarball FetchTarballFunc

	// Confirm prompts for user confirmation (version switch).
	Confirm ConfirmFunc

	// Stdout is the output writer for progress messages.
	Stdout io.Writer
}

// DefaultDeps returns production dependencies.
func DefaultDeps(w io.Writer) Deps {
	return Deps{
		FetchReleases: sync.DefaultFetchReleases,
		FetchTarball:  tarball.DefaultFetch,
		Confirm:       nil, // Set by caller when needed.
		Stdout:        w,
	}
}
