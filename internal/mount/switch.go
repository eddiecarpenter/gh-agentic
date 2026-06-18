package mount

import (
	"fmt"
	"io"
)

// RunSwitch handles the version switch flow when a different framework
// version is requested. Shows a confirmation prompt before proceeding.
// Remounts the framework and updates wrapper workflow tags. The mounted
// version is tracked via .agents/.git metadata — no flat file is written.
func RunSwitch(w io.Writer, root, currentVersion, newVersion string, fetch CloneFunc, confirm ConfirmFunc) error {
	if confirm != nil {
		prompt := fmt.Sprintf("Switch AI-Native Delivery Framework from %s to %s?", currentVersion, newVersion)
		ok, err := confirm(prompt)
		if err != nil {
			return fmt.Errorf("confirmation prompt: %w", err)
		}
		if !ok {
			return nil
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Mounting framework...")

	if err := DownloadFramework(root, newVersion, fetch); err != nil {
		return fmt.Errorf("switching framework: %w", err)
	}
	fmt.Fprintf(w, "  ✓ Mounting AI Framework (%s) at .agents/\n", newVersion)
	fmt.Fprintf(w, "  ✓ Framework version updated (%s → %s)\n", currentVersion, newVersion)

	// Regenerate the wrapper workflows from the template rather than only
	// rewriting the @version tag. The wrapper is a framework-managed thin shell
	// fully determined by the template + version, so regenerating propagates
	// template content fixes (e.g. actions: read, explicit secrets) to existing
	// repos on upgrade — a surgical @version rewrite would leave stale content
	// behind, which is how a v3.0.0 CP could keep running a wrapper missing
	// actions: read even after `gh agentic upgrade`.
	if err := GenerateWorkflows(w, root, newVersion); err != nil {
		return fmt.Errorf("regenerating workflows: %w", err)
	}
	fmt.Fprintf(w, "  ✓ Wrapper workflows regenerated at @%s\n", newVersion)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "AI Framework successfully mounted at .agents/")

	return nil
}
