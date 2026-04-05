// Package bootstrap implements the business logic for gh agentic bootstrap.
package bootstrap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// StepState carries values produced by earlier steps and consumed by later steps.
// It is allocated by RunSteps and passed (by pointer) to each step function.
type StepState struct {
	// RepoName is the created repository name (e.g. "my-project" or "my-project-agentic").
	RepoName string

	// ClonePath is the absolute local path to the cloned repository.
	ClonePath string

	// RepoURL is the HTTPS URL of the created repository.
	RepoURL string

	// RepoNodeID is the GitHub node ID (global relay ID) of the repository.
	// Used in Step 8 to link the project to the repo via GraphQL.
	RepoNodeID string

	// ProjectURL is the URL of the created GitHub Project.
	ProjectURL string

	// ProjectNodeID is the GitHub node ID (global relay ID) of the created project.
	// Used for GraphQL mutations such as configuring status columns.
	ProjectNodeID string
}

// repoName derives the repository name from the config.
// Single: <project-name>, Federated: <project-name>-agentic.
func repoName(cfg BootstrapConfig) string {
	if cfg.Topology == "Federated" {
		return cfg.ProjectName + "-agentic"
	}
	return cfg.ProjectName
}

// --------------------------------------------------------------------------------------
// Step 3 — CreateRepo
// --------------------------------------------------------------------------------------

// CreateRepo creates the GitHub repository from the agentic-development template,
// clones it locally, and populates state with the repo name, clone path, URL, and node ID.
//
// workDir is the parent directory into which the repo will be cloned.
// run is injected so tests can substitute a fake implementation without spawning real processes.
func CreateRepo(w io.Writer, cfg BootstrapConfig, state *StepState, workDir string, run RunCommandFunc) error {
	name := repoName(cfg)
	state.RepoName = name
	state.ClonePath = filepath.Join(workDir, name)
	state.RepoURL = fmt.Sprintf("https://github.com/%s/%s", cfg.Owner, name)

	fullName := cfg.Owner + "/" + name

	// Create the repo from the template.
	out, err := run("gh", "repo", "create", fullName,
		"--template", "eddiecarpenter/agentic-development",
		"--private")
	if err != nil {
		return fmt.Errorf("gh repo create: %w\n%s", err, strings.TrimSpace(out))
	}

	// Clone the repo.
	sshURL := fmt.Sprintf("git@github.com:%s.git", fullName)
	out, err = run("git", "clone", sshURL, state.ClonePath)
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, strings.TrimSpace(out))
	}

	// Fetch the repo node ID via REST API (best-effort).
	client, err := api.DefaultRESTClient()
	if err != nil {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Could not create GitHub API client: "+err.Error()))
		return nil
	}

	var repoResp struct {
		NodeID string `json:"node_id"`
	}
	if err := client.Get(fmt.Sprintf("repos/%s", fullName), &repoResp); err != nil {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Could not fetch repo node ID: "+err.Error()))
		return nil
	}
	state.RepoNodeID = repoResp.NodeID
	return nil
}

// --------------------------------------------------------------------------------------
// Step 4 — RemoveTemplateFiles
// --------------------------------------------------------------------------------------

// RemoveTemplateFiles deletes bootstrap.sh and bootstrap.sh.md5 from the cloned repo
// and commits the removal. If either file is absent it logs a warning and continues.
//
// run is injected so tests can substitute a fake implementation.
func RemoveTemplateFiles(w io.Writer, state *StepState, run RunCommandFunc) error {
	files := []string{"bootstrap.sh", "bootstrap.sh.md5"}

	// Determine which files actually exist.
	var toRemove []string
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(state.ClonePath, f)); err == nil {
			toRemove = append(toRemove, f)
		} else {
			fmt.Fprintln(w, "  "+ui.Muted.Render("· "+f+" not present — skipping"))
		}
	}

	if len(toRemove) == 0 {
		// Nothing to remove — template was already clean.
		return nil
	}

	rmArgs := append([]string{"rm"}, toRemove...)
	out, err := runInDir(run, state.ClonePath, "git", rmArgs...)
	if err != nil {
		return fmt.Errorf("git rm: %w\n%s", err, strings.TrimSpace(out))
	}

	out, err = runInDir(run, state.ClonePath, "git", "commit", "-m", "chore: remove template bootstrap files")
	if err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(out))
	}

	return nil
}

// --------------------------------------------------------------------------------------
// Step 5 — ScaffoldStack
// --------------------------------------------------------------------------------------

// stackFileName maps a stack name to its standards file basename.
var stackFileName = map[string]string{
	"Go":                 "go.md",
	"Java Quarkus":       "java-quarkus.md",
	"Java Spring Boot":   "java-spring.md",
	"TypeScript Node.js": "typescript.md",
	"Python":             "python.md",
	"Rust":               "rust.md",
}

// ScaffoldStack reads the Project Initialisation section from the stack standards file
// in the cloned repo and executes each bash code block sequentially.
//
// run is injected so tests can substitute a fake implementation.
func ScaffoldStack(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
	filename, ok := stackFileName[cfg.Stack]
	if !ok {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Stack "+cfg.Stack+" has no initialisation template — skipping scaffold"))
		return nil
	}

	standardsPath := filepath.Join(state.ClonePath, "base", "standards", filename)
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

	// Find the start of the section.
	start := strings.Index(content, sectionHeading)
	if start == -1 {
		return nil, fmt.Errorf("section %q not found", sectionHeading)
	}

	// Find the end of the section (next ## heading or EOF).
	rest := content[start+len(sectionHeading):]
	end := strings.Index(rest, "\n## ")
	if end == -1 {
		end = len(rest)
	}
	section := rest[:end]

	// Extract bash code blocks.
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
// Step 6 — ConfigureRepo
// --------------------------------------------------------------------------------------

// standardLabels are the 9 labels created in every agentic repo.
var standardLabels = []string{
	"requirement", "feature", "task", "backlog", "draft",
	"in-design", "in-development", "in-review", "done",
}

// ConfigureRepo creates the standard labels in the new repo.
// Branch protection is skipped (free plan limitation).
// Label creation failures are logged as warnings but do not fail the step.
//
// run is injected so tests can substitute a fake implementation.
func ConfigureRepo(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
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
// Step 7 — PopulateRepo
// --------------------------------------------------------------------------------------

// PopulateRepo writes REPOS.md, AGENTS.local.md, and README.md into the cloned repo,
// optionally scaffolds Antora, then commits and pushes.
//
// run is injected so tests can substitute a fake implementation.
func PopulateRepo(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
	// Write REPOS.md.
	reposMD := fmt.Sprintf("# REPOS.md\n\n## %s\n\n- **Repo:** git@github.com:%s/%s.git\n- **Stack:** %s\n- **Type:** agentic\n- **Status:** active\n- **Description:** %s\n",
		state.RepoName, cfg.Owner, state.RepoName, cfg.Stack, cfg.Description)
	if err := os.WriteFile(filepath.Join(state.ClonePath, "REPOS.md"), []byte(reposMD), 0644); err != nil {
		return fmt.Errorf("writing REPOS.md: %w", err)
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
			"- **Topology:** %s\n"+
			"- **Stack:** %s\n"+
			"- **Description:** %s\n\n"+
			"## Repo\n\n"+
			"- **GitHub:** %s\n"+
			"- **Owner:** %s\n",
		cfg.ProjectName, cfg.Topology, cfg.Stack, cfg.Description, state.RepoURL, cfg.Owner)
	if err := os.WriteFile(filepath.Join(state.ClonePath, "AGENTS.local.md"), []byte(agentsLocal), 0644); err != nil {
		return fmt.Errorf("writing AGENTS.local.md: %w", err)
	}

	// Write README.md.
	readmeMD := fmt.Sprintf(
		"# %s\n\n%s\n\n## Setup\n\n"+
			"See [docs/PROJECT_BRIEF.md](docs/PROJECT_BRIEF.md) for project context.\n\n"+
			"## Agent sessions\n\n"+
			"This repo uses the [agentic development framework](https://github.com/eddiecarpenter/agentic-development).\n"+
			"See `base/AGENTS.md` and `AGENTS.local.md` for session protocols.\n",
		cfg.ProjectName, cfg.Description)
	if err := os.WriteFile(filepath.Join(state.ClonePath, "README.md"), []byte(readmeMD), 0644); err != nil {
		return fmt.Errorf("writing README.md: %w", err)
	}

	// Write AGENT_USER if configured.
	if cfg.AgentUser != "" {
		if err := os.WriteFile(filepath.Join(state.ClonePath, "AGENT_USER"), []byte(cfg.AgentUser+"\n"), 0644); err != nil {
			return fmt.Errorf("writing AGENT_USER: %w", err)
		}
	}

	// Scaffold Antora if requested.
	if cfg.Antora {
		if err := scaffoldAntora(state.ClonePath, cfg.ProjectName); err != nil {
			return fmt.Errorf("scaffolding Antora: %w", err)
		}
	}

	// Stage and commit.
	out, err := runInDir(run, state.ClonePath, "git", "add", "-A")
	if err != nil {
		return fmt.Errorf("git add: %w\n%s", err, strings.TrimSpace(out))
	}

	commitMsg := "chore: bootstrap " + cfg.ProjectName
	out, err = runInDir(run, state.ClonePath, "git", "commit", "-m", commitMsg)
	if err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(out))
	}

	// Push.
	out, err = runInDir(run, state.ClonePath, "git", "push", "origin", "main")
	if err != nil {
		return fmt.Errorf("git push: %w\n%s", err, strings.TrimSpace(out))
	}

	return nil
}

// scaffoldAntora creates the minimal Antora directory structure and playbook.
func scaffoldAntora(clonePath, projectName string) error {
	dirs := []string{
		filepath.Join(clonePath, "docs", "modules", "ROOT", "pages"),
		filepath.Join(clonePath, "docs", "modules", "ROOT", "nav"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating dir %s: %w", d, err)
		}
	}

	playbook := fmt.Sprintf(`site:
  title: %s
  url: https://example.com

content:
  sources:
    - url: .
      branches: HEAD
      start_path: docs

ui:
  bundle:
    url: https://gitlab.com/antora/antora-ui-default/-/jobs/artifacts/HEAD/raw/build/ui-bundle.zip?job=bundle-stable
    snapshot: true
`, projectName)

	if err := os.WriteFile(filepath.Join(clonePath, "antora-playbook.yml"), []byte(playbook), 0644); err != nil {
		return fmt.Errorf("writing antora-playbook.yml: %w", err)
	}

	antoraMeta := fmt.Sprintf("name: %s\ntitle: %s\nversion: ~\nnav:\n  - modules/ROOT/nav/nav.adoc\n",
		projectName, projectName)
	if err := os.WriteFile(filepath.Join(clonePath, "docs", "antora.yml"), []byte(antoraMeta), 0644); err != nil {
		return fmt.Errorf("writing docs/antora.yml: %w", err)
	}

	indexPage := fmt.Sprintf("= %s\n\nWelcome to the %s documentation.\n", projectName, projectName)
	if err := os.WriteFile(filepath.Join(clonePath, "docs", "modules", "ROOT", "pages", "index.adoc"),
		[]byte(indexPage), 0644); err != nil {
		return fmt.Errorf("writing index.adoc: %w", err)
	}

	return nil
}

// --------------------------------------------------------------------------------------
// Step 8 — CreateProject
// --------------------------------------------------------------------------------------

// GraphQLDoFunc is a function that executes a GraphQL query or mutation.
// Injected so tests can substitute a fake implementation without real gh auth.
type GraphQLDoFunc func(query string, variables map[string]interface{}, response interface{}) error

// DefaultGraphQLDo returns a GraphQLDoFunc backed by the go-gh/v2 GraphQL client.
func DefaultGraphQLDo() (GraphQLDoFunc, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}
	return func(query string, variables map[string]interface{}, response interface{}) error {
		return client.Do(query, variables, response)
	}, nil
}

// projectCreateResponse is the JSON shape returned by gh project create --format json.
type projectCreateResponse struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// CreateProject creates a GitHub Project, stores its URL in state, and links it
// to the repository via GraphQL. The link step is best-effort — failure is logged
// as a warning, not returned as an error.
//
// run is injected for the gh CLI call. graphqlDo is injected for GraphQL calls.
func CreateProject(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc, graphqlDo GraphQLDoFunc) error {
	// Create the project.
	out, err := run("gh", "project", "create", "--owner", cfg.Owner, "--title", cfg.ProjectName, "--format", "json")
	if err != nil {
		return fmt.Errorf("gh project create: %w\n%s", err, strings.TrimSpace(out))
	}

	// Parse JSON response.
	var projectResp projectCreateResponse
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &projectResp); jsonErr != nil {
		// Fall back: try extracting a URL from plain-text output.
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "github.com") {
				state.ProjectURL = strings.TrimSpace(line)
				break
			}
		}
	} else {
		state.ProjectURL = projectResp.URL
	}

	if graphqlDo == nil || projectResp.Number == 0 || state.RepoNodeID == "" {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Skipping project–repo link (missing IDs)"))
		setProjectVariable(w, cfg, state, run)
		return nil
	}

	// Fetch the project node ID — try personal account first, then org.
	projectNodeID := fetchProjectNodeID(graphqlDo, cfg.Owner, projectResp.Number)
	if projectNodeID == "" {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Could not resolve project node ID — skipping link"))
		setProjectVariable(w, cfg, state, run)
		return nil
	}
	state.ProjectNodeID = projectNodeID

	// Link the project to the repository.
	linkMutation := `mutation($projectId: ID!, $repositoryId: ID!) {
		linkProjectV2ToRepository(input: { projectId: $projectId, repositoryId: $repositoryId }) {
			repository { name }
		}
	}`
	var linkResp interface{}
	if err := graphqlDo(linkMutation, map[string]interface{}{
		"projectId":    projectNodeID,
		"repositoryId": state.RepoNodeID,
	}, &linkResp); err != nil {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Project–repo link failed (non-fatal): "+err.Error()))
	}

	setProjectVariable(w, cfg, state, run)
	return nil
}

// setProjectVariable stores the project node ID as a repository variable via gh CLI.
// This is best-effort — failure is logged as a warning, not returned as an error.
func setProjectVariable(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) {
	if state.ProjectNodeID == "" {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Skipping AGENTIC_PROJECT_ID variable (no project node ID)"))
		return
	}

	fullName := cfg.Owner + "/" + state.RepoName
	out, err := run("gh", "variable", "set", "AGENTIC_PROJECT_ID",
		"--body", state.ProjectNodeID, "--repo", fullName)
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not set AGENTIC_PROJECT_ID variable: "+strings.TrimSpace(out)))
		return
	}
}

// fetchProjectNodeID tries to resolve the GraphQL node ID for a project by number,
// checking personal accounts and organisations.
func fetchProjectNodeID(graphqlDo GraphQLDoFunc, owner string, number int) string {
	vars := map[string]interface{}{
		"login":  owner,
		"number": number,
	}

	// Try user first.
	userQuery := `query($login: String!, $number: Int!) {
		user(login: $login) { projectV2(number: $number) { id } }
	}`
	var userResp struct {
		User struct {
			ProjectV2 struct {
				ID string `json:"id"`
			} `json:"projectV2"`
		} `json:"user"`
	}
	if err := graphqlDo(userQuery, vars, &userResp); err == nil && userResp.User.ProjectV2.ID != "" {
		return userResp.User.ProjectV2.ID
	}

	// Try as org.
	orgQuery := `query($login: String!, $number: Int!) {
		organization(login: $login) { projectV2(number: $number) { id } }
	}`
	var orgResp struct {
		Organization struct {
			ProjectV2 struct {
				ID string `json:"id"`
			} `json:"projectV2"`
		} `json:"organization"`
	}
	if err := graphqlDo(orgQuery, vars, &orgResp); err == nil {
		return orgResp.Organization.ProjectV2.ID
	}

	return ""
}

// --------------------------------------------------------------------------------------
// Step 8b — ConfigureProjectStatus (org accounts only)
// --------------------------------------------------------------------------------------

// StatusOption defines a single status column option for agentic projects.
type StatusOption struct {
	Name        string
	Color       string
	Description string
}

// AgenticStatusOptions defines the canonical 7-option status column configuration for agentic projects.
// Exported so the verify package can reference the canonical set for check/repair.
var AgenticStatusOptions = []StatusOption{
	{Name: "Scoping", Color: "PURPLE", Description: "Requirement or feature being scoped"},
	{Name: "Scheduled", Color: "BLUE", Description: "Scoped and queued, waiting for design"},
	{Name: "Backlog", Color: "GRAY", Description: "Prioritised, ready to start"},
	{Name: "In Design", Color: "PINK", Description: "Feature Design session active"},
	{Name: "In Implementation", Color: "YELLOW", Description: "Dev Session active"},
	{Name: "In Review", Color: "ORANGE", Description: "PR open, awaiting review"},
	{Name: "Done", Color: "GREEN", Description: "Merged and closed"},
}

// StatusOptionNames returns the canonical status option names in order.
func StatusOptionNames() []string {
	names := make([]string, len(AgenticStatusOptions))
	for i, opt := range AgenticStatusOptions {
		names[i] = opt.Name
	}
	return names
}

// ConfigureProjectStatus customises the GitHub Project Status field options.
// This is best-effort — failures are logged as warnings, not returned as errors.
func ConfigureProjectStatus(w io.Writer, cfg BootstrapConfig, state *StepState, graphqlDo GraphQLDoFunc) error {
	if state.ProjectNodeID == "" {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Skipping status column customisation (no project node ID)"))
		return nil
	}

	// Fetch the Status field ID.
	statusQuery := `query($projectId: ID!) {
		node(id: $projectId) {
			... on ProjectV2 {
				field(name: "Status") {
					... on ProjectV2SingleSelectField {
						id
						options { id name }
					}
				}
			}
		}
	}`

	var statusResp struct {
		Node struct {
			Field struct {
				ID      string `json:"id"`
				Options []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"options"`
			} `json:"field"`
		} `json:"node"`
	}

	if err := graphqlDo(statusQuery, map[string]interface{}{
		"projectId": state.ProjectNodeID,
	}, &statusResp); err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not fetch Status field: "+err.Error()))
		return nil
	}

	fieldID := statusResp.Node.Field.ID
	if fieldID == "" {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Status field not found on project — skipping"))
		return nil
	}

	// Build the singleSelectOptions input.
	var optionInputs []map[string]string
	for _, opt := range AgenticStatusOptions {
		optionInputs = append(optionInputs, map[string]string{
			"name":        opt.Name,
			"color":       opt.Color,
			"description": opt.Description,
		})
	}

	updateMutation := `mutation($fieldId: ID!, $projectId: ID!, $options: [ProjectV2SingleSelectFieldOptionInput!]!) {
		updateProjectV2Field(input: {
			fieldId: $fieldId
			projectId: $projectId
			singleSelectOptions: $options
		}) {
			field {
				... on ProjectV2SingleSelectField { id }
			}
		}
	}`

	var updateResp interface{}
	if err := graphqlDo(updateMutation, map[string]interface{}{
		"fieldId":   fieldID,
		"projectId": state.ProjectNodeID,
		"options":   optionInputs,
	}, &updateResp); err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not update Status columns: "+err.Error()))
		return nil
	}

	return nil
}

// --------------------------------------------------------------------------------------
// Step 8c — DeploySyncWorkflows (org accounts only)
// --------------------------------------------------------------------------------------

// syncWorkflowFiles lists the workflow files to copy for org accounts.
var syncWorkflowFiles = []string{
	"sync-label-to-status.yml",
	"sync-status-to-label.yml",
}

// DeploySyncWorkflows copies kanban sync workflow files from base/.github/workflows/
// into the bootstrapped repo's .github/workflows/ directory for org accounts.
// For personal accounts this step is silently skipped.
// Missing source files are handled gracefully (warning, not error).
func DeploySyncWorkflows(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
	if cfg.OwnerType != OwnerTypeOrg {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Skipping sync workflow deployment (personal account)"))
		return nil
	}

	sourceDir := filepath.Join(state.ClonePath, "base", ".github", "workflows")
	destDir := filepath.Join(state.ClonePath, ".github", "workflows")

	// Check which source files exist.
	var toCopy []string
	for _, f := range syncWorkflowFiles {
		srcPath := filepath.Join(sourceDir, f)
		if _, err := os.Stat(srcPath); err == nil {
			toCopy = append(toCopy, f)
		} else {
			fmt.Fprintln(w, "  "+ui.RenderWarning("Sync workflow "+f+" not found in base/ — skipping (dependency not yet met)"))
		}
	}

	if len(toCopy) == 0 {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· No sync workflows to deploy"))
		return nil
	}

	// Ensure destination directory exists.
	if err := os.MkdirAll(destDir, 0755); err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not create workflows directory: "+err.Error()))
		return nil
	}

	// Copy each file.
	for _, f := range toCopy {
		srcPath := filepath.Join(sourceDir, f)
		dstPath := filepath.Join(destDir, f)

		data, err := os.ReadFile(srcPath)
		if err != nil {
			fmt.Fprintln(w, "  "+ui.RenderWarning("Could not read "+f+": "+err.Error()))
			return nil
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			fmt.Fprintln(w, "  "+ui.RenderWarning("Could not write "+f+": "+err.Error()))
			return nil
		}
	}

	// Stage the copied files.
	out, err := runInDir(run, state.ClonePath, "git", "add", ".github/workflows/sync-label-to-status.yml", ".github/workflows/sync-status-to-label.yml")
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not stage sync workflows: "+strings.TrimSpace(out)))
		return nil
	}

	// Commit.
	out, err = runInDir(run, state.ClonePath, "git", "commit", "-m", "chore: deploy kanban sync workflows")
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not commit sync workflows: "+strings.TrimSpace(out)))
		return nil
	}

	// Push.
	out, err = runInDir(run, state.ClonePath, "git", "push", "origin", "main")
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not push sync workflows: "+strings.TrimSpace(out)))
		return nil
	}

	return nil
}

// --------------------------------------------------------------------------------------
// Step 9 — PrintSummary + Goose launch
// --------------------------------------------------------------------------------------

// LaunchFunc is called to launch Goose in the given directory.
// Injected so tests can substitute a fake without spawning a real process.
type LaunchFunc func(clonePath string) error

// DefaultLaunchGoose is the production LaunchFunc that runs goose session
// with CWD set to clonePath.
func DefaultLaunchGoose(clonePath string) error {
	quoted := "'" + strings.ReplaceAll(clonePath, "'", "'\\''") + "'"
	_, err := DefaultRunCommand("bash", "-c", "cd "+quoted+" && goose session")
	return err
}

// PrintSummary renders the final "Bootstrap complete" box and offers
// the Goose launch prompt.
func PrintSummary(w io.Writer, cfg BootstrapConfig, state *StepState, launch LaunchFunc) error {
	fmt.Fprintln(w)

	successBold := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorSuccess))
	fmt.Fprintln(w, successBold.Render("  ✔ Bootstrap complete"))
	fmt.Fprintln(w)

	content := fmt.Sprintf(
		"  %s  %s\n  %s  %s\n  %s  %s",
		ui.Muted.Render("Repo   "), ui.URL.Render(state.RepoURL),
		ui.Muted.Render("Project"), ui.URL.Render(state.ProjectURL),
		ui.Muted.Render("Clone  "), ui.Value.Render(state.ClonePath),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorSuccess)).
		Width(56).
		Padding(0, 1)

	fmt.Fprintln(w, box.Render(content))
	fmt.Fprintln(w)

	// --- PAT scope guidance for org accounts ---
	if cfg.OwnerType == OwnerTypeOrg {
		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorPrimary))
		fmt.Fprintln(w, "  "+infoStyle.Render("ℹ")+"  GOOSE_AGENT_PAT requires 'repo' and 'project' scopes for kanban sync to work.")
		fmt.Fprintln(w, "     Verify scopes at: "+ui.URL.Render("github.com/settings/tokens"))
		fmt.Fprintln(w)
	}

	// --- Goose launch prompt ---
	fmt.Fprintln(w, ui.SectionHeading.Render("  Start Requirements Session"))
	fmt.Fprintln(w)

	var choice string
	launchForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How would you like to continue?").
				Options(
					huh.NewOption("Terminal  — launch Goose CLI in "+cfg.ProjectName, "terminal"),
					huh.NewOption("Skip      — I'll do it manually", "skip"),
				).
				Value(&choice),
		),
	)
	if err := launchForm.Run(); err != nil {
		return fmt.Errorf("launch prompt: %w", err)
	}

	switch choice {
	case "terminal":
		fmt.Fprintln(w, "  "+ui.Muted.Render("Launching Goose..."))
		if err := launch(state.ClonePath); err != nil {
			return fmt.Errorf("launching Goose: %w", err)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.RenderOK("Goose launched in "+state.ClonePath))
		fmt.Fprintln(w, "  "+ui.Muted.Render("Your agent will read context and begin the Requirements Session."))
	default:
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.Value.Render(state.ClonePath))
		fmt.Fprintln(w, "  "+ui.Muted.Render("cd into the repo and start a Requirements Session when ready."))
	}

	return nil
}

// --------------------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------------------

// runInDir wraps RunCommandFunc so that the command runs with the given working directory.
// Because RunCommandFunc uses exec.Command internally and cannot set Dir directly,
// we wrap the command in "bash -c 'cd <dir> && <cmd>'".
func runInDir(run RunCommandFunc, dir string, name string, args ...string) (string, error) {
	quotedDir := "'" + strings.ReplaceAll(dir, "'", "'\\''") + "'"
	inner := "cd " + quotedDir + " && " + shellJoin(name, args...)
	return run("bash", "-c", inner)
}

// shellJoin single-quotes each token and joins them with spaces, producing a
// string safe for embedding inside a bash -c '...' invocation.
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
