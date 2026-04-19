// Package mount implements the core framework mount logic.
// It clones the AI-Native Delivery Framework from a versioned tag of gh-agentic.
package mount

import (
	"io"
)

// FrameworkRepo is the default repository for framework clones.
const FrameworkRepo = "eddiecarpenter/gh-agentic"

// FrameworkRepoURL is the HTTPS clone URL for the framework repo.
const FrameworkRepoURL = "https://github.com/" + FrameworkRepo + ".git"

// CloneFunc clones a repository at a given tag into a destination directory.
// It must perform a shallow clone (--depth 1) and strip the .git/ directory.
type CloneFunc func(repoURL, tag, destDir string) error

// ConfirmFunc prompts the user for confirmation. Returns true if confirmed.
type ConfirmFunc func(prompt string) (bool, error)

// Deps holds injectable dependencies for mount operations.
// Tests can supply fakes; the production path fills in real defaults.
type Deps struct {
	// FetchReleases fetches all releases for a repo slug.
	FetchReleases FetchReleasesFunc

	// Clone performs the shallow git clone of the framework.
	Clone CloneFunc

	// Confirm prompts for user confirmation (version switch).
	Confirm ConfirmFunc

	// Stdout is the output writer for progress messages.
	Stdout io.Writer
}

// DefaultDeps returns production dependencies.
func DefaultDeps(w io.Writer) Deps {
	return Deps{
		FetchReleases: DefaultFetchReleases,
		Clone:         DefaultClone,
		Confirm:       nil, // Set by caller when needed.
		Stdout:        w,
	}
}
