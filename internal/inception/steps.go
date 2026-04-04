package inception

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// StepState carries values produced by earlier steps and consumed by later steps.
// It is allocated by RunSteps and passed (by pointer) to each step function.
type StepState struct {
	// RepoName is the full GitHub repo name (e.g. "charging-domain").
	RepoName string

	// ClonePath is the absolute local path to the cloned repository.
	ClonePath string

	// RepoURL is the HTTPS URL of the created repository.
	RepoURL string
}

// standardLabels are the 9 labels created in every agentic repo.
var standardLabels = []string{
	"requirement", "feature", "task", "backlog", "draft",
	"in-design", "in-development", "in-review", "done",
}

// stackFileName maps a stack name to its standards file basename.
var stackFileName = map[string]string{
	"Go":                 "go.md",
	"Java Quarkus":       "java-quarkus.md",
	"Java Spring Boot":   "java-spring.md",
	"TypeScript Node.js": "typescript.md",
	"Python":             "python.md",
	"Rust":               "rust.md",
}

// typeDirName maps a repo type to its parent directory name.
func typeDirName(repoType string) string {
	switch repoType {
	case "domain":
		return "domains"
	case "tool":
		return "tools"
	default:
		return "others"
	}
}

// --------------------------------------------------------------------------------------
// Step 1 — CreateRepo
// --------------------------------------------------------------------------------------

// CreateRepo creates the GitHub repository and clones it locally into the
// appropriate type directory (e.g. domains/charging-domain).
//
// run is injected so tests can substitute a fake implementation.
func CreateRepo(w io.Writer, cfg *InceptionConfig, state *StepState, env *EnvContext, run bootstrap.RunCommandFunc) error {
	name := FullRepoName(*cfg)
	state.RepoName = name
	state.RepoURL = fmt.Sprintf("https://github.com/%s/%s", cfg.Owner, name)

	typeDir := filepath.Join(env.AgenticRepoRoot, typeDirName(cfg.RepoType))
	state.ClonePath = filepath.Join(typeDir, name)

	// Ensure the type directory exists.
	if err := os.MkdirAll(typeDir, 0755); err != nil {
		return fmt.Errorf("creating type directory %s: %w", typeDir, err)
	}

	fullName := cfg.Owner + "/" + name

	// Create the repo.
	out, err := run("gh", "repo", "create", fullName, "--private")
	if err != nil {
		return fmt.Errorf("gh repo create: %w\n%s", err, strings.TrimSpace(out))
	}

	// Clone the repo.
	sshURL := fmt.Sprintf("git@github.com:%s.git", fullName)
	out, err = run("git", "clone", sshURL, state.ClonePath)
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, strings.TrimSpace(out))
	}

	return nil
}

// --------------------------------------------------------------------------------------
// Step 2 — ConfigureLabels
// --------------------------------------------------------------------------------------

// ConfigureLabels creates the standard labels in the new repo.
// Label creation failures are logged as warnings but do not fail the step.
//
// run is injected so tests can substitute a fake implementation.
func ConfigureLabels(w io.Writer, cfg *InceptionConfig, state *StepState, run bootstrap.RunCommandFunc) error {
	fullName := cfg.Owner + "/" + state.RepoName

	for _, label := range standardLabels {
		out, err := run("gh", "label", "create", label, "--repo", fullName, "--force")
		if err != nil {
			fmt.Fprintln(w, "  "+ui.RenderWarning("label "+label+": "+strings.TrimSpace(out)))
		}
	}

	return nil
}

// --------------------------------------------------------------------------------------
// Step 3 — ScaffoldStack
// --------------------------------------------------------------------------------------

// ScaffoldStack reads the Project Initialisation section from the stack standards file
// in the agentic repo and executes each bash code block in the cloned repo.
//
// run is injected so tests can substitute a fake implementation.
func ScaffoldStack(w io.Writer, cfg *InceptionConfig, state *StepState, env *EnvContext, run bootstrap.RunCommandFunc) error {
	filename, ok := stackFileName[cfg.Stack]
	if !ok {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Stack "+cfg.Stack+" has no initialisation template — skipping scaffold"))
		return nil
	}

	standardsPath := filepath.Join(env.AgenticRepoRoot, "base", "standards", filename)
	data, err := os.ReadFile(standardsPath)
	if err != nil {
		return fmt.Errorf("reading standards file %s: %w", standardsPath, err)
	}

	commands, err := extractInitCommands(string(data))
	if err != nil {
		return fmt.Errorf("parsing Project Initialisation section: %w", err)
	}

	for _, cmd := range commands {
		out, err := runInDir(run, state.ClonePath, "bash", "-c", cmd)
		if err != nil {
			return fmt.Errorf("scaffold command %q: %w\n%s", cmd, err, strings.TrimSpace(out))
		}
	}

	return nil
}

// extractInitCommands parses Markdown content and returns the shell commands found
// inside ```bash code blocks within the "## Project Initialisation" section only.
func extractInitCommands(content string) ([]string, error) {
	const sectionHeading = "## Project Initialisation"

	start := strings.Index(content, sectionHeading)
	if start == -1 {
		return nil, fmt.Errorf("section %q not found", sectionHeading)
	}

	rest := content[start+len(sectionHeading):]
	end := strings.Index(rest, "\n## ")
	if end == -1 {
		end = len(rest)
	}
	section := rest[:end]

	var commands []string
	scanner := bufio.NewScanner(strings.NewReader(section))
	inBlock := false
	var blockLines []string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "```bash" {
			inBlock = true
			blockLines = nil
			continue
		}
		if inBlock && trimmed == "```" {
			inBlock = false
			if len(blockLines) > 0 {
				commands = append(commands, strings.Join(blockLines, "\n"))
			}
			continue
		}
		if inBlock {
			blockLines = append(blockLines, line)
		}
	}

	return commands, nil
}

// --------------------------------------------------------------------------------------
// Step 4 — PopulateRepo
// --------------------------------------------------------------------------------------

// PopulateRepo writes CLAUDE.md, AGENTS.local.md, and README.md into the cloned repo,
// then commits and pushes.
//
// run is injected so tests can substitute a fake implementation.
func PopulateRepo(w io.Writer, cfg *InceptionConfig, state *StepState, env *EnvContext, run bootstrap.RunCommandFunc) error {
	// Write CLAUDE.md referencing the agentic repo's AGENTS.md.
	claudeMD := "# CLAUDE.md\n\n" +
		"This project uses AGENTS.md as the single source of truth for agent instructions.\n" +
		"All development rules, workflows, and session protocols are defined there.\n\n" +
		"@base/AGENTS.md\n" +
		"@AGENTS.local.md\n"
	if err := os.WriteFile(filepath.Join(state.ClonePath, "CLAUDE.md"), []byte(claudeMD), 0644); err != nil {
		return fmt.Errorf("writing CLAUDE.md: %w", err)
	}

	// Write AGENTS.local.md.
	agentsLocal := fmt.Sprintf(
		"# AGENTS.local.md — Local Overrides\n\n"+
			"This file contains project-specific rules and overrides that extend or\n"+
			"supersede the global protocol defined in `base/AGENTS.md`.\n\n"+
			"This file is never overwritten by a template sync.\n\n---\n\n"+
			"## Template Source\n\nTemplate: eddiecarpenter/agentic-development\n\n"+
			"## Project\n\n"+
			"- **Name:** %s\n"+
			"- **Type:** %s\n"+
			"- **Stack:** %s\n"+
			"- **Description:** %s\n\n"+
			"## Repo\n\n"+
			"- **GitHub:** %s\n"+
			"- **Owner:** %s\n",
		state.RepoName, cfg.RepoType, cfg.Stack, cfg.Description, state.RepoURL, cfg.Owner)
	if err := os.WriteFile(filepath.Join(state.ClonePath, "AGENTS.local.md"), []byte(agentsLocal), 0644); err != nil {
		return fmt.Errorf("writing AGENTS.local.md: %w", err)
	}

	// Write README.md.
	readmeMD := fmt.Sprintf(
		"# %s\n\n%s\n\n## Setup\n\n"+
			"See `docs/PROJECT_BRIEF.md` for project context.\n\n"+
			"## Agent sessions\n\n"+
			"This repo uses the [agentic development framework](https://github.com/eddiecarpenter/agentic-development).\n"+
			"See `base/AGENTS.md` and `AGENTS.local.md` for session protocols.\n",
		state.RepoName, cfg.Description)
	if err := os.WriteFile(filepath.Join(state.ClonePath, "README.md"), []byte(readmeMD), 0644); err != nil {
		return fmt.Errorf("writing README.md: %w", err)
	}

	// Stage, commit, and push.
	out, err := runInDir(run, state.ClonePath, "git", "add", "-A")
	if err != nil {
		return fmt.Errorf("git add: %w\n%s", err, strings.TrimSpace(out))
	}

	commitMsg := "chore: bootstrap " + state.RepoName
	out, err = runInDir(run, state.ClonePath, "git", "commit", "-m", commitMsg)
	if err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(out))
	}

	out, err = runInDir(run, state.ClonePath, "git", "push", "origin", "main")
	if err != nil {
		return fmt.Errorf("git push: %w\n%s", err, strings.TrimSpace(out))
	}

	return nil
}

// --------------------------------------------------------------------------------------
// Step 5 — RegisterInREPOS
// --------------------------------------------------------------------------------------

// RegisterInREPOS appends a new repo entry to REPOS.md in the agentic repo root
// and commits the change.
//
// run is injected so tests can substitute a fake implementation.
func RegisterInREPOS(w io.Writer, cfg *InceptionConfig, state *StepState, env *EnvContext, run bootstrap.RunCommandFunc) error {
	reposPath := filepath.Join(env.AgenticRepoRoot, "REPOS.md")

	existing, err := os.ReadFile(reposPath)
	if err != nil {
		return fmt.Errorf("reading REPOS.md: %w", err)
	}

	entry := fmt.Sprintf("\n## %s\n\n"+
		"- **Repo:** git@github.com:%s/%s.git\n"+
		"- **Stack:** %s\n"+
		"- **Type:** %s\n"+
		"- **Status:** active\n"+
		"- **Description:** %s\n",
		state.RepoName, cfg.Owner, state.RepoName, cfg.Stack, cfg.RepoType, cfg.Description)

	updated := string(existing) + entry
	if err := os.WriteFile(reposPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("writing REPOS.md: %w", err)
	}

	// Stage and commit in the agentic repo.
	out, err := runInDir(run, env.AgenticRepoRoot, "git", "add", "REPOS.md")
	if err != nil {
		return fmt.Errorf("git add REPOS.md: %w\n%s", err, strings.TrimSpace(out))
	}

	commitMsg := "chore: register " + state.RepoName + " in REPOS.md"
	out, err = runInDir(run, env.AgenticRepoRoot, "git", "commit", "-m", commitMsg)
	if err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(out))
	}

	return nil
}

// --------------------------------------------------------------------------------------
// Step 6 — PrintSummary
// --------------------------------------------------------------------------------------

// PrintSummary renders the final "Inception complete" box with repo URL,
// clone path, and next steps.
func PrintSummary(w io.Writer, cfg *InceptionConfig, state *StepState) {
	fmt.Fprintln(w)

	successBold := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorSuccess))
	fmt.Fprintln(w, successBold.Render("  ✔ Inception complete"))
	fmt.Fprintln(w)

	content := fmt.Sprintf(
		"  %s  %s\n  %s  %s\n  %s  %s",
		ui.Muted.Render("Repo  "), ui.URL.Render(state.RepoURL),
		ui.Muted.Render("Clone "), ui.Value.Render(state.ClonePath),
		ui.Muted.Render("Type  "), ui.Value.Render(cfg.RepoType),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorSuccess)).
		Width(56).
		Padding(0, 1)

	fmt.Fprintln(w, box.Render(content))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  "+ui.Muted.Render("Next: cd into the repo and start a Requirements Session (Phase 1)."))
	fmt.Fprintln(w)
}

// --------------------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------------------

// runInDir wraps RunCommandFunc so that the command runs with the given working directory.
func runInDir(run bootstrap.RunCommandFunc, dir string, name string, args ...string) (string, error) {
	quotedDir := "'" + strings.ReplaceAll(dir, "'", "'\\''") + "'"
	inner := "cd " + quotedDir + " && " + shellJoin(name, args...)
	return run("bash", "-c", inner)
}

// shellJoin single-quotes each token and joins them with spaces.
func shellJoin(name string, args ...string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, shellQuote(name))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

// shellQuote wraps s in single quotes and escapes any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
