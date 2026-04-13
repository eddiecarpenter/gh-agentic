package project

import (
	"fmt"
	"io"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// CreateConfig holds the parameters for project creation.
type CreateConfig struct {
	// Title is the name of the GitHub ProjectV2 to create.
	Title string
	// Version is the framework version tag to mount (e.g. "v2.0.10").
	Version string
}

// Create establishes this repository as a new project control plane.
//
// Guards:
//   - Blocked if AGENTIC_PROJECT_ID is already set.
//   - Blocked if the repo is already linked to a GitHub ProjectV2.
//
// Steps:
//  1. Check AGENTIC_PROJECT_ID is not already set.
//  2. Check repo is not already linked to any project.
//  3. Detect owner type; warn if personal account (proceed regardless).
//  4. Fetch owner and repo node IDs.
//  5. Create the GitHub ProjectV2.
//  6. Link the repo to the project.
//  7. Set AGENTIC_PROJECT_ID repo variable.
//  8. Clone the framework into .ai/.
func Create(w io.Writer, deps Deps, cfg CreateConfig) error {
	// Guard 1: AGENTIC_PROJECT_ID must not already be set.
	existing, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if existing != "" {
		return fmt.Errorf("repo is already affiliated with project %s — use 'gh agentic project join' to re-affiliate or 'gh agentic project unlink' first", existing)
	}

	// Guard 2: Repo must not already be linked to any GitHub Project.
	projects, err := deps.FetchProjectsForRepo(deps.Owner, deps.RepoName)
	if err != nil {
		return fmt.Errorf("checking existing project links: %w", err)
	}
	if len(projects) > 0 {
		return fmt.Errorf("repo is already linked to GitHub Project %q (%s) — unlink it first in GitHub Project settings", projects[0].Title, projects[0].ID)
	}

	// Step 3: Detect owner type and warn if personal account.
	ownerType, err := deps.DetectOwnerType(deps.Owner)
	if err != nil {
		fmt.Fprintf(w, "  %s  could not detect owner type: %v\n", ui.StatusWarning.Render("⚠"), err)
	} else if ownerType == auth.OwnerTypeUser {
		fmt.Fprintf(w, "  %s  %s is a personal account — GitHub Projects work best on organisations\n",
			ui.StatusWarning.Render("⚠"), deps.Owner)
	}

	// Step 4: Fetch owner and repo node IDs.
	fmt.Fprintf(w, "  Fetching owner and repository IDs...\n")
	ownerID, repoID, err := deps.FetchOwnerAndRepoIDs(deps.Owner, deps.RepoName)
	if err != nil {
		return fmt.Errorf("fetching node IDs: %w", err)
	}

	// Step 5: Create the GitHub ProjectV2.
	fmt.Fprintf(w, "  Creating project %q...\n", cfg.Title)
	projectID, err := deps.CreateProject(ownerID, cfg.Title)
	if err != nil {
		return fmt.Errorf("creating project: %w", err)
	}
	fmt.Fprintf(w, "  %s  Project created: %s\n", ui.StatusOK.Render("✓"), projectID)

	// Step 6: Link repo to project.
	fmt.Fprintf(w, "  Linking repository to project...\n")
	if err := deps.LinkRepoToProject(projectID, repoID); err != nil {
		return fmt.Errorf("linking repo to project: %w", err)
	}
	fmt.Fprintf(w, "  %s  Repository linked\n", ui.StatusOK.Render("✓"))

	// Step 7: Set AGENTIC_PROJECT_ID.
	fmt.Fprintf(w, "  Setting %s...\n", ProjectVarName)
	if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName, projectID); err != nil {
		return fmt.Errorf("setting %s: %w", ProjectVarName, err)
	}
	fmt.Fprintf(w, "  %s  %s set\n", ui.StatusOK.Render("✓"), ProjectVarName)

	// Step 8: Clone framework into .ai/.
	fmt.Fprintf(w, "  Mounting framework %s...\n", cfg.Version)
	if err := mount.DownloadFramework(deps.Root, cfg.Version, deps.Clone); err != nil {
		return fmt.Errorf("mounting framework: %w", err)
	}
	fmt.Fprintf(w, "  %s  Framework mounted at %s\n", ui.StatusOK.Render("✓"), cfg.Version)

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Project created and control plane configured"))
	return nil
}
