package project

import (
	"fmt"
	"io"
	"strings"

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
	// Topology is the AGENTIC_TOPOLOGY marker to write for this repo.
	// Empty value defaults to "federated" — Create's historical behaviour.
	// Single-topology init sets this to "single" so the federated-owner guard
	// does not fire on user-owned repos that are setting up a standalone
	// control plane.
	Topology string
}

// Create establishes this repository as a new project control plane.
//
// Guards:
//   - Blocked if AGENTIC_PROJECT_ID is already set.
//   - Blocked if the repo is already linked to a GitHub ProjectV2.
//   - Blocked if cfg.Topology is federated and the owner is a user account.
//
// Steps:
//  1. Check AGENTIC_PROJECT_ID is not already set.
//  2. Check repo is not already linked to any project.
//  3. Detect owner type; refuse if federated+user, warn otherwise.
//  4. Fetch owner and repo node IDs.
//  5. Create the GitHub ProjectV2.
//  6. Link the repo to the project.
//  7. Set AGENTIC_PROJECT_ID repo variable.
//  8. Clone the framework into .ai/.
func Create(w io.Writer, deps Deps, cfg CreateConfig) error {
	// Default topology for control-plane creation is federated — matches the
	// historical behaviour of this function and the CLI `project create`
	// contract. initSingle passes "single" explicitly.
	topology := cfg.Topology
	if topology == "" {
		topology = "federated"
	}
	// Guard 1: AGENTIC_PROJECT_ID must not already be set.
	existing, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if existing != "" {
		existingName := ProjectDisplayName(deps, existing)
		return fmt.Errorf("this repo is already part of agentic project %q — use 'gh agentic project init' to join a different project or 'gh agentic project unlink' first", existingName)
	}

	// Guard 2: Repo must not already be linked to any GitHub Project.
	projects, err := deps.FetchProjectsForRepo(deps.Owner, deps.RepoName)
	if err != nil {
		return fmt.Errorf("checking existing project links: %w", err)
	}
	if len(projects) > 0 {
		return fmt.Errorf("repo is already linked to GitHub Project %q (%s) — unlink it first in GitHub Project settings", projects[0].Title, projects[0].ID)
	}

	// Step 3: Detect owner type.
	// Federated topology is a hard rule: it requires a GitHub Organization.
	// Refuse here — before we create the project, write any variable, or
	// mount the framework — if the owner is a user account. Under single
	// topology a personal account is merely sub-optimal, so the legacy
	// warning is preserved.
	ownerType, err := deps.DetectOwnerType(deps.Owner)
	if err != nil {
		fmt.Fprintf(w, "  %s  could not detect owner type: %v\n", ui.StatusWarning.Render("⚠"), err)
	} else {
		if guardErr := EnsureFederatedOwnerIsOrg(topology, deps.Owner, ownerType); guardErr != nil {
			return guardErr
		}
		if ownerType == auth.OwnerTypeUser {
			fmt.Fprintf(w, "  %s  %s is a personal account — GitHub Projects work best on organisations\n",
				ui.StatusWarning.Render("⚠"), deps.Owner)
		}
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

	// Set topology marker. Defaults to "federated" when the caller did not
	// specify one; single-topology init passes "single" to avoid the
	// federated-owner guard on user-owned repos.
	if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, TopologyVarName, topology); err != nil {
		return fmt.Errorf("setting %s: %w", TopologyVarName, err)
	}
	fmt.Fprintf(w, "  %s  %s set to %s\n", ui.StatusOK.Render("✓"), TopologyVarName, topology)

	// Set the canonical framework version for domain repos to read.
	if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName, cfg.Version); err != nil {
		return fmt.Errorf("setting %s: %w", FrameworkVersionVarName, err)
	}
	fmt.Fprintf(w, "  %s  %s set to %s\n", ui.StatusOK.Render("✓"), FrameworkVersionVarName, cfg.Version)

	// Step 8: Clone framework into .ai/.
	fmt.Fprintf(w, "  Mounting framework %s...\n", cfg.Version)
	if err := mount.DownloadFramework(deps.Root, cfg.Version, deps.Clone); err != nil {
		return fmt.Errorf("mounting framework: %w", err)
	}
	fmt.Fprintf(w, "  %s  Framework mounted at %s\n", ui.StatusOK.Render("✓"), cfg.Version)

	// Step 9: Scaffold project from template (description, readme, status options, views).
	fmt.Fprintf(w, "  Scaffolding project from template...\n")
	scaffoldProject(w, deps, projectID, ownerType)

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Agentic project created and control plane configured"))
	return nil
}

// scaffoldProject applies the project template to a newly created GitHub Project.
// It updates the project description, README, Status field options, and creates views.
// Scaffold failures are non-fatal — they emit warnings but do not abort the create flow.
func scaffoldProject(w io.Writer, deps Deps, projectID, ownerType string) {
	tpl, err := ReadProjectTemplate()
	if err != nil {
		fmt.Fprintf(w, "  %s  Could not read project template (%v) — skipping scaffold\n",
			ui.StatusWarning.Render("⚠"), err)
		return
	}

	// Update project description and README.
	if tpl.ShortDescription != "" || tpl.Readme != "" {
		if err := deps.UpdateProject(projectID, tpl.ShortDescription, tpl.Readme); err != nil {
			fmt.Fprintf(w, "  %s  Could not update project description: %v\n", ui.StatusWarning.Render("⚠"), err)
		} else {
			fmt.Fprintf(w, "  %s  Project description and README updated\n", ui.StatusOK.Render("✓"))
		}
	}

	// Update Status field options.
	if len(tpl.StatusField.Options) > 0 {
		fields, err := deps.FetchProjectFields(projectID)
		if err != nil {
			fmt.Fprintf(w, "  %s  Could not fetch project fields: %v\n", ui.StatusWarning.Render("⚠"), err)
		} else {
			var statusFieldID string
			for _, f := range fields {
				if f.Name == "Status" && f.DataType == "SINGLE_SELECT" {
					statusFieldID = f.ID
					break
				}
			}
			if statusFieldID == "" {
				fmt.Fprintf(w, "  %s  Status field not found — skipping option update\n", ui.StatusWarning.Render("⚠"))
			} else if err := deps.UpdateStatusFieldOptions(statusFieldID, tpl.StatusField.Options); err != nil {
				fmt.Fprintf(w, "  %s  Could not update Status field options: %v\n", ui.StatusWarning.Render("⚠"), err)
			} else {
				fmt.Fprintf(w, "  %s  Status field options updated (%d options)\n",
					ui.StatusOK.Render("✓"), len(tpl.StatusField.Options))
			}
		}
	}

	// Create views via the REST API.
	if len(tpl.Views) > 0 {
		projectNumber, err := deps.FetchProjectNumber(projectID)
		if err != nil {
			fmt.Fprintf(w, "  %s  Could not fetch project number: %v\n", ui.StatusWarning.Render("⚠"), err)
		} else {
			viewsCreated := 0
			for _, v := range tpl.Views {
				layout := strings.ToLower(strings.TrimSuffix(v.Layout, "_LAYOUT"))
				if err := deps.CreateProjectView(deps.Owner, ownerType, projectNumber, v.Name, layout, v.Filter); err != nil {
					fmt.Fprintf(w, "  %s  Could not create view %q: %v\n", ui.StatusWarning.Render("⚠"), v.Name, err)
				} else {
					fmt.Fprintf(w, "  %s  View %q created (%s)\n", ui.StatusOK.Render("✓"), v.Name, layout)
					viewsCreated++
				}
			}
			if viewsCreated > 0 {
				fmt.Fprintf(w, "\n  %s  GitHub creates a default \"View 1\" that cannot be deleted via API.\n", ui.StatusWarning.Render("⚠"))
				fmt.Fprintf(w, "       Delete it manually: open the project → click ··· on the \"View 1\" tab → Delete view\n")
			}
		}
	}
}
