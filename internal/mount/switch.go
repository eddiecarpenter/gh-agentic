package mount

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// RunSwitch handles the version switch flow when .ai-version exists and a
// different version is requested. Shows a confirmation prompt before proceeding.
// Updates .ai-version, remounts the framework, and updates wrapper workflow tags.
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
	fmt.Fprintf(w, "  ✓ Mounting AI Framework (%s) at .ai/\n", newVersion)

	if err := WriteAIVersion(root, newVersion); err != nil {
		return fmt.Errorf("updating .ai-version: %w", err)
	}
	fmt.Fprintf(w, "  ✓ .ai-version updated (%s → %s)\n", currentVersion, newVersion)

	if err := UpdateWorkflowVersions(root, newVersion); err != nil {
		return fmt.Errorf("updating workflows: %w", err)
	}
	fmt.Fprintf(w, "  ✓ Wrapper workflows updated to @%s\n", newVersion)

	workflowsDir := filepath.Join(root, ".github", "workflows")
	entries, _ := os.ReadDir(workflowsDir)
	for _, e := range entries {
		if !e.IsDir() {
			fmt.Fprintf(w, "\n    * .github/workflows/%s updated\n", e.Name())
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "AI Framework successfully mounted at .ai/")

	return nil
}
