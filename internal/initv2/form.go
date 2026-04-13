package initv2

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// FormRunFunc is a function that runs a huh.Form. The production implementation
// simply calls f.Run(). Tests inject a fake that sets form-bound values directly.
type FormRunFunc func(f *huh.Form) error

// DefaultFormRun is the production FormRunFunc — it delegates to huh.Form.Run().
var DefaultFormRun FormRunFunc = func(f *huh.Form) error { return f.Run() }

// DetectOwnerTypeFunc detects whether a GitHub owner is a personal account or an organisation.
type DetectOwnerTypeFunc func(owner string) (string, error)

// FormDeps holds injectable dependencies for the interactive form.
type FormDeps struct {
	RunForm         FormRunFunc
	RunCommand      RunCommandFunc
	DetectOwnerType DetectOwnerTypeFunc
}

// DefaultFormDeps returns production dependencies for the interactive form.
func DefaultFormDeps() FormDeps {
	return FormDeps{
		RunForm:    DefaultFormRun,
		DetectOwnerType: func(owner string) (string, error) {
			return bootstrap.OwnerTypeUser, nil // fallback; real detection below
		},
	}
}

// CollectConfigInteractive presents the huh interactive form and returns a
// populated InitConfig. It auto-detects the repo from git remote and the
// owner type from the GitHub API.
func CollectConfigInteractive(w io.Writer, repoFullName string, deps FormDeps) (*InitConfig, error) {
	cfg := &InitConfig{}

	// --- Auto-detect repo ---
	if repoFullName == "" {
		if deps.RunCommand != nil {
			detected, err := DetectRepoFromRemote(deps.RunCommand)
			if err == nil && detected != "" {
				repoFullName = detected
			}
		}
	}

	if repoFullName != "" {
		cfg.RepoFullName = repoFullName
		parts := splitOwnerRepo(repoFullName)
		cfg.Owner = parts[0]
		cfg.RepoName = parts[1]
		fmt.Fprintf(w, "  %s Detected repo: %s\n", ui.SymbolInfo, repoFullName)
	}

	// --- Auto-detect owner type ---
	if cfg.Owner != "" && deps.DetectOwnerType != nil {
		ownerType, err := deps.DetectOwnerType(cfg.Owner)
		if err != nil {
			fmt.Fprintf(w, "  %s\n", ui.RenderWarning("Could not detect owner type — defaulting to user"))
			cfg.OwnerType = bootstrap.OwnerTypeUser
		} else {
			cfg.OwnerType = ownerType
			fmt.Fprintf(w, "  %s Owner type: %s\n", ui.SymbolInfo, ownerType)
		}
	} else {
		cfg.OwnerType = bootstrap.OwnerTypeUser
	}
	fmt.Fprintln(w)

	// --- Phase 1: Version and Topology ---
	if err := collectVersionTopology(cfg, deps.RunForm); err != nil {
		return nil, err
	}

	// --- Phase 2: Stack and Agent configuration ---
	if err := collectStackAndAgent(cfg, deps.RunForm); err != nil {
		return nil, err
	}

	// --- Phase 3: Pipeline configuration ---
	if err := collectPipelineConfig(cfg, deps.RunForm); err != nil {
		return nil, err
	}

	// --- Phase 4: Credentials and Project ---
	if err := collectCredentialsAndProject(cfg, deps.RunForm); err != nil {
		return nil, err
	}

	return cfg, nil
}

// collectVersionTopology collects the framework version and project topology.
func collectVersionTopology(cfg *InitConfig, runForm FormRunFunc) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Framework version").
				Description("The gh-agentic release tag to mount (e.g. v2.0.0)").
				Value(&cfg.Version).
				Validate(validateVersion),
			huh.NewSelect[string]().
				Title("Project topology").
				Description("How your project repos are structured").
				Options(
					huh.NewOption("Single      — one repo, everything in one place", "Single"),
					huh.NewOption("Federated   — separate control plane + domain repos", "Federated"),
				).
				Value(&cfg.Topology),
		),
	)
	if err := runForm(form); err != nil {
		return fmt.Errorf("version/topology form: %w", err)
	}
	return nil
}

// collectStackAndAgent collects stacks, agent user, and agent user scope.
func collectStackAndAgent(cfg *InitConfig, runForm FormRunFunc) error {
	var fields []huh.Field

	fields = append(fields,
		huh.NewMultiSelect[string]().
			Title("Stack (select all that apply)").
			Description("The primary technology stack(s) for this project").
			Options(stackOptions()...).
			Value(&cfg.Stacks).
			Validate(validateStackSelection),
	)

	fields = append(fields,
		huh.NewInput().
			Title("Agent GitHub username").
			Description("The GitHub account that will act as the AI agent").
			Value(&cfg.AgentUser).
			Validate(validateRequired("agent username")),
	)

	if cfg.OwnerType == bootstrap.OwnerTypeOrg {
		if cfg.AgentUserScope == "" {
			cfg.AgentUserScope = bootstrap.AgentUserScopeOrg
		}
		fields = append(fields,
			huh.NewSelect[string]().
				Title("AGENT_USER variable scope").
				Description("Where to store the AGENT_USER variable — org level shares it across repos").
				Options(
					huh.NewOption("Organisation level", bootstrap.AgentUserScopeOrg),
					huh.NewOption("Repository level", bootstrap.AgentUserScopeRepo),
				).
				Value(&cfg.AgentUserScope),
		)
	} else {
		cfg.AgentUserScope = bootstrap.AgentUserScopeRepo
	}

	if err := runForm(huh.NewForm(huh.NewGroup(fields...))); err != nil {
		return fmt.Errorf("stack/agent form: %w", err)
	}
	return nil
}

// collectPipelineConfig collects runner label, Goose provider, and model.
func collectPipelineConfig(cfg *InitConfig, runForm FormRunFunc) error {
	if cfg.RunnerLabel == "" {
		cfg.RunnerLabel = bootstrap.DefaultRunnerLabel
	}
	if cfg.GooseProvider == "" {
		cfg.GooseProvider = bootstrap.DefaultGooseProvider
	}
	if cfg.GooseModel == "" {
		cfg.GooseModel = bootstrap.DefaultGooseModel
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Runner label").
				Description("The GitHub Actions runner label for the agentic pipeline").
				Value(&cfg.RunnerLabel).
				Validate(validateRequired("runner label")),
			huh.NewInput().
				Title("Goose provider").
				Description("The LLM provider the agent will use").
				Value(&cfg.GooseProvider),
			huh.NewInput().
				Title("Goose model").
				Description("The model the agent will use — leave as 'default' unless specific").
				Value(&cfg.GooseModel),
		),
	)
	if err := runForm(form); err != nil {
		return fmt.Errorf("pipeline config form: %w", err)
	}
	return nil
}

// collectCredentialsAndProject collects PAT, Claude credentials, and project ID.
func collectCredentialsAndProject(cfg *InitConfig, runForm FormRunFunc) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("GOOSE_AGENT_PAT").
				Description("GitHub PAT for the agent user (scopes: repo, workflow, read:org)").
				Value(&cfg.GooseAgentPAT).
				EchoMode(huh.EchoModePassword),
			huh.NewInput().
				Title("Claude credentials JSON").
				Description("Base64-encoded Claude credentials (leave empty to skip)").
				Value(&cfg.ClaudeCreds).
				EchoMode(huh.EchoModePassword),
			huh.NewInput().
				Title("GitHub Project ID").
				Description("The GitHub Project number for pipeline tracking (e.g. PVT_123)").
				Value(&cfg.ProjectID),
		),
	)
	if err := runForm(form); err != nil {
		return fmt.Errorf("credentials form: %w", err)
	}
	return nil
}

// --- Validation functions ---

// validateVersion validates that a version string is not empty and starts with "v".
func validateVersion(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("version is required (e.g. v2.0.0)")
	}
	if !strings.HasPrefix(s, "v") {
		return errors.New("version must start with 'v' (e.g. v2.0.0)")
	}
	return nil
}

// validateStackSelection returns an error if no stacks are selected.
func validateStackSelection(selected []string) error {
	if len(selected) == 0 {
		return errors.New("at least one stack must be selected")
	}
	return nil
}

// validateRequired returns a validation function that ensures a non-empty string.
func validateRequired(fieldName string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", fieldName)
		}
		return nil
	}
}

// --- Helper functions ---

// stackOptions returns the selectable stack options for the form.
func stackOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Go", "Go"),
		huh.NewOption("Java — Quarkus", "Java Quarkus"),
		huh.NewOption("Java — Spring Boot", "Java Spring Boot"),
		huh.NewOption("TypeScript / Node.js", "TypeScript Node.js"),
		huh.NewOption("Python", "Python"),
		huh.NewOption("Rust", "Rust"),
		huh.NewOption("Other", "Other"),
	}
}

// splitOwnerRepo splits "owner/repo" into [owner, repo].
// Returns ["", ""] if the format is invalid.
func splitOwnerRepo(fullName string) [2]string {
	for i := 0; i < len(fullName); i++ {
		if fullName[i] == '/' {
			return [2]string{fullName[:i], fullName[i+1:]}
		}
	}
	return [2]string{fullName, ""}
}
