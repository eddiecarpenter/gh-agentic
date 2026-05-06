package project

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// InitMode indicates whether the repo is being set up as a single control
// plane or as a domain repo joining a federated project.
type InitMode int

const (
	InitModeSingle    InitMode = iota // Create new project, set up as control plane.
	InitModeFederated                 // Join existing federated project.
)

// InitRepoConfig holds the collected configuration for project init.
type InitRepoConfig struct {
	Mode      InitMode
	ProjectID string              // Federated mode only — selected project.
	InitCfg   *initpkg.InitConfig // Full wizard config.
}

// InitRepo performs first-time setup for a repo.
// For single: creates project + mount + configure secrets/variables.
// For federated: joins existing federated project + mount at CP version + configure.
func InitRepo(w io.Writer, deps Deps, cfg InitRepoConfig) error {
	// Guard: must not already be affiliated.
	existing, _ := deps.GetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName)
	if existing != "" {
		existingName := ProjectDisplayName(deps, existing)
		return fmt.Errorf("repo is already affiliated with project %q — use 'gh agentic project switch project' to change affiliation", existingName)
	}

	// Federated mode requires a GitHub Organization. Refuse before any
	// side effect (project mount, variable writes, ConfigureRepo). Owner
	// detection is best-effort — if it fails, the per-site guards in
	// Create / repairTopologyVars are the last line of defence.
	if cfg.Mode == InitModeFederated {
		if ownerType, otErr := deps.DetectOwnerType(deps.Owner); otErr == nil {
			if guardErr := EnsureFederatedOwnerIsOrg("federated", deps.Owner, ownerType); guardErr != nil {
				return guardErr
			}
		}
	}

	switch cfg.Mode {
	case InitModeSingle:
		return initSingle(w, deps, cfg.InitCfg)
	case InitModeFederated:
		return initFederated(w, deps, cfg)
	default:
		return fmt.Errorf("unknown init mode")
	}
}

// initSingle creates a new project and sets up this repo as the control plane.
func initSingle(w io.Writer, deps Deps, cfg *initpkg.InitConfig) error {
	fmt.Fprintln(w, "  Setting up single control plane...")
	fmt.Fprintln(w)

	// Build create config. Single topology is declared here so Create writes
	// AGENTIC_TOPOLOGY=single directly and the federated-owner guard does
	// not fire on user-owned single-topology setups.
	createCfg := CreateConfig{
		Title:    deps.RepoName,
		Version:  cfg.Version,
		Topology: "single",
	}

	if err := Create(w, deps, createCfg); err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	// Configure secrets, variables, and agent access.
	if cfg != nil && cfg.RepoFullName != "" {
		if err := initpkg.ConfigureRepo(w, cfg, deps.Run); err != nil {
			return fmt.Errorf("configuring repo: %w", err)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Repository initialised as single control plane"))
	return nil
}

// initFederated joins this repo to an existing federated project and configures it.
func initFederated(w io.Writer, deps Deps, cfg InitRepoConfig) error {
	fmt.Fprintln(w, "  Joining federated project...")
	fmt.Fprintln(w)

	// Get the control plane version.
	linked, err := deps.FetchLinkedRepos(cfg.ProjectID)
	if err != nil {
		return fmt.Errorf("fetching linked repos: %w", err)
	}
	cp, ok := ControlPlaneRepo(linked)
	if !ok {
		return fmt.Errorf("no control plane found for this project")
	}
	parts := strings.SplitN(cp.NameWithOwner, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("cannot parse control plane repo name: %s", cp.NameWithOwner)
	}

	cpVersion, err := deps.GetRepoVariable(parts[0], parts[1], FrameworkVersionVarName)
	if err != nil || cpVersion == "" {
		// Fall back to latest release.
		releases, relErr := deps.FetchReleases(mount.FrameworkRepo)
		if relErr != nil || len(releases) == 0 {
			return fmt.Errorf("cannot determine framework version from control plane and no releases available")
		}
		cpVersion = releases[0].TagName
		fmt.Fprintf(w, "  %s  Control plane version not set — using latest: %s\n", ui.StatusWarning.Render("⚠"), cpVersion)
	} else {
		fmt.Fprintf(w, "  %s  Control plane framework version: %s\n", ui.StatusOK.Render("✓"), cpVersion)
	}

	// Set AGENTIC_PROJECT_ID.
	fmt.Fprintf(w, "  Setting %s...\n", ProjectVarName)
	if err := deps.SetRepoVariable(deps.Owner, deps.RepoName, ProjectVarName, cfg.ProjectID); err != nil {
		return fmt.Errorf("setting %s: %w", ProjectVarName, err)
	}
	fmt.Fprintf(w, "  %s  %s set\n", ui.StatusOK.Render("✓"), ProjectVarName)

	// Mount the framework at the control plane version. The mounted version
	// is tracked via .agents/.git metadata — no flat .ai-version file is
	// written (removed by feature #571 / task #585).
	aiDir := filepath.Join(deps.Root, ".agents")
	if _, err := os.Stat(aiDir); os.IsNotExist(err) {
		fmt.Fprintf(w, "  Mounting framework %s...\n", cpVersion)
		if err := mount.DownloadFramework(deps.Root, cpVersion, deps.Clone); err != nil {
			return fmt.Errorf("mounting framework: %w", err)
		}
		fmt.Fprintf(w, "  %s  Framework mounted at %s\n", ui.StatusOK.Render("✓"), cpVersion)
	} else {
		localVersion, _ := mount.ReadAIVersionFromGit(deps.Root)
		if localVersion == "" {
			localVersion = "(unknown)"
		}
		fmt.Fprintf(w, "  %s  Framework already mounted at %s\n", ui.StatusOK.Render("✓"), localVersion)
	}

	// Configure secrets, variables, and agent access.
	if cfg.InitCfg != nil && cfg.InitCfg.RepoFullName != "" {
		if err := initpkg.ConfigureRepo(w, cfg.InitCfg, deps.Run); err != nil {
			return fmt.Errorf("configuring repo: %w", err)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Repository initialised as federated domain repo"))
	return nil
}

// ListFederatedProjects returns only projects whose control plane has AGENTIC_TOPOLOGY=federated.
func ListFederatedProjects(deps Deps) ([]ProjectInfo, error) {
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
