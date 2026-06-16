package project

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// JoinDomain registers a domain repo with the control-plane project. Run on the
// control plane, it adds targetOwner/targetRepo to FEDERATION.md under the named
// domain (lazy-creating the domain with domainPurpose when it does not yet
// exist), links the repo to the Project, and sets AGENTIC_PROJECT_ID on the
// target repo. It does NOT mount the framework — domain repos are pure code.
func JoinDomain(w io.Writer, deps Deps, projectID, targetOwner, targetRepo, domain, domainPurpose, repoPurpose string) error {
	ownerRepo := targetOwner + "/" + targetRepo

	// Read the manifest, or start an empty one when the control plane has no
	// FEDERATION.md yet (the first join bootstraps it).
	fed, err := ReadFederation(deps.Root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fed = &Federation{}
		} else {
			return err
		}
	}

	created, err := fed.AddRepo(domain, domainPurpose, ownerRepo, repoPurpose)
	if err != nil {
		return err
	}
	if err := WriteFederation(deps.Root, fed); err != nil {
		return err
	}
	if created {
		fmt.Fprintf(w, "  %s  Created domain %q\n", ui.StatusOK.Render("✓"), domain)
	}
	fmt.Fprintf(w, "  %s  Registered %s under domain %q in FEDERATION.md\n", ui.StatusOK.Render("✓"), ownerRepo, domain)

	// Link the target repo to the federation Project.
	_, repoID, err := deps.FetchOwnerAndRepoIDs(targetOwner, targetRepo)
	if err != nil {
		return fmt.Errorf("resolving %s node IDs: %w", ownerRepo, err)
	}
	if err := deps.LinkRepoToProject(projectID, repoID); err != nil {
		return fmt.Errorf("linking %s to the project: %w", ownerRepo, err)
	}
	fmt.Fprintf(w, "  %s  Linked %s to the project\n", ui.StatusOK.Render("✓"), ownerRepo)

	// Register the repo by setting AGENTIC_PROJECT_ID on the target domain repo.
	// No framework mount — domain repos are pure code.
	if err := deps.SetRepoVariable(targetOwner, targetRepo, ProjectVarName, projectID); err != nil {
		return fmt.Errorf("setting %s on %s: %w", ProjectVarName, ownerRepo, err)
	}
	fmt.Fprintf(w, "  %s  Set %s on %s — no framework mounted (domain repos are pure code)\n", ui.StatusOK.Render("✓"), ProjectVarName, ownerRepo)
	return nil
}

// ProjectDisplayName returns the human-readable project title for projectID,
// falling back to the raw ID if the title cannot be fetched.
func ProjectDisplayName(deps Deps, projectID string) string {
	if deps.FetchProjectTitle != nil {
		if title, err := deps.FetchProjectTitle(projectID); err == nil && title != "" {
			return title
		}
	}
	return projectID
}

// Unlink removes this repository's project affiliation by deleting AGENTIC_PROJECT_ID.
func Unlink(w io.Writer, deps Deps) error {
	guard, err := EvalUnlinkGuard(deps)
	if err != nil {
		return fmt.Errorf("evaluating unlink guard: %w", err)
	}

	switch guard.Guard {
	case JoinGuardBlocked:
		return fmt.Errorf("blocked: %s", guard.Message)

	case JoinGuardClear:
		fmt.Fprintf(w, "  %s  %s\n\n", ui.StatusOK.Render("✓"), guard.Message)
		return nil

	case JoinGuardWarnConfirm:
		fmt.Fprintf(w, "  %s  %s\n", ui.StatusWarning.Render("⚠"), guard.Message)
		ok, err := deps.Confirm("Proceed?")
		if err != nil {
			return fmt.Errorf("confirmation failed: %w", err)
		}
		if !ok {
			return fmt.Errorf("aborted by user")
		}
	}

	if err := deps.DeleteRepoVariable(deps.Owner, deps.RepoName, ProjectVarName); err != nil {
		return fmt.Errorf("deleting %s: %w", ProjectVarName, err)
	}
	fmt.Fprintf(w, "  %s  %s removed\n", ui.StatusOK.Render("✓"), ProjectVarName)

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Repository removed from agentic project"))
	return nil
}
