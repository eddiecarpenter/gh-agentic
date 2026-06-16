package project

import (
	"fmt"
	"io"

	initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// InitMode indicates whether the repo is being set up as a single-topology
// control plane or as a federated control plane.
type InitMode int

const (
	InitModeSingle    InitMode = iota // Create a single-topology control plane.
	InitModeFederated                 // Create a federated control plane.
)

// InitRepoConfig holds the collected configuration for project init.
type InitRepoConfig struct {
	Mode    InitMode
	InitCfg *initpkg.InitConfig // Full wizard config.
}

// InitRepo performs first-time setup for a repo.
// For single: creates a single-topology project + mount + configure secrets/variables.
// For federated: creates a federated control plane — project + an empty FEDERATION.md
// + federated-tier system docs + mount. Domain repos are registered from the control
// plane via 'gh agentic project join', not via init.
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
			if guardErr := EnsureFederatedOwnerIsOrg(TopologyStringFederation, deps.Owner, ownerType); guardErr != nil {
				return guardErr
			}
		}
	}

	switch cfg.Mode {
	case InitModeSingle:
		return initSingle(w, deps, cfg.InitCfg)
	case InitModeFederated:
		return initFederatedCP(w, deps, cfg.InitCfg)
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

// initFederatedCP creates a federated control plane: a project board, an empty
// FEDERATION.md + federated-tier system docs (both scaffolded by Create), the
// framework mount, and the standard agent / config files. Domain repos are
// registered from the control plane via 'gh agentic project join', not via init.
func initFederatedCP(w io.Writer, deps Deps, cfg *initpkg.InitConfig) error {
	fmt.Fprintln(w, "  Creating federated control plane...")
	fmt.Fprintln(w)

	createCfg := CreateConfig{
		Title:    deps.RepoName,
		Version:  cfg.Version,
		Topology: TopologyStringFederation,
	}
	if err := Create(w, deps, createCfg); err != nil {
		return fmt.Errorf("creating control plane: %w", err)
	}

	// Configure secrets, variables, and agent access.
	if cfg != nil && cfg.RepoFullName != "" {
		if err := initpkg.ConfigureRepo(w, cfg, deps.Run); err != nil {
			return fmt.Errorf("configuring repo: %w", err)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n\n", ui.StatusOK.Render("Repository initialised as federated control plane"))
	return nil
}
