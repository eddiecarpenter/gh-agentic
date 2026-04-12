package mount

import (
	"fmt"
	"io"
)

// RunRemount remounts the framework at the current version. No confirmation
// prompt is shown since the version is unchanged.
//
// This is a stub — full implementation in task #422.
func RunRemount(w io.Writer, root, version string, fetch FetchTarballFunc) error {
	fmt.Fprintf(w, "Mounting AI Framework (%s) at .ai/...\n", version)

	if err := DownloadFramework(root, version, fetch); err != nil {
		return fmt.Errorf("remounting framework: %w", err)
	}

	fmt.Fprintln(w, "  ✓ Framework mounted at .ai/")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "AI Framework successfully mounted at .ai/\n")

	return nil
}
