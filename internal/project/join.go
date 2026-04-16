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
// It evaluates the guard, handles warnings and confirmation, then calls joinWork.
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
	return joinWork(w, deps, projectID)
}

// JoinConfirmed affiliates this repository with a project, skipping the guard evaluation.
// Use this when the caller has already evaluated and confirmed any warnings (e.g. the
// interactive CLI path that shows the warning before a project selection UI).
func JoinConfirmed(w io.Writer, deps Deps, projectID string) error {
	// Still handle the same-project case — no-op if already affiliated.
	currentID, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if currentID == projectID {
		name := ProjectDisplayName(deps, projectID)
		fmt.Fprintf(w, "  %s  this repo is already part of agentic project %s\n\n", ui.StatusOK.Render("✓"), name)
		return nil
	}
	return joinWork(w, deps, projectID)
}

// joinWork performs the actual join steps: setting the variable and mounting the framework.
func joinWork(w io.Writer, deps Deps, projectID string) error {
	// Resolve project name for display (best-effort — fall back to ID on error).
	projectName := ProjectDisplayName(deps, projectID)

	// Set AGENTIC_PROJECT_ID.
	fmt.Fprintf(w, "  Setting %s...\n", ProjectVarName)
	if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName, projectID); err != nil {
		return fmt.Errorf("setting %s: %w", ProjectVarName, err)
	}
	fmt.Fprintf(w, "  %s  %s set\n", ui.StatusOK.Render("✓"), ProjectVarName)

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
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Joined project \""+projectName+"\""))
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
