package mount

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// RunSwitch handles the version switch flow when .ai-version exists and a
// different version is requested. Shows a confirmation prompt before proceeding.
// Updates .ai-version, remounts the framework, and updates wrapper workflow tags.
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

	// Download and extract the new framework version.
	if err := DownloadFramework(root, newVersion, fetch); err != nil {
		return fmt.Errorf("switching framework: %w", err)
	}
	fmt.Fprintf(w, "  ✓ Mounting AI Framework (%s) at .ai/\n", newVersion)

	// Update .ai-version.
	if err := WriteAIVersion(root, newVersion); err != nil {
		return fmt.Errorf("updating .ai-version: %w", err)
	}
	fmt.Fprintf(w, "  ✓ .ai-version updated (%s → %s)\n", currentVersion, newVersion)

	// Update workflow version tags.
	if err := UpdateWorkflowVersions(root, newVersion); err != nil {
		return fmt.Errorf("updating workflows: %w", err)
	}
	fmt.Fprintf(w, "  ✓ Wrapper workflows updated to @%s\n", newVersion)

	// List updated workflow files.
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

// versionTagPattern matches @vX.Y.Z version tags in uses: lines for
// the gh-agentic reusable workflows.
var versionTagPattern = regexp.MustCompile(`(eddiecarpenter/gh-agentic/\.github/workflows/[^@]+)@(v[0-9]+\.[0-9]+\.[0-9]+[^\s]*)`)

// UpdateWorkflowVersions scans all .yml files in .github/workflows/ and
// replaces gh-agentic version tags with the new version. If no workflow
// files exist, this is a no-op.
func UpdateWorkflowVersions(root, newVersion string) error {
	workflowsDir := filepath.Join(root, ".github", "workflows")

	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No workflows directory — nothing to update.
		}
		return fmt.Errorf("reading workflows directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".yml" && filepath.Ext(name) != ".yaml" {
			continue
		}

		path := filepath.Join(workflowsDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}

		updated := versionTagPattern.ReplaceAllString(string(data), "${1}@"+newVersion)
		if updated != string(data) {
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", name, err)
			}
		}
	}

	return nil
}
