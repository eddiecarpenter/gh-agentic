package init

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
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
	FetchReleases   mount.FetchReleasesFunc
	// Topology, when non-empty, pre-seeds cfg.Topology and suppresses the
	// topology select in collectVersionTopology. The CLI captures topology
	// first (single vs. federated routing decision) and passes it here so
	// the form does not ask a second time.
	Topology string
}

// DefaultFormDeps returns production dependencies for the interactive form.
func DefaultFormDeps() FormDeps {
	return FormDeps{
		RunForm: DefaultFormRun,
		DetectOwnerType: func(owner string) (string, error) {
			return auth.OwnerTypeUser, nil // fallback; real detection below
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
			cfg.OwnerType = auth.OwnerTypeUser
		} else {
			cfg.OwnerType = ownerType
			fmt.Fprintf(w, "  %s Owner type: %s\n", ui.SymbolInfo, ownerType)
		}
	} else {
		cfg.OwnerType = auth.OwnerTypeUser
	}
	fmt.Fprintln(w)

	// --- Pre-seed topology when the caller already captured it ---
	if deps.Topology != "" {
		cfg.Topology = deps.Topology
	}

	// --- Fetch available releases for version dropdown ---
	var releases []mount.Release
	if deps.FetchReleases != nil {
		var fetchErr error
		releases, fetchErr = deps.FetchReleases(mount.FrameworkRepo)
		if fetchErr != nil {
			fmt.Fprintf(w, "  %s\n", ui.RenderWarning("Could not fetch releases — enter version manually"))
		}
	}

	// --- Phase 1: Version and Topology ---
	if err := collectVersionTopology(cfg, releases, deps.RunForm); err != nil {
		return nil, err
	}

	// --- Phase 2: Stack selection ---
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
// If releases are provided, the version is presented as a dropdown; otherwise
// falls back to a free-text input.
func collectVersionTopology(cfg *InitConfig, releases []mount.Release, runForm FormRunFunc) error {
	var versionField huh.Field
	if len(releases) > 0 {
		// Pre-select the latest release.
		if cfg.Version == "" {
			cfg.Version = releases[0].TagName
		}
		opts := make([]huh.Option[string], len(releases))
		for i, r := range releases {
			opts[i] = huh.NewOption(r.TagName, r.TagName)
		}
		versionField = huh.NewSelect[string]().
			Title("Framework version").
			Description("Select the gh-agentic release to mount").
			Options(opts...).
			Height(10).
			Value(&cfg.Version)
	} else {
		versionField = huh.NewInput().
			Title("Framework version").
			Description("The gh-agentic release tag to mount (e.g. v2.0.0)").
			Value(&cfg.Version).
			Validate(validateVersion)
	}

	// Include the topology select only when the caller has not pre-seeded it.
	// The CLI captures topology first (to route single vs. federated) and
	// passes it through FormDeps.Topology so the user is not asked twice.
	var fields []huh.Field
	fields = append(fields, versionField)
	if cfg.Topology == "" {
		fields = append(fields, huh.NewSelect[string]().
			Title("Project topology").
			Description("How your project repos are structured").
			Options(
				huh.NewOption("Single      — one repo, everything in one place", "Single"),
				huh.NewOption("Federated   — separate control plane + domain repos", "Federated"),
			).
			Value(&cfg.Topology))
	}

	form := huh.NewForm(huh.NewGroup(fields...))
	if err := runForm(form); err != nil {
		return fmt.Errorf("version/topology form: %w", err)
	}
	return nil
}

// collectStackAndAgent collects the stack selection.
// Agent identity is set automatically from the GitHub App — no user prompt needed.
func collectStackAndAgent(cfg *InitConfig, runForm FormRunFunc) error {
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Stack (select all that apply)").
			Description("The primary technology stack(s) for this project").
			Options(stackOptions()...).
			Value(&cfg.Stacks).
			Validate(validateStackSelection),
	))
	if err := runForm(form); err != nil {
		return fmt.Errorf("stack form: %w", err)
	}
	return nil
}

// collectPipelineConfig collects runner label, agent provider, and model.
func collectPipelineConfig(cfg *InitConfig, runForm FormRunFunc) error {
	if cfg.RunnerLabel == "" {
		cfg.RunnerLabel = RunnerDefaultForTopology(cfg.Topology, cfg.Owner)
	}
	if cfg.AgentProvider == "" {
		cfg.AgentProvider = DefaultAgentProvider
	}
	if cfg.AgentModel == "" {
		cfg.AgentModel = DefaultAgentModel
	}

	// Phase 3a: Runner select (mirrors bootstrap runner selection).
	runnerOpts := BuildRunnerOptions(cfg.RepoName, cfg.Owner)
	runnerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Runner label").
				Description("The GitHub Actions runner label for the agentic pipeline").
				Options(runnerOpts...).
				Value(&cfg.RunnerLabel),
		),
	)
	if err := runForm(runnerForm); err != nil {
		return fmt.Errorf("runner form: %w", err)
	}

	// If "other" selected, prompt for custom label.
	if cfg.RunnerLabel == RunnerOther {
		cfg.RunnerLabel = ""
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Custom runner label").
					Description("Enter your custom GitHub Actions runner label").
					Value(&cfg.RunnerLabel).
					Validate(validateRequired("runner label")),
			),
		)
		if err := runForm(customForm); err != nil {
			return fmt.Errorf("custom runner form: %w", err)
		}
	}

	// Phase 3b: Provider and model.
	providerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Agent provider").
				Description("The LLM provider the agent will use").
				Value(&cfg.AgentProvider),
			huh.NewInput().
				Title("Agent model").
				Description("The model the agent will use — leave as 'default' unless specific").
				Value(&cfg.AgentModel),
		),
	)
	if err := runForm(providerForm); err != nil {
		return fmt.Errorf("pipeline config form: %w", err)
	}
	return nil
}

// collectCredentialsAndProject collects the PROJECT_PAT and PIPELINE_PAT.
// Claude credentials are no longer prompted here — they are read automatically
// from the local machine (~/.claude/.credentials.json or macOS Keychain) after
// init completes, using the same mechanism as 'gh agentic auth refresh'. If
// credentials are not found locally, the user is prompted to run
// 'gh agentic auth refresh' manually.
//
// The AGENTIC_PROJECT_ID is never prompted here — for single topology it is
// created automatically by project.Create; for federated topology it is
// selected from the project list by the CLI before CollectConfigInteractive
// is called. Both paths set the variable through their own code paths.
func collectCredentialsAndProject(cfg *InitConfig, runForm FormRunFunc) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("PROJECT_PAT").
				Description("Personal Access Token for Projects v2 mutations (scopes: repo, project, read:org)").
				Value(&cfg.GooseAgentPAT).
				EchoMode(huh.EchoModePassword),
			huh.NewInput().
				Title("PIPELINE_PAT").
				Description("Fine-grained PAT for pipeline trigger operations (permissions: Issues: write, Pull requests: write, Secrets: write)").
				Value(&cfg.PipelinePAT).
				EchoMode(huh.EchoModePassword),
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
