package mount

import (
	"fmt"
	"io"
)

// RunRemount remounts the framework at the current version. No confirmation
// prompt is shown since the version is unchanged. This re-downloads and
// extracts the framework, ensuring a clean state.
func RunRemount(w io.Writer, root, version string, fetch CloneFunc) error {
	fmt.Fprintf(w, "Mounting AI Framework (%s) at .agents/...\n", version)

	if err := DownloadFramework(root, version, fetch); err != nil {
		return fmt.Errorf("remounting framework: %w", err)
	}

	fmt.Fprintln(w, "  ✓ Framework mounted at .agents/")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "AI Framework successfully mounted at .agents/")

	return nil
}
