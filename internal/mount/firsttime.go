package mount

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// RunFirstTime orchestrates the first-time mount flow when no .agents/
// directory exists yet. It:
//  1. Installs the framework as a `.agents/` submodule
//  2. Generates CLAUDE.md, AGENTS.md, and wrapper workflows via ScaffoldProjectFiles
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
	fmt.Fprintf(w, "  ✓ Installing AI Framework (%s) at .agents/\n", version)
	if err := DownloadFramework(root, version, fetch); err != nil {
		return fmt.Errorf("installing framework: %w", err)
	}

	// Steps 2-4: Generate CLAUDE.md, AGENTS.md, and wrapper workflows.
	if err := ScaffoldProjectFiles(w, root, version); err != nil {
		return err
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "AI Framework successfully mounted at .agents/")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintln(w, "  1. Review and commit the generated files")
	fmt.Fprintln(w, "  2. Run 'gh agentic init' to join or create an agentic project")
	fmt.Fprintln(w, "  3. Run 'gh agentic check' to verify the full setup")

	return nil
}

// ScaffoldProjectFiles creates the standard agent instruction files and
// wrapper workflows for a domain repo. It is safe to call after
// DownloadFramework — each file is only written when it does not already
// exist (idempotent). project.Create and project.initFederated call this
// after mounting the framework so that init always produces the full set
// of required files regardless of which code path ran.
func ScaffoldProjectFiles(w io.Writer, root, version string) error {
	// CLAUDE.md — only if absent.
	claudePath := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		fmt.Fprintln(w, "  ✓ CLAUDE.md created")
		if err := os.WriteFile(claudePath, []byte(claudeMDTemplate), 0o644); err != nil {
			return fmt.Errorf("creating CLAUDE.md: %w", err)
		}
	}

	// AGENTS.md — only if absent.
	agentsPath := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		fmt.Fprintln(w, "  ✓ AGENTS.md created")
		if err := os.WriteFile(agentsPath, []byte(agentsMDTemplate), 0o644); err != nil {
			return fmt.Errorf("creating AGENTS.md: %w", err)
		}
	}

	// Wrapper workflows.
	fmt.Fprintln(w, "  ✓ Wrapper workflows created")
	if err := GenerateWorkflows(w, root, version); err != nil {
		return fmt.Errorf("creating workflows: %w", err)
	}

	return nil
}

// ScaffoldLocalRules creates a starter LOCALRULES.md in root when none
// exists. The file is pre-populated with the repo name, owner, and
// topology so the developer has a ready-to-edit file rather than a blank
// slate. It is safe to call multiple times — a no-op when LOCALRULES.md
// already exists.
func ScaffoldLocalRules(w io.Writer, root, repoName, owner, topology string) error {
	path := filepath.Join(root, "LOCALRULES.md")
	if _, err := os.Stat(path); err == nil {
		return nil // already exists — preserve the user's overrides
	}
	content := localrulesMDTemplate(repoName, owner, topology)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("creating LOCALRULES.md: %w", err)
	}
	fmt.Fprintln(w, "  ✓ LOCALRULES.md created")
	return nil
}

// GenerateWorkflows creates the wrapper workflow files in .github/workflows/.
// It is exported so that repair tooling can scaffold missing workflow files
// without going through the full first-time mount flow.
func GenerateWorkflows(w io.Writer, root, version string) error {
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
