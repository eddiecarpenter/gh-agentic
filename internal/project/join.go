package project

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// Join affiliates this repository with an existing project as a domain repo.
// It does NOT link the repo to the GitHub Project (only the control plane should be
// linked in GitHub Project settings). It sets AGENTIC_PROJECT_ID and mounts the
// framework if not already present.
func Join(w io.Writer, deps Deps, projectID string) error {
	guard, err := EvalJoinGuard(deps, projectID)
	if err != nil {
		return fmt.Errorf("evaluating join guard: %w", err)
	}

	switch guard.Guard {
	case JoinGuardBlocked:
		return fmt.Errorf("blocked: %s", guard.Message)

	case JoinGuardSameProject:
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
	// JoinGuardClear falls through here.

	// Set AGENTIC_PROJECT_ID.
	fmt.Fprintf(w, "  Setting %s...\n", ProjectVarName)
	if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName, projectID); err != nil {
		return fmt.Errorf("setting %s: %w", ProjectVarName, err)
	}
	fmt.Fprintf(w, "  %s  %s set to %s\n", ui.StatusOK.Render("✓"), ProjectVarName, projectID)

	// Mount framework if not already present.
	aiDir := filepath.Join(deps.Root, ".ai")
	if _, err := os.Stat(aiDir); os.IsNotExist(err) {
		releases, err := deps.FetchReleases(mount.FrameworkRepo)
		if err != nil {
			return fmt.Errorf("fetching framework releases: %w", err)
		}
		if len(releases) == 0 {
			return fmt.Errorf("no framework releases found")
		}
		latest := releases[0].TagName
		fmt.Fprintf(w, "  Mounting framework %s...\n", latest)
		if err := mount.DownloadFramework(deps.Root, latest, deps.Clone); err != nil {
			return fmt.Errorf("mounting framework: %w", err)
		}
		fmt.Fprintf(w, "  %s  Framework mounted at %s\n", ui.StatusOK.Render("✓"), latest)
	} else {
		version, _ := deps.ReadAIVersion(deps.Root)
		if version != "" {
			fmt.Fprintf(w, "  %s  Framework already mounted at %s\n", ui.StatusOK.Render("✓"), version)
		}
	}

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Joined project "+projectID))
	return nil
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
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Repository unlinked from project"))
	return nil
}
