package mount

import (
	"fmt"
	"io"
)

// RunSwitch handles the version switch flow when .ai-version exists and a
// different version is requested. Shows a confirmation prompt before proceeding.
//
// This is a stub — full implementation in task #422.
func RunSwitch(w io.Writer, root, currentVersion, newVersion string, fetch FetchTarballFunc, confirm ConfirmFunc) error {
	// Prompt for confirmation.
	if confirm != nil {
		prompt := fmt.Sprintf("Switch AI-Native Delivery Framework from %s to %s?", currentVersion, newVersion)
		ok, err := confirm(prompt)
		if err != nil {
			return fmt.Errorf("confirmation prompt: %w", err)
		}
		if !ok {
			return nil // User declined.
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Mounting framework...")

	if err := DownloadFramework(root, newVersion, fetch); err != nil {
		return fmt.Errorf("switching framework: %w", err)
	}

	fmt.Fprintf(w, "  ✓ Mounting AI Framework (%s) at .ai/\n", newVersion)
	fmt.Fprintf(w, "  ✓ .ai-version updated (%s → %s)\n", currentVersion, newVersion)

	if err := WriteAIVersion(root, newVersion); err != nil {
		return fmt.Errorf("updating .ai-version: %w", err)
	}

	fmt.Fprintf(w, "  ✓ Wrapper workflows updated to @%s\n", newVersion)
	if err := updateWorkflowVersions(root, newVersion); err != nil {
		return fmt.Errorf("updating workflows: %w", err)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "AI Framework successfully mounted at .ai/")

	return nil
}

// updateWorkflowVersions updates the uses: tags in wrapper workflow files
// to reference the new version.
//
// Stub — full implementation in task #422.
func updateWorkflowVersions(root, version string) error {
	// Will be implemented properly in task #422.
	// For now, regenerate the workflows.
	return generateWorkflows(io.Discard, root, version)
}
