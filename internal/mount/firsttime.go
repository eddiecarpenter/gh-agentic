package mount

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// RunFirstTime orchestrates the first-time mount flow when no .ai-version
// file exists. It:
//  1. Downloads and extracts the framework to .ai/
//  2. Creates .ai-version with the specified version
//  3. Adds .ai/ to .gitignore
//  4. Generates CLAUDE.md
//  5. Generates AGENTS.md with bootstrap rule
//  6. Generates wrapper workflows in .github/workflows/
//
// No confirmation prompt is shown for first-time mount.
func RunFirstTime(w io.Writer, root, version string, fetch FetchTarballFunc) error {
	fmt.Fprintln(w, "Initialising AI-Native Delivery Framework...")

	// Step 1: Download framework.
	fmt.Fprintf(w, "  ✓ Mounting AI Framework (%s) at .ai/\n", version)
	if err := DownloadFramework(root, version, fetch); err != nil {
		return fmt.Errorf("mounting framework: %w", err)
	}

	// Step 2: Create .ai-version.
	fmt.Fprintf(w, "  ✓ .ai-version created (%s)\n", version)
	if err := WriteAIVersion(root, version); err != nil {
		return fmt.Errorf("creating .ai-version: %w", err)
	}

	// Step 3: Add .ai/ to .gitignore.
	fmt.Fprintln(w, "  ✓ .ai/ added to .gitignore")
	if err := EnsureGitignore(root); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}

	// Step 4: Generate CLAUDE.md (only if it doesn't exist).
	claudePath := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		fmt.Fprintln(w, "  ✓ CLAUDE.md created")
		if err := os.WriteFile(claudePath, []byte(claudeMDTemplate), 0o644); err != nil {
			return fmt.Errorf("creating CLAUDE.md: %w", err)
		}
	}

	// Step 5: Generate AGENTS.md (only if it doesn't exist).
	agentsPath := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		fmt.Fprintln(w, "  ✓ AGENTS.md created")
		if err := os.WriteFile(agentsPath, []byte(agentsMDTemplate), 0o644); err != nil {
			return fmt.Errorf("creating AGENTS.md: %w", err)
		}
	}

	// Step 6: Generate wrapper workflows.
	fmt.Fprintln(w, "  ✓ Wrapper workflows created")
	if err := generateWorkflows(w, root, version); err != nil {
		return fmt.Errorf("creating workflows: %w", err)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "AI Framework successfully mounted at .ai/")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "  1. Review and commit the generated files")
	fmt.Fprintln(w, "  2. Configure GOOSE_AGENT_PAT and CLAUDE_CREDENTIALS_JSON as secrets")
	fmt.Fprintln(w, "  3. Run 'gh agentic -v2 doctor' to verify")

	return nil
}

// generateWorkflows creates the wrapper workflow files in .github/workflows/.
func generateWorkflows(w io.Writer, root, version string) error {
	workflowsDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0o755); err != nil {
		return fmt.Errorf("creating workflows directory: %w", err)
	}

	workflows := map[string]string{
		"agentic-pipeline.yml": workflowTemplate(version),
		"release.yml":          releaseWorkflowTemplate(version),
	}

	for name, content := range workflows {
		path := filepath.Join(workflowsDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		fmt.Fprintf(w, "\n    * .github/workflows/%s\n", name)
	}

	return nil
}
