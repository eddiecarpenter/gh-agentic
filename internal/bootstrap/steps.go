// Package bootstrap implements the business logic for gh agentic bootstrap.
package bootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/eddiecarpenter/gh-agentic/internal/tarball"
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

	// CredentialsSet is true when CLAUDE_CREDENTIALS_JSON was successfully set
	// as a repo secret by SetClaudeCredentials.
	CredentialsSet bool

	// AgentPATFound is true when GOOSE_AGENT_PAT was found in the repo secrets
	// by ValidateAgentPAT.
	AgentPATFound bool

	// ExistingRepo is true when the target repository already existed before
	// bootstrap ran. When set, downstream steps use a branch-based flow
	// instead of committing directly to main.
	ExistingRepo bool

	// PRURL is the URL of the pull request created when bootstrapping an
	// existing repo onto a bootstrap/init branch.
	PRURL string
}

// agenticMarkers are the files and directories whose presence indicates
// a repository has already been bootstrapped with the agentic framework.
var agenticMarkers = []string{"TEMPLATE_SOURCE", "TEMPLATE_VERSION", "base"}

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

// FetchReleaseFunc fetches the latest release tag for a given repo slug.
// Injected so tests can substitute a fake implementation.
type FetchReleaseFunc func(repo string) (string, error)

// fetchTarballFn is the tarball fetch function used by CreateRepo.
// Overridden in tests to simulate fetch failures.
var fetchTarballFn = tarball.DefaultFetch

// commitTemplateFiles stages all files in the cloned repo and commits with the
// standard template initialisation message.
func commitTemplateFiles(state *StepState, run RunCommandFunc, releaseTag string) error {
	out, err := runInDir(run, state.ClonePath, "git", "add", "-A")
	if err != nil {
		return fmt.Errorf("git add: %w\n%s", err, strings.TrimSpace(out))
	}
	commitMsg := fmt.Sprintf("chore: initialise from template %s", releaseTag)
	out, err = runInDir(run, state.ClonePath, "git", "commit", "-m", commitMsg)
	if err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(out))
	}
	return nil
}

// fetchRepoNodeID fetches the GitHub node ID for the repository via the REST API
// and stores it in state.RepoNodeID. Best-effort — failures are logged as warnings.
func fetchRepoNodeID(w io.Writer, fullName string, state *StepState) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Could not create GitHub API client: "+err.Error()))
		return
	}
	var repoResp struct {
		NodeID string `json:"node_id"`
	}
	if err := client.Get(fmt.Sprintf("repos/%s", fullName), &repoResp); err != nil {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Could not fetch repo node ID: "+err.Error()))
		return
	}
	state.RepoNodeID = repoResp.NodeID
}

// cleanupRemoteRepo deletes the named GitHub repo during error recovery.
// Best-effort — failures are logged as warnings.
func cleanupRemoteRepo(w io.Writer, fullName string, run RunCommandFunc) {
	out, err := run("gh", "repo", "delete", fullName, "--yes")
	if err != nil {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Could not clean up remote repo: "+strings.TrimSpace(out)))
	}
}

// bootstrapExistingRepo clones an existing repo, guards against double-bootstrap
// and branch conflicts, creates the bootstrap/init branch, applies the template
// tarball, and commits the initial template state.
func bootstrapExistingRepo(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc, releaseTag string) error {
	fmt.Fprintln(w, "  "+ui.RenderInfo("Repo already exists — bootstrapping onto existing repo"))

	resolvedPath, err := resolveCloneConflictFn(w, state.ClonePath)
	if err != nil {
		return err
	}
	state.ClonePath = resolvedPath

	fullName := cfg.Owner + "/" + state.RepoName
	sshURL := fmt.Sprintf("git@github.com:%s.git", fullName)
	out, err := run("git", "clone", sshURL, state.ClonePath)
	if err != nil {
		return fmt.Errorf("git clone (existing repo): %w\n%s", err, strings.TrimSpace(out))
	}

	for _, marker := range agenticMarkers {
		if _, statErr := os.Stat(filepath.Join(state.ClonePath, marker)); statErr == nil {
			return fmt.Errorf("this repo has already been bootstrapped — aborting to prevent overwrite")
		}
	}

	// Only treat non-empty ls-remote output as "branch exists" when the command
	// succeeded — a failure (network/auth issue) is not the same as the branch existing.
	lsOut, lsErr := runInDir(run, state.ClonePath, "git", "ls-remote", "--heads", "origin", "bootstrap/init")
	if lsErr == nil && strings.TrimSpace(lsOut) != "" {
		return fmt.Errorf("branch bootstrap/init already exists — aborting to prevent force-push")
	}

	out, err = runInDir(run, state.ClonePath, "git", "checkout", "-b", "bootstrap/init")
	if err != nil {
		return fmt.Errorf("git checkout -b bootstrap/init: %w\n%s", err, strings.TrimSpace(out))
	}

	if err := tarball.ExtractFromTemplate(cfg.TemplateRepo, releaseTag, state.ClonePath, nil, fetchTarballFn); err != nil {
		return fmt.Errorf("populating from template tarball: %w", err)
	}

	versionPath := filepath.Join(state.ClonePath, "TEMPLATE_VERSION")
	if err := os.WriteFile(versionPath, []byte(releaseTag+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing TEMPLATE_VERSION: %w", err)
	}

	return commitTemplateFiles(state, run, releaseTag)
}

// createNewRepo creates a blank private GitHub repo, clones it locally, applies
// the template tarball, and commits. Cleans up the remote repo on clone or
// tarball failure to avoid leaving orphaned repos.
func createNewRepo(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc, releaseTag string) error {
	resolvedPath, err := resolveCloneConflictFn(w, state.ClonePath)
	if err != nil {
		return err
	}
	state.ClonePath = resolvedPath

	fullName := cfg.Owner + "/" + state.RepoName
	out, err := run("gh", "repo", "create", fullName, "--private")
	if err != nil {
		return fmt.Errorf("gh repo create: %w\n%s", err, strings.TrimSpace(out))
	}

	sshURL := fmt.Sprintf("git@github.com:%s.git", fullName)
	out, err = run("git", "clone", sshURL, state.ClonePath)
	if err != nil {
		cleanupRemoteRepo(w, fullName, run)
		return fmt.Errorf("git clone: %w\n%s", err, strings.TrimSpace(out))
	}

	if err := tarball.ExtractFromTemplate(cfg.TemplateRepo, releaseTag, state.ClonePath, nil, fetchTarballFn); err != nil {
		cleanupRemoteRepo(w, fullName, run)
		return fmt.Errorf("populating from template tarball: %w", err)
	}

	versionPath := filepath.Join(state.ClonePath, "TEMPLATE_VERSION")
	if err := os.WriteFile(versionPath, []byte(releaseTag+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing TEMPLATE_VERSION: %w", err)
	}

	return commitTemplateFiles(state, run, releaseTag)
}

// CreateRepo initialises state, validates preconditions, fetches the release tag,
// and delegates to bootstrapExistingRepo or createNewRepo depending on cfg.ExistingRepo.
// On success it fetches the repo node ID (best-effort) for use by downstream steps.
//
// workDir is the parent directory into which the repo will be cloned.
// run is injected so tests can substitute a fake implementation without spawning real processes.
// fetchRelease is injected to fetch the latest release tag (use sync.DefaultFetchRelease for production).
func CreateRepo(w io.Writer, cfg BootstrapConfig, state *StepState, workDir string, run RunCommandFunc, fetchRelease FetchReleaseFunc) error {
	name := repoName(cfg)
	state.RepoName = name
	state.ClonePath = filepath.Join(workDir, name)
	state.RepoURL = fmt.Sprintf("https://github.com/%s/%s", cfg.Owner, name)
	state.ExistingRepo = cfg.ExistingRepo

	if cfg.TemplateRepo == "" {
		return fmt.Errorf("template repo (TEMPLATE_SOURCE) is not configured")
	}

	releaseTag, err := fetchRelease(cfg.TemplateRepo)
	if err != nil {
		return fmt.Errorf("fetching release tag for %s: %w", cfg.TemplateRepo, err)
	}
	if releaseTag == "" {
		return fmt.Errorf("no release found for template repo %s", cfg.TemplateRepo)
	}

	if cfg.ExistingRepo {
		if err := bootstrapExistingRepo(w, cfg, state, run, releaseTag); err != nil {
			return err
		}
	} else {
		if err := createNewRepo(w, cfg, state, run, releaseTag); err != nil {
			return err
		}
	}

	fetchRepoNodeID(w, cfg.Owner+"/"+name, state)
	return nil
}

// --------------------------------------------------------------------------------------
// Step 6 — ConfigureRepo
// --------------------------------------------------------------------------------------

// standardLabels are the 11 labels created in every agentic repo.
var standardLabels = []string{
	"requirement", "feature", "task", "backlog", "draft",
	"scoping", "scheduled",
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

// writeProjectFiles writes the standard project configuration files into the
// cloned repo: REPOS.md, AGENTS.local.md, TEMPLATE_SOURCE, skills/.gitkeep,
// and README.md.
func writeProjectFiles(cfg BootstrapConfig, state *StepState) error {
	reposMD := fmt.Sprintf("# REPOS.md\n\n## %s\n\n- **Repo:** git@github.com:%s/%s.git\n- **Stack:** %s\n- **Type:** agentic\n- **Status:** active\n- **Description:** %s\n",
		state.RepoName, cfg.Owner, state.RepoName, strings.Join(cfg.Stacks, ", "), cfg.Description)
	if err := os.WriteFile(filepath.Join(state.ClonePath, "REPOS.md"), []byte(reposMD), 0644); err != nil {
		return fmt.Errorf("writing REPOS.md: %w", err)
	}

	agentsLocal := fmt.Sprintf(
		"# AGENTS.local.md — Local Overrides\n\n"+
			"This file contains project-specific rules and overrides that extend or\n"+
			"supersede the global protocol defined in `base/AGENTS.md`.\n\n"+
			"This file is never overwritten by a template sync.\n\n---\n\n"+
			"## Template Source\n\nTemplate: %s\n\n"+
			"## Project\n\n"+
			"- **Name:** %s\n"+
			"- **Topology:** %s\n"+
			"- **Stack:** %s\n"+
			"- **Description:** %s\n\n"+
			"## Repo\n\n"+
			"- **GitHub:** %s\n"+
			"- **Owner:** %s\n\n"+
			"## Skills\n\n"+
			"The `skills/` directory is for local project-specific skills that extend\n"+
			"or override template skills in `base/skills/`.\n",
		cfg.TemplateRepo, cfg.ProjectName, cfg.Topology, strings.Join(cfg.Stacks, ", "), cfg.Description, state.RepoURL, cfg.Owner)
	if err := os.WriteFile(filepath.Join(state.ClonePath, "AGENTS.local.md"), []byte(agentsLocal), 0644); err != nil {
		return fmt.Errorf("writing AGENTS.local.md: %w", err)
	}

	if err := os.WriteFile(filepath.Join(state.ClonePath, "TEMPLATE_SOURCE"), []byte(cfg.TemplateRepo+"\n"), 0644); err != nil {
		return fmt.Errorf("writing TEMPLATE_SOURCE: %w", err)
	}

	skillsDir := filepath.Join(state.ClonePath, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("creating skills/: %w", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, ".gitkeep"), []byte{}, 0644); err != nil {
		return fmt.Errorf("writing skills/.gitkeep: %w", err)
	}

	readmeMD := fmt.Sprintf(
		"# %s\n\n%s\n\n## Setup\n\n"+
			"See [docs/PROJECT_BRIEF.md](docs/PROJECT_BRIEF.md) for project context.\n\n"+
			"## Agent sessions\n\n"+
			"This repo uses the [agentic development framework](https://github.com/%s).\n"+
			"See `base/AGENTS.md` and `AGENTS.local.md` for session protocols.\n",
		cfg.ProjectName, cfg.Description, cfg.TemplateRepo)
	if err := os.WriteFile(filepath.Join(state.ClonePath, "README.md"), []byte(readmeMD), 0644); err != nil {
		return fmt.Errorf("writing README.md: %w", err)
	}

	return nil
}

// deployPublishWorkflow copies publish-release.yml from the template examples
// into the repo's .github/workflows/ directory. Skipped silently if the source
// file does not exist. Returns an error only on unexpected read/write failures.
func deployPublishWorkflow(w io.Writer, state *StepState) error {
	publishReleaseSrc := filepath.Join(state.ClonePath, "base", "docs", "examples", "publish-release.yml")
	publishReleaseDst := filepath.Join(state.ClonePath, ".github", "workflows", "publish-release.yml")
	srcData, err := os.ReadFile(publishReleaseSrc)
	if os.IsNotExist(err) {
		fmt.Fprintf(w, "· publish-release.yml example not found in template — skipping\n")
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading publish-release.yml example: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(publishReleaseDst), 0755); err != nil {
		return fmt.Errorf("creating .github/workflows/: %w", err)
	}
	if err := os.WriteFile(publishReleaseDst, srcData, 0644); err != nil {
		return fmt.Errorf("writing publish-release.yml: %w", err)
	}
	fmt.Fprintf(w, "· publish-release.yml deployed\n")
	return nil
}

// PopulateRepo writes project configuration files into the cloned repo,
// optionally scaffolds Antora, deploys publish-release.yml, then commits and pushes.
//
// run is injected so tests can substitute a fake implementation.
func PopulateRepo(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
	if err := writeProjectFiles(cfg, state); err != nil {
		return err
	}

	if cfg.Antora {
		if err := scaffoldAntora(state.ClonePath, cfg.ProjectName); err != nil {
			return fmt.Errorf("scaffolding Antora: %w", err)
		}
	}

	if err := deployPublishWorkflow(w, state); err != nil {
		return err
	}

	out, err := runInDir(run, state.ClonePath, "git", "add", "-A")
	if err != nil {
		return fmt.Errorf("git add: %w\n%s", err, strings.TrimSpace(out))
	}

	out, err = runInDir(run, state.ClonePath, "git", "commit", "-m", "chore: bootstrap "+cfg.ProjectName)
	if err != nil {
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(out))
	}

	// Push — skip for existing repos (branch push + PR handled by OpenBootstrapPR).
	if !state.ExistingRepo {
		out, err = runInDir(run, state.ClonePath, "git", "push", "origin", "main")
		if err != nil {
			return fmt.Errorf("git push: %w\n%s", err, strings.TrimSpace(out))
		}
	}

	return nil
}

// OpenBootstrapPR pushes the bootstrap/init branch and opens a pull request
// titled "chore: bootstrap agentic environment". It is only called when
// state.ExistingRepo is true. The PR URL is stored in state.PRURL.
//
// run is injected so tests can substitute a fake implementation.
func OpenBootstrapPR(cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
	if !state.ExistingRepo {
		return nil
	}

	fullName := cfg.Owner + "/" + state.RepoName

	// Push the bootstrap/init branch.
	out, err := runInDir(run, state.ClonePath, "git", "push", "-u", "origin", "bootstrap/init")
	if err != nil {
		return fmt.Errorf("git push bootstrap/init: %w\n%s", err, strings.TrimSpace(out))
	}

	// Open a PR.
	prOut, prErr := run("gh", "pr", "create",
		"--repo", fullName,
		"--base", "main",
		"--head", "bootstrap/init",
		"--title", "chore: bootstrap agentic environment",
		"--body", "Bootstrap the agentic development framework onto this repository.",
	)
	if prErr != nil {
		return fmt.Errorf("gh pr create: %w\n%s", prErr, strings.TrimSpace(prOut))
	}

	state.PRURL = strings.TrimSpace(prOut)
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

// setProjectVariable stores the project node ID as a variable via gh CLI.
// For org-owned repos it sets at org level; for personal repos it sets at repo
// level. This is best-effort — failure is logged as a warning, not returned as
// an error.
func setProjectVariable(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) {
	if state.ProjectNodeID == "" {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Skipping AGENTIC_PROJECT_ID variable (no project node ID)"))
		return
	}

	// Use --org for org-owned repos, --repo for personal repos.
	var scopeArgs []string
	if cfg.OwnerType == OwnerTypeOrg {
		scopeArgs = []string{"--org", cfg.Owner}
	} else {
		fullName := cfg.Owner + "/" + state.RepoName
		scopeArgs = []string{"--repo", fullName}
	}

	args := append([]string{"variable", "set", "AGENTIC_PROJECT_ID", "--body", state.ProjectNodeID}, scopeArgs...)
	out, err := run("gh", args...)
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
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// fetchStatusFieldID fetches the GraphQL node ID of the Status single-select field
// on the given project. Returns an empty string if the field is not found.
func fetchStatusFieldID(graphqlDo GraphQLDoFunc, projectNodeID string) (string, error) {
	query := `query($projectId: ID!) {
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

	var resp struct {
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

	if err := graphqlDo(query, map[string]interface{}{"projectId": projectNodeID}, &resp); err != nil {
		return "", err
	}
	return resp.Node.Field.ID, nil
}

// ConfigureProjectStatus customises the GitHub Project Status field options.
// It reads canonical options from base/project-template.json in the cloned repo.
// This is best-effort — failures are logged as warnings, not returned as errors.
func ConfigureProjectStatus(w io.Writer, state *StepState, graphqlDo GraphQLDoFunc) error {
	if state.ProjectNodeID == "" {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Skipping status column customisation (no project node ID)"))
		return nil
	}

	tmpl, err := LoadProjectTemplate(state.ClonePath)
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not load project template: "+err.Error()))
		return nil
	}

	fieldID, err := fetchStatusFieldID(graphqlDo, state.ProjectNodeID)
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not fetch Status field: "+err.Error()))
		return nil
	}
	if fieldID == "" {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Status field not found on project — skipping"))
		return nil
	}

	var optionInputs []map[string]string
	for _, opt := range tmpl.ResolvedStatusOptions() {
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
			continue
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			fmt.Fprintln(w, "  "+ui.RenderWarning("Could not write "+f+": "+err.Error()))
			continue
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

// PrintSummary renders the final "Bootstrap complete" box with clear
// next-step instructions. No interactive prompt is shown.
func PrintSummary(w io.Writer, cfg BootstrapConfig, state *StepState) error {
	fmt.Fprintln(w)

	successBold := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorSuccess))
	fmt.Fprintln(w, successBold.Render("  ✔ Bootstrap complete!"))
	fmt.Fprintln(w)

	var content string
	if state.ExistingRepo && state.PRURL != "" {
		content = fmt.Sprintf(
			"  %s  %s\n  %s  %s\n  %s  %s",
			ui.Muted.Render("PR     "), ui.URL.Render(state.PRURL),
			ui.Muted.Render("Project"), ui.URL.Render(state.ProjectURL),
			ui.Muted.Render("Clone  "), ui.Value.Render(state.ClonePath),
		)
	} else {
		content = fmt.Sprintf(
			"  %s  %s\n  %s  %s\n  %s  %s",
			ui.Muted.Render("Repo   "), ui.URL.Render(state.RepoURL),
			ui.Muted.Render("Project"), ui.URL.Render(state.ProjectURL),
			ui.Muted.Render("Clone  "), ui.Value.Render(state.ClonePath),
		)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorSuccess)).
		Width(56).
		Padding(0, 1)

	fmt.Fprintln(w, box.Render(content))
	fmt.Fprintln(w)

	// --- Pipeline configuration ---
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorPrimary))

	credStatus := ui.RenderWarning("not set")
	if state.CredentialsSet {
		credStatus = ui.RenderOK("set")
	}

	fmt.Fprintln(w, "  "+ui.Muted.Render("Runner      ")+ui.Value.Render(cfg.RunnerLabel))
	fmt.Fprintln(w, "  "+ui.Muted.Render("Provider    ")+ui.Value.Render(cfg.GooseProvider))
	fmt.Fprintln(w, "  "+ui.Muted.Render("Model       ")+ui.Value.Render(cfg.GooseModel))
	fmt.Fprintln(w, "  "+ui.Muted.Render("Credentials ")+credStatus)
	fmt.Fprintln(w)

	// --- GOOSE_AGENT_PAT warning ---
	if !state.AgentPATFound {
		fullName := cfg.Owner + "/" + state.RepoName
		fmt.Fprintln(w, "  "+ui.RenderWarning("GOOSE_AGENT_PAT secret not found — pipeline will not run until added."))
		fmt.Fprintln(w, "  "+ui.Muted.Render("  Add it at: "+fmt.Sprintf("https://github.com/%s/settings/secrets/actions", fullName)))
		fmt.Fprintln(w)
	}

	// --- PAT scope guidance for org accounts ---
	if cfg.OwnerType == OwnerTypeOrg {
		fmt.Fprintln(w, "  "+infoStyle.Render("ℹ")+"  GOOSE_AGENT_PAT requires 'repo' and 'project' scopes for kanban sync to work.")
		fmt.Fprintln(w, "     Verify scopes at: "+ui.URL.Render("github.com/settings/tokens"))
		fmt.Fprintln(w)
	}

	// --- Next steps ---
	repoName := state.RepoName
	if repoName == "" {
		repoName = cfg.ProjectName
	}
	fmt.Fprintln(w, ui.SectionHeading.Render("  Next steps"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  1. Open Claude Code or Goose Desktop")
	fmt.Fprintln(w, "  2. Select '"+ui.Value.Render(repoName)+"' as your workspace")
	fmt.Fprintln(w, "  3. Start a new session with the prompt:")
	fmt.Fprintln(w, "     "+ui.Muted.Render("\"Assist me with defining my new AI-native developed application\""))
	fmt.Fprintln(w)

	return nil
}

// ErrCloneAborted is returned when the user chooses to abort clone conflict resolution.
var ErrCloneAborted = errors.New("clone aborted by user")

// cloneConflictRename is the value for the "rename to backup" option.
const cloneConflictRename = "rename"

// cloneConflictAbort is the value for the "abort" option.
const cloneConflictAbort = "abort"

// ResolveCloneConflictFunc checks whether the target directory exists and resolves
// the conflict interactively. Injected so tests can substitute a no-op.
type ResolveCloneConflictFunc func(w io.Writer, clonePath string) (string, error)

// resolveCloneConflictFn is the active conflict resolver. Override in tests.
var resolveCloneConflictFn ResolveCloneConflictFunc = DefaultResolveCloneConflict

// DefaultResolveCloneConflict checks whether the target directory exists. If it does,
// it presents the user with recovery options: rename to backup or abort.
// Returns the resolved clone path or an error.
func DefaultResolveCloneConflict(w io.Writer, clonePath string) (string, error) {
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		return clonePath, nil // No conflict — proceed normally.
	}

	fmt.Fprintln(w, "  "+ui.RenderWarning(fmt.Sprintf("Directory %q already exists", filepath.Base(clonePath))))

	var choice string
	conflictForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Directory already exists").
				Description("Choose how to resolve the conflict").
				Options(
					huh.NewOption("Rename existing to .backup and continue", cloneConflictRename),
					huh.NewOption("Abort", cloneConflictAbort),
				).
				Value(&choice),
		),
	)
	if err := conflictForm.Run(); err != nil {
		return "", fmt.Errorf("clone conflict form: %w", err)
	}

	switch choice {
	case cloneConflictRename:
		backupPath := findBackupPath(clonePath)
		if err := os.Rename(clonePath, backupPath); err != nil {
			return "", fmt.Errorf("renaming %s to %s: %w", clonePath, backupPath, err)
		}
		fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("· Renamed existing directory to %s", filepath.Base(backupPath))))
		return clonePath, nil
	case cloneConflictAbort:
		return "", ErrCloneAborted
	default:
		return "", ErrCloneAborted
	}
}

// findBackupPath returns a non-conflicting backup path for the given directory.
// It tries <path>.backup, <path>.backup.1, <path>.backup.2, etc.
func findBackupPath(path string) string {
	candidate := path + ".backup"
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for i := 1; ; i++ {
		candidate = fmt.Sprintf("%s.backup.%d", path, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
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

// SetAgentUserVariable sets the AGENT_USER GitHub Actions variable at the
// resolved scope (org or repo level). If AgentUser is empty, the step is
// skipped silently. Failure is logged as a warning (non-fatal), matching the
// best-effort pattern used by setProjectVariable.
func SetAgentUserVariable(w io.Writer, cfg BootstrapConfig, state *StepState, run RunCommandFunc) error {
	if cfg.AgentUser == "" {
		fmt.Fprintln(w, "  "+ui.Muted.Render("· Skipping AGENT_USER variable (no agent user configured)"))
		return nil
	}

	var out string
	var err error
	if cfg.AgentUserScope == AgentUserScopeOrg {
		out, err = run("gh", "variable", "set", "AGENT_USER", "--body", cfg.AgentUser, "--org", cfg.Owner)
	} else {
		fullName := cfg.Owner + "/" + state.RepoName
		out, err = run("gh", "variable", "set", "AGENT_USER", "--body", cfg.AgentUser, "--repo", fullName)
	}

	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning("Could not set AGENT_USER variable: "+strings.TrimSpace(out)))
		return nil // non-fatal
	}

	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("· AGENT_USER=%s set at %s level", cfg.AgentUser, cfg.AgentUserScope)))

	// For org-scoped agent users, verify org membership and invite if needed.
	if cfg.AgentUserScope == AgentUserScopeOrg {
		verifyAndInviteOrgMember(w, cfg.Owner, cfg.AgentUser, run)
	}

	return nil
}

// verifyAndInviteOrgMember checks whether agentUser is an org member and
// attempts to send an invitation if not. Failures are logged as warnings
// (non-fatal) to avoid blocking bootstrap.
func verifyAndInviteOrgMember(w io.Writer, owner, agentUser string, run RunCommandFunc) {
	// Check if already an org member (204 = member, 404 = not a member).
	_, err := run("gh", "api", "orgs/"+owner+"/members/"+agentUser)
	if err == nil {
		fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("· %s is already an org member of %s", agentUser, owner)))
		return
	}

	// Not a member — resolve numeric user ID for the invitation.
	idOut, err := run("gh", "api", "users/"+agentUser, "--jq", ".id")
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning(fmt.Sprintf("Could not resolve GitHub user ID for %s: %v", agentUser, err)))
		return
	}

	userID := strings.TrimSpace(idOut)
	if userID == "" || userID == "null" {
		fmt.Fprintln(w, "  "+ui.RenderWarning(fmt.Sprintf("GitHub user %s not found — cannot invite to org", agentUser)))
		return
	}

	// Attempt org invitation.
	_, err = run("gh", "api", "-X", "POST", "orgs/"+owner+"/invitations", "-f", "invitee_id="+userID)
	if err != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning(fmt.Sprintf(
			"Could not invite %s to org %s. An org admin must invite them at https://github.com/orgs/%s/people",
			agentUser, owner, owner)))
		return
	}

	fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("· Invited %s to org %s", agentUser, owner)))
}
