package project

import (
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// SwitchProject moves an already-initialised repo to a different project.
// Blocked if the repo does not have AGENTIC_PROJECT_ID set (not initialised).
func SwitchProject(w io.Writer, deps Deps, newProjectID string) error {
	// Guard: must be already initialised.
	currentID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || currentID == "" {
		return fmt.Errorf("this repo is not affiliated with a project — run 'gh agentic project init' first")
	}
	if currentID == newProjectID {
		fmt.Fprintf(w, "  %s  this repo is already part of agentic project %s\n\n", ui.StatusOK.Render("✓"), ProjectDisplayName(deps, newProjectID))
		return nil
	}
	// Re-point AGENTIC_PROJECT_ID. No framework mount — switching membership is
	// metadata only (domain repos are pure code; the control plane carries the mount).
	if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName, newProjectID); err != nil {
		return fmt.Errorf("setting %s: %w", ProjectVarName, err)
	}
	fmt.Fprintf(w, "  %s  Moved to agentic project %s\n\n", ui.StatusOK.Render("✓"), ProjectDisplayName(deps, newProjectID))
	return nil
}

// SwitchVersionPreflight holds the pre-resolved data from preflight checks,
// avoiding duplicate API calls when SwitchVersion runs immediately after.
type SwitchVersionPreflight struct {
	CurrentVersion string
	IsFederatedCP  bool
}

// PreflightSwitchVersion validates that a version switch can proceed without
// producing any output. Returns pre-resolved data so SwitchVersion can skip
// repeating the same API calls.
func PreflightSwitchVersion(deps Deps, version string) (SwitchVersionPreflight, error) {
	projectID, err := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if err != nil || projectID == "" {
		return SwitchVersionPreflight{}, fmt.Errorf("this repo is not part of an agentic project — run 'gh agentic project init' first")
	}

	linked, err := deps.FetchLinkedRepos(projectID)
	if err != nil {
		return SwitchVersionPreflight{}, fmt.Errorf("determining topology: %w", err)
	}
	topo := DetectTopology(deps.RepoFullName, linked)
	if topo == TopologyFederated {
		cp, _ := ControlPlaneRepo(linked)
		return SwitchVersionPreflight{}, fmt.Errorf("version switching is only allowed on the control plane repo (%s) — run 'gh agentic project switch version %s' there", cp.NameWithOwner, version)
	}

	releases, err := deps.FetchReleases(mount.FrameworkRepo)
	if err != nil {
		return SwitchVersionPreflight{}, fmt.Errorf("fetching releases: %w", err)
	}
	if err := mount.ValidateTag(version, releases); err != nil {
		return SwitchVersionPreflight{}, err
	}

	currentVersion, _ := deps.ReadAIVersion(deps.Root)
	topology, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, TopologyVarName)

	return SwitchVersionPreflight{
		CurrentVersion: currentVersion,
		IsFederatedCP:  topology == "federated",
	}, nil
}

// SwitchVersion upgrades or downgrades the framework version using pre-resolved
// preflight data to avoid duplicate API calls.
//
// AGENTIC_FRAMEWORK_VERSION is the authoritative version the pipeline
// and `check`/`info` read. `create` (init) and `repair` both write it
// unconditionally — `SwitchVersion` does the same so upgrade leaves no
// drift between the mounted `.agents/` tree, the workflow file versions, and
// the repo variable. Single topology uses the variable locally; a
// federated control plane broadcasts it to domain repos. Either way it
// must be written.
func SwitchVersion(w io.Writer, deps Deps, version string, pre SwitchVersionPreflight, confirm func(string) (bool, error)) error {
	writeVariable := func() error {
		existing, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName)
		if existing == version {
			return nil
		}
		if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, FrameworkVersionVarName, version); err != nil {
			return fmt.Errorf("updating %s: %w", FrameworkVersionVarName, err)
		}
		suffix := ""
		if pre.IsFederatedCP {
			suffix = " — domain repos will sync on next mount"
		}
		fmt.Fprintf(w, "  %s  %s set to %s%s\n",
			ui.StatusOK.Render("✓"), FrameworkVersionVarName, version, suffix)
		return nil
	}

	if pre.CurrentVersion == version {
		fmt.Fprintf(w, "  %s  Framework is already at %s\n", ui.StatusOK.Render("✓"), version)
		// Reconcile wrapper workflow refs even when the mounted .agents/ already
		// matches the target. They can drift from the mount — e.g. a prior
		// upgrade's workflow-file edit was rejected (GitHub blocks workflow
		// pushes from runner tokens), or only the submodule pointer moved. The
		// mount-version short-circuit would otherwise leave a stale @version in
		// the workflow forever, since RunSwitch (the only other caller of
		// UpdateWorkflowVersions) runs only on an actual version change.
		// Idempotent: a no-op when the refs already point at `version`.
		n, err := mount.UpdateWorkflowVersionsCount(deps.Root, version)
		if err != nil {
			return fmt.Errorf("reconciling workflow versions: %w", err)
		}
		if n > 0 {
			fmt.Fprintf(w, "  %s  Reconciled %d wrapper workflow(s) to @%s\n", ui.StatusOK.Render("✓"), n, version)
		}
		return writeVariable()
	}

	// Switch framework version (mount the new version).
	if err := mount.RunSwitch(w, deps.Root, pre.CurrentVersion, version, deps.Clone, confirm); err != nil {
		return err
	}

	return writeVariable()
}

// switchProjectListFederated returns only the federated projects available to the owner.
// It checks each project's control-plane repo for AGENTIC_TOPOLOGY=federated.
func switchProjectListFederated(deps Deps) ([]ProjectInfo, error) {
	ownerType, err := deps.DetectOwnerType(deps.Owner)
	if err != nil {
		ownerType = "User"
	}

	all, err := deps.FetchProjectsForOwner(deps.Owner, ownerType)
	if err != nil {
		return nil, fmt.Errorf("fetching projects: %w", err)
	}

	var federated []ProjectInfo
	for _, p := range all {
		linked, err := deps.FetchLinkedRepos(p.ID)
		if err != nil {
			continue
		}
		cp, ok := ControlPlaneRepo(linked)
		if !ok {
			continue
		}
		parts := strings.SplitN(cp.NameWithOwner, "/", 2)
		if len(parts) != 2 {
			continue
		}
		topo, _ := deps.GetRepoVariable(parts[0], parts[1], TopologyVarName)
		if topo == "federated" {
			federated = append(federated, p)
		}
	}
	return federated, nil
}
