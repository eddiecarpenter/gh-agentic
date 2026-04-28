package mount

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// RunFirstTime orchestrates the first-time mount flow when no .ai/
// directory exists yet. It:
//  1. Installs the framework as a `.ai/` submodule
//  2. Generates CLAUDE.md
//  3. Generates AGENTS.md with bootstrap rule
//  4. Generates wrapper workflows in .github/workflows/
//
// The mounted version is recorded by the submodule's gitlink in the
// parent repo's index. Callers that need to broadcast the version
// (federated CP, single CP) write AGENTIC_FRAMEWORK_VERSION through the
// canonical project.SetRepoVariable path.
//
// No confirmation prompt is shown for first-time mount.
func RunFirstTime(w io.Writer, root, version string, fetch CloneFunc) error {
	fmt.Fprintln(w, "Initialising AI-Native Delivery Framework...")

	// Step 1: Install framework as a submodule.
	fmt.Fprintf(w, "  ✓ Installing AI Framework (%s) at .ai/\n", version)
	if err := DownloadFramework(root, version, fetch); err != nil {
		return fmt.Errorf("installing framework: %w", err)
	}

	// Step 2: Generate CLAUDE.md (only if it doesn't exist).
	claudePath := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		fmt.Fprintln(w, "  ✓ CLAUDE.md created")
		if err := os.WriteFile(claudePath, []byte(claudeMDTemplate), 0o644); err != nil {
			return fmt.Errorf("creating CLAUDE.md: %w", err)
		}
	}

	// Step 3: Generate AGENTS.md (only if it doesn't exist).
	agentsPath := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		fmt.Fprintln(w, "  ✓ AGENTS.md created")
		if err := os.WriteFile(agentsPath, []byte(agentsMDTemplate), 0o644); err != nil {
			return fmt.Errorf("creating AGENTS.md: %w", err)
		}
	}

	// Step 4: Generate wrapper workflows.
	fmt.Fprintln(w, "  ✓ Wrapper workflows created")
	if err := generateWorkflows(w, root, version); err != nil {
		return fmt.Errorf("creating workflows: %w", err)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "AI Framework successfully mounted at .ai/")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "  1. Review and commit the generated files")
	fmt.Fprintln(w, "  2. Run 'gh agentic init' to join or create an agentic project")
	fmt.Fprintln(w, "  3. Run 'gh agentic check' to verify the full setup")

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
