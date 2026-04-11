package bootstrap

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// FormRunFunc is a function that runs a huh.Form. The production implementation
// simply calls f.Run(). Tests inject a fake that sets form-bound values directly.
type FormRunFunc func(f *huh.Form) error

// DefaultFormRun is the production FormRunFunc — it delegates to huh.Form.Run().
var DefaultFormRun FormRunFunc = func(f *huh.Form) error { return f.Run() }

// ErrAborted is returned by RunForm when the user declines the final "Create project?" confirm.
var ErrAborted = errors.New("aborted by user")

// ErrFederatedRequiresOrg is returned when the user selects Federated topology with a personal account.
var ErrFederatedRequiresOrg = errors.New("federated topology requires an org account")

// Owner represents a selectable GitHub owner (personal account or organisation).
type Owner struct {
	// Login is the GitHub account or org login.
	Login string
	// Label is the display string shown in the form select list.
	Label string
}

// FetchOwnersFunc fetches the list of available GitHub owners.
// Injected so tests can substitute a fake implementation without real gh auth.
type FetchOwnersFunc func() ([]Owner, error)

// ghUserResp is the API response shape for GET /user.
type ghUserResp struct {
	Login string `json:"login"`
}

// ghOrgResp is the API response shape for one element in GET /user/orgs.
type ghOrgResp struct {
	Login string `json:"login"`
}

// ghRepoListResp is the API response shape for one element in GET /orgs/{org}/repos.
type ghRepoListResp struct {
	ID int `json:"id"`
}

// Repo represents a GitHub repository returned by FetchReposFunc.
type Repo struct {
	// Name is the short name of the repository (e.g. "my-project").
	Name string
	// FullName is the owner-qualified name (e.g. "alice/my-project").
	FullName string
}

// ghRepoResp is the API response shape for one element in the repos list endpoint.
type ghRepoResp struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
}

// FetchReposFunc fetches the list of repositories owned by a given GitHub owner.
// Injected so tests can substitute a fake implementation without real gh auth.
type FetchReposFunc func(owner string) ([]Repo, error)

// DefaultFetchRepos fetches all repositories for the given owner using the
// authenticated go-gh/v2 REST client. It handles pagination (GitHub returns
// max 100 per page) and returns results sorted alphabetically by name.
func DefaultFetchRepos(owner string) ([]Repo, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("creating GitHub API client: %w", err)
	}

	// Detect whether the owner is a user or org to choose the correct endpoint.
	var ownerResp struct {
		Type string `json:"type"`
	}
	if err := client.Get(fmt.Sprintf("users/%s", owner), &ownerResp); err != nil {
		return nil, fmt.Errorf("detecting owner type for %q: %w", owner, err)
	}

	var allRepos []Repo
	page := 1
	for {
		var endpoint string
		if ownerResp.Type == OwnerTypeOrg {
			endpoint = fmt.Sprintf("orgs/%s/repos?per_page=100&page=%d", owner, page)
		} else {
			// Use the authenticated endpoint so private repos are included.
			endpoint = fmt.Sprintf("user/repos?per_page=100&page=%d&affiliation=owner", page)
		}

		var pageRepos []ghRepoResp
		if err := client.Get(endpoint, &pageRepos); err != nil {
			return nil, fmt.Errorf("fetching repos for %q (page %d): %w", owner, page, err)
		}

		for _, r := range pageRepos {
			allRepos = append(allRepos, Repo{
				Name:     r.Name,
				FullName: r.FullName,
			})
		}

		// If we got fewer than 100, we've reached the last page.
		if len(pageRepos) < 100 {
			break
		}
		page++
	}

	// Sort alphabetically by name (case-insensitive).
	sort.Slice(allRepos, func(i, j int) bool {
		return strings.ToLower(allRepos[i].Name) < strings.ToLower(allRepos[j].Name)
	})

	return allRepos, nil
}

// CheckRepoExistsFunc checks whether a repository exists under a given owner.
// Returns true if the repo exists, false if it does not.
// Injected so tests can substitute a fake implementation without real gh auth.
type CheckRepoExistsFunc func(owner, name string) (bool, error)

// DefaultCheckRepoExists calls GET repos/{owner}/{name} via the authenticated
// go-gh/v2 REST client and returns whether the repository exists.
func DefaultCheckRepoExists(owner, name string) (bool, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return false, fmt.Errorf("creating GitHub API client: %w", err)
	}

	var resp struct{}
	if err := client.Get(fmt.Sprintf("repos/%s/%s", owner, name), &resp); err != nil {
		// A 404 means the repo does not exist — that's the expected "available" case.
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			return false, nil
		}
		return false, fmt.Errorf("checking repo existence for %s/%s: %w", owner, name, err)
	}

	return true, nil
}

// validateNewRepoName combines format validation (via validateProjectName) and
// existence checking (via CheckRepoExistsFunc). It returns an error if the name
// format is invalid or if the repo already exists under the given owner.
// This is used as the Validate func on the "Create new repo" input field.
func validateNewRepoName(owner string, checkExists CheckRepoExistsFunc) func(string) error {
	return func(name string) error {
		// First validate the name format.
		if err := validateProjectName(name); err != nil {
			return err
		}

		// Then check whether the repo already exists.
		exists, err := checkExists(owner, name)
		if err != nil {
			return fmt.Errorf("unable to verify repo name: %w", err)
		}
		if exists {
			return fmt.Errorf("repository %s/%s already exists", owner, name)
		}

		return nil
	}
}

// DefaultFetchOwners fetches owners using the authenticated go-gh/v2 REST client.
// It returns the personal account first, followed by orgs sorted alphabetically.
// Each org is annotated with "✔ clean" or "⚠ has repos".
func DefaultFetchOwners() ([]Owner, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("creating GitHub API client: %w", err)
	}

	// Fetch personal account.
	var user ghUserResp
	if err := client.Get("user", &user); err != nil {
		return nil, fmt.Errorf("fetching authenticated user: %w", err)
	}

	// Fetch orgs.
	var orgs []ghOrgResp
	if err := client.Get("user/orgs?per_page=100", &orgs); err != nil {
		return nil, fmt.Errorf("fetching user orgs: %w", err)
	}

	// Sort orgs alphabetically by login.
	sort.Slice(orgs, func(i, j int) bool {
		return strings.ToLower(orgs[i].Login) < strings.ToLower(orgs[j].Login)
	})

	owners := []Owner{
		{Login: user.Login, Label: user.Login + "  (personal)"},
	}

	for _, org := range orgs {
		// Check whether the org has any repos by fetching up to 1.
		var repos []ghRepoListResp
		path := fmt.Sprintf("orgs/%s/repos?per_page=1", org.Login)
		if err := client.Get(path, &repos); err != nil {
			// If we cannot check, assume it has repos (safer).
			owners = append(owners, Owner{
				Login: org.Login,
				Label: org.Login + "  " + ui.StatusWarning.Render("⚠ has repos"),
			})
			continue
		}

		var annotation string
		if len(repos) > 0 {
			annotation = ui.StatusWarning.Render("⚠ has repos")
		} else {
			annotation = ui.StatusOK.Render("✔ clean")
		}
		owners = append(owners, Owner{
			Login: org.Login,
			Label: org.Login + "  " + annotation,
		})
	}

	return owners, nil
}

// ValidateProjectName returns an error if s is not a valid project name.
// Valid names are non-empty, lowercase, and contain only letters, digits, and hyphens.
func ValidateProjectName(s string) error {
	return validateProjectName(s)
}

// validateProjectName returns an error if s is not a valid project name.
// Valid names are non-empty, lowercase, and contain only letters, digits, and hyphens.
func validateProjectName(s string) error {
	if s == "" {
		return errors.New("project name cannot be empty")
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return fmt.Errorf("project name must be lowercase with hyphens only (got %q)", s)
		}
	}
	return nil
}

// stackOptions are the selectable stacks.
var stackOptions = []huh.Option[string]{
	huh.NewOption("Go", "Go"),
	huh.NewOption("Java — Quarkus", "Java Quarkus"),
	huh.NewOption("Java — Spring Boot", "Java Spring Boot"),
	huh.NewOption("TypeScript / Node.js", "TypeScript Node.js"),
	huh.NewOption("Python", "Python"),
	huh.NewOption("Rust", "Rust"),
	huh.NewOption("Other", "Other"),
}

// ValidateStackSelection returns an error if no stacks are selected.
func ValidateStackSelection(selected []string) error {
	return validateStackSelection(selected)
}

// validateStackSelection returns an error if no stacks are selected.
func validateStackSelection(selected []string) error {
	if len(selected) == 0 {
		return errors.New("at least one stack must be selected")
	}
	return nil
}

// ValidateTopologyOwner checks whether the selected topology is valid for the given owner type.
// Returns ErrFederatedRequiresOrg if a personal account selects Federated topology.
// Returns nil for all other combinations.
func ValidateTopologyOwner(topology, ownerType string) error {
	return validateTopologyOwner(topology, ownerType)
}

// validateTopologyOwner checks whether the selected topology is valid for the given owner type.
// Returns ErrFederatedRequiresOrg if a personal account selects Federated topology.
// Returns nil for all other combinations (including personal + Single, which is valid but triggers a warning).
func validateTopologyOwner(topology, ownerType string) error {
	if ownerType == OwnerTypeUser && topology == "Federated" {
		return ErrFederatedRequiresOrg
	}
	return nil
}

// repoModeSelectExisting is the value for the "Select existing repo" choice.
const repoModeSelectExisting = "existing"

// repoModeCreateNew is the value for the "Create new repo" choice.
const repoModeCreateNew = "new"

// runnerOther is the sentinel value for the "other" runner option.
const runnerOther = "__other__"

// RunnerDefaultForTopology returns the smart default runner label based on topology.
// Single topology defaults to "ubuntu-latest"; Federated defaults to the org name.
func RunnerDefaultForTopology(topology, owner string) string {
	if topology == "Federated" {
		return owner
	}
	return DefaultRunnerLabel
}

// buildRunnerOptions builds the runner select options with dynamic repo and org names.
func buildRunnerOptions(projectName, owner string) []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("ubuntu-latest — GitHub-hosted runner", DefaultRunnerLabel),
		huh.NewOption(projectName+" — Selfhosted ARC queue", projectName),
		huh.NewOption(owner+" — Selfhosted ARC queue", owner),
		huh.NewOption("self-hosted — Self-hosted runner (not production)", "self-hosted"),
		huh.NewOption("other — enter a custom label", runnerOther),
	}
}

// resolveTemplateRepo sets cfg.TemplateRepo from the CLI flag or an interactive prompt.
// If templateFlag is non-empty it is used directly; otherwise the user is prompted
// with DefaultTemplateRepo pre-filled.
func resolveTemplateRepo(templateFlag string, cfg *BootstrapConfig, runForm FormRunFunc) error {
	if templateFlag != "" {
		cfg.TemplateRepo = templateFlag
		return nil
	}
	cfg.TemplateRepo = DefaultTemplateRepo
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Template repo").
				Description("The upstream template that provides the agentic framework").
				Value(&cfg.TemplateRepo),
		),
	)
	if err := runForm(form); err != nil {
		return fmt.Errorf("template repo form: %w", err)
	}
	return nil
}

// runPhase1TopologyOwner presents the topology and owner selection forms, then
// validates the combination. Returns the detected owner type on success.
func runPhase1TopologyOwner(w io.Writer, cfg *BootstrapConfig, fetchOwners FetchOwnersFunc, detectOwnerType DetectOwnerTypeFunc, runForm FormRunFunc) (string, error) {
	topologyForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select project topology").
				Description("How your project repos are structured").
				Options(
					huh.NewOption("Single      — one repo, control plane and project in one place", "Single"),
					huh.NewOption("Federated   — separate control plane + domain/tool repos", "Federated"),
				).
				Value(&cfg.Topology),
		),
	)
	if err := runForm(topologyForm); err != nil {
		return "", fmt.Errorf("topology form: %w", err)
	}

	owners, err := fetchOwners()
	if err != nil {
		return "", fmt.Errorf("fetching owner list: %w", err)
	}

	ownerOpts := make([]huh.Option[string], len(owners))
	for i, o := range owners {
		ownerOpts[i] = huh.NewOption(o.Label, o.Login)
	}

	ownerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where should the repo be created?").
				Description("The GitHub account or organisation that will own the repo").
				Options(ownerOpts...).
				Value(&cfg.Owner),
		),
	)
	if err := runForm(ownerForm); err != nil {
		return "", fmt.Errorf("owner form: %w", err)
	}

	ownerType, err := detectOwnerType(cfg.Owner)
	if err != nil {
		return "", fmt.Errorf("detecting owner type: %w", err)
	}

	if valErr := validateTopologyOwner(cfg.Topology, ownerType); valErr != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.RenderError("Federated topology requires an org account"))
		fmt.Fprintln(w, "  "+ui.Muted.Render("Personal accounts cannot host federated agentic environments."))
		fmt.Fprintln(w, "  "+ui.Muted.Render("Choose Single topology or select an org as the owner."))
		fmt.Fprintln(w)
		return "", ErrFederatedRequiresOrg
	}

	return ownerType, nil
}

// runPhase2RepoSelection presents the repo mode selector and, depending on the
// choice, either a filterable list of existing repos or a new-name input.
// ESC on the existing-repo list loops back to the mode selector.
func runPhase2RepoSelection(cfg *BootstrapConfig, fetchRepos FetchReposFunc, checkRepoExists CheckRepoExistsFunc, runForm FormRunFunc) error {
	for {
		var repoMode string
		repoModeForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Repository").
					Description("Choose whether to bootstrap into an existing repo or create a new one").
					Options(
						huh.NewOption("Select existing repo", repoModeSelectExisting),
						huh.NewOption("Create new repo", repoModeCreateNew),
					).
					Value(&repoMode),
			),
		)
		if err := runForm(repoModeForm); err != nil {
			return fmt.Errorf("repo mode form: %w", err)
		}

		if repoMode == repoModeSelectExisting {
			cfg.ExistingRepo = true

			repos, err := fetchRepos(cfg.Owner)
			if err != nil {
				return fmt.Errorf("fetching repo list: %w", err)
			}
			if len(repos) == 0 {
				return fmt.Errorf("no repositories found for %s — use 'Create new repo' instead", cfg.Owner)
			}

			repoOpts := make([]huh.Option[string], 0, len(repos))
			for _, r := range repos {
				repoOpts = append(repoOpts, huh.NewOption(r.Name, r.Name))
			}

			repoSelectForm := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title(fmt.Sprintf("Select repository (%d available)", len(repos))).
						Description("Type to filter. Press ESC to go back and create a new repo instead.").
						Options(repoOpts...).
						Filtering(true).
						Height(len(repoOpts)).
						Value(&cfg.ProjectName),
				),
			)
			if err := runForm(repoSelectForm); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					cfg.ProjectName = ""
					continue // ESC — loop back to repo mode choice
				}
				return fmt.Errorf("repo select form: %w", err)
			}
		} else {
			cfg.ExistingRepo = false

			repoNameForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Repository name").
						Description("Your new GitHub repository name — lowercase with hyphens only").
						Value(&cfg.ProjectName).
						Validate(validateNewRepoName(cfg.Owner, checkRepoExists)),
				),
			)
			if err := runForm(repoNameForm); err != nil {
				return fmt.Errorf("repo name form: %w", err)
			}
		}
		return nil
	}
}

// runPhase3Details presents the project details form: description (new repos only),
// stack selection, agent username, agent user scope (org accounts only), and Antora toggle.
func runPhase3Details(cfg *BootstrapConfig, ownerType string, runForm FormRunFunc) error {
	var fields []huh.Field

	if !cfg.ExistingRepo {
		fields = append(fields,
			huh.NewInput().
				Title("Description").
				Description("A short description shown on the GitHub repo page").
				Value(&cfg.Description),
		)
	}

	fields = append(fields,
		huh.NewMultiSelect[string]().
			Title("Stack (select all that apply)").
			Description("The primary technology stack(s) for this project").
			Options(stackOptions...).
			Value(&cfg.Stacks).
			Validate(validateStackSelection),
	)

	fields = append(fields,
		huh.NewInput().
			Title("Agent GitHub username").
			Description("The GitHub account that will act as the AI agent. This user must exist and will be granted write access to the repo.").
			Value(&cfg.AgentUser).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("agent username is required")
				}
				return nil
			}),
	)

	if ownerType == OwnerTypeOrg {
		if cfg.AgentUserScope == "" {
			cfg.AgentUserScope = AgentUserScopeOrg
		}
		fields = append(fields,
			huh.NewSelect[string]().
				Title("AGENT_USER variable scope").
				Description("Where to store the AGENT_USER variable — org level shares it across repos").
				Options(
					huh.NewOption("Organisation level", AgentUserScopeOrg),
					huh.NewOption("Repository level", AgentUserScopeRepo),
				).
				Value(&cfg.AgentUserScope),
		)
	} else {
		cfg.AgentUserScope = AgentUserScopeRepo
	}

	fields = append(fields,
		huh.NewConfirm().
			Title("Antora documentation site?").
			Description("Enable if this project will publish documentation via Antora").
			Value(&cfg.Antora),
	)

	if err := runForm(huh.NewForm(huh.NewGroup(fields...))); err != nil {
		return fmt.Errorf("project details form: %w", err)
	}
	return nil
}

// runPipelineConfig presents the pipeline configuration form: runner, provider,
// model, and GOOSE_AGENT_PAT. Defaults are set on the first pass only so that
// values the user entered on a previous loop iteration are preserved.
// If the user selects "other" for the runner, a follow-up free-text prompt is shown.
func runPipelineConfig(cfg *BootstrapConfig, runForm FormRunFunc) error {
	if cfg.RunnerLabel == "" {
		cfg.RunnerLabel = RunnerDefaultForTopology(cfg.Topology, cfg.Owner)
	}
	if cfg.GooseProvider == "" {
		cfg.GooseProvider = DefaultGooseProvider
	}
	if cfg.GooseModel == "" {
		cfg.GooseModel = DefaultGooseModel
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Runner label").
				Description("The GitHub Actions runner that will execute the agentic pipeline").
				Options(buildRunnerOptions(cfg.ProjectName, cfg.Owner)...).
				Value(&cfg.RunnerLabel),
			huh.NewInput().
				Title("Goose provider").
				Description("The LLM provider the agent will use").
				Value(&cfg.GooseProvider),
			huh.NewInput().
				Title("Goose model").
				Description("The model the agent will use — leave as 'default' unless you have a specific requirement").
				Value(&cfg.GooseModel),
			huh.NewInput().
				Title("GOOSE_AGENT_PAT").
				Description("A GitHub Personal Access Token for the agent user. Requires scopes: repo, workflow, read:org. Create one at: github.com/settings/tokens").
				Value(&cfg.GooseAgentPAT),
			huh.NewNote().
				Title("").
				Description(ui.Muted.Render("Press Enter to submit ↵")),
		),
	)
	if err := runForm(form); err != nil {
		return fmt.Errorf("pipeline config form: %w", err)
	}

	if cfg.RunnerLabel == runnerOther {
		cfg.RunnerLabel = ""
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Custom runner label").
					Description("Enter your custom GitHub Actions runner label").
					Value(&cfg.RunnerLabel).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("runner label cannot be empty")
						}
						return nil
					}),
			),
		)
		if err := runForm(customForm); err != nil {
			return fmt.Errorf("custom runner form: %w", err)
		}
	}
	return nil
}

// RunForm runs the multi-phase huh form, renders the summary box, and asks
// for final confirmation. Returns a populated BootstrapConfig.
//
// Answering "No" at the confirm step loops back to Phase 1 with all
// previously entered values pre-filled — the user only needs to edit the
// field they want to change. The loop exits only when the user confirms.
//
// templateFlag is the value of --template from the CLI. If non-empty it is
// used directly; otherwise an interactive prompt pre-filled with
// DefaultTemplateRepo is shown.
func RunForm(w io.Writer, fetchOwners FetchOwnersFunc, detectOwnerType DetectOwnerTypeFunc, fetchRepos FetchReposFunc, checkRepoExists CheckRepoExistsFunc, templateFlag string, formRun FormRunFunc) (BootstrapConfig, error) {
	var cfg BootstrapConfig

	if err := resolveTemplateRepo(templateFlag, &cfg, formRun); err != nil {
		return BootstrapConfig{}, err
	}

	for {
		ownerType, err := runPhase1TopologyOwner(w, &cfg, fetchOwners, detectOwnerType, formRun)
		if err != nil {
			return BootstrapConfig{}, err
		}

		if err := runPhase2RepoSelection(&cfg, fetchRepos, checkRepoExists, formRun); err != nil {
			return BootstrapConfig{}, err
		}

		if err := runPhase3Details(&cfg, ownerType, formRun); err != nil {
			return BootstrapConfig{}, err
		}
		cfg.OwnerType = ownerType

		if err := runPipelineConfig(&cfg, formRun); err != nil {
			return BootstrapConfig{}, err
		}

		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.SectionHeading.Render("  Summary"))
		fmt.Fprintln(w)
		fmt.Fprintln(w, RenderSummaryBox(cfg))
		fmt.Fprintln(w)

		var confirmed bool
		if err := formRun(huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title("Create project?").
				Description("Review the summary above before confirming. Select 'No' to go back and edit.").
				Value(&confirmed),
		))); err != nil {
			return BootstrapConfig{}, fmt.Errorf("confirm form: %w", err)
		}

		if confirmed {
			break
		}

		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.Muted.Render("Returning to the form — your entries are preserved. Press Enter to keep a value, or change it."))
		fmt.Fprintln(w)
	}

	return cfg, nil
}

// RenderSummaryBox renders the lipgloss summary box for the given config.
// It is a pure function, extracted to allow unit testing without a TTY.
func RenderSummaryBox(cfg BootstrapConfig) string {
	label := ui.Muted.Render
	value := ui.Value.Render

	antoraVal := "No"
	if cfg.Antora {
		antoraVal = "Yes"
	}

	repoModeVal := "new"
	if cfg.ExistingRepo {
		repoModeVal = "existing"
	}

	patStatus := "not set"
	if cfg.GooseAgentPAT != "" {
		patStatus = "set"
	}

	content := fmt.Sprintf(
		"  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s",
		label("Topology   "), value(cfg.Topology),
		label("Owner      "), value(cfg.Owner),
		label("Repo       "), value(repoModeVal),
		label("Name       "), value(cfg.ProjectName),
		label("Agent user "), value(cfg.AgentUser),
		label("Description"), value(cfg.Description),
		label("Stack      "), value(strings.Join(cfg.Stacks, ", ")),
		label("Antora     "), value(antoraVal),
		label("Runner     "), value(cfg.RunnerLabel),
		label("Provider   "), value(cfg.GooseProvider),
		label("Model      "), value(cfg.GooseModel),
		label("Agent PAT  "), value(patStatus),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Width(56).
		Padding(0, 1)

	return box.Render(content)
}
