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

// validateTopologyOwner checks whether the selected topology is valid for the given owner type.
// Returns ErrFederatedRequiresOrg if a personal account selects Federated topology.
// Returns nil for all other combinations (including personal + Single, which is valid but triggers a warning).
func validateTopologyOwner(topology, ownerType string) error {
	if ownerType == OwnerTypeUser && topology == "Federated" {
		return ErrFederatedRequiresOrg
	}
	return nil
}

// isPersonalSingleTopology returns true when a personal account selects Single topology.
// This combination is allowed but the caller should show a warning.
func isPersonalSingleTopology(topology, ownerType string) bool {
	return ownerType == OwnerTypeUser && topology == "Single"
}

// RunForm runs the three-group huh form, renders the summary box, and asks
// for final confirmation. Returns a populated BootstrapConfig, or ErrAborted
// if the user declines the final "Create project?" confirm.
//
// templateFlag is the value of --template from the CLI. If non-empty, it is
// used directly and the interactive prompt is skipped. If empty, an interactive
// input pre-filled with DefaultTemplateRepo is shown.
func RunForm(w io.Writer, fetchOwners FetchOwnersFunc, detectOwnerType DetectOwnerTypeFunc, templateFlag string) (BootstrapConfig, error) {
	var cfg BootstrapConfig

	// Resolve template repo: flag value or interactive prompt.
	if templateFlag != "" {
		cfg.TemplateRepo = templateFlag
	} else {
		cfg.TemplateRepo = DefaultTemplateRepo
		templateForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Template repo").
					Value(&cfg.TemplateRepo),
			),
		)
		if err := templateForm.Run(); err != nil {
			return BootstrapConfig{}, fmt.Errorf("template repo form: %w", err)
		}
	}

	// --- Group 1: Topology ---
	topologyForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select project topology").
				Options(
					huh.NewOption("Single      — one repo, control plane and project in one place", "Single"),
					huh.NewOption("Federated   — separate control plane + domain/tool repos", "Federated"),
				).
				Value(&cfg.Topology),
		),
	)
	if err := topologyForm.Run(); err != nil {
		return BootstrapConfig{}, fmt.Errorf("topology form: %w", err)
	}

	// --- Group 2: Owner ---
	owners, err := fetchOwners()
	if err != nil {
		return BootstrapConfig{}, fmt.Errorf("fetching owner list: %w", err)
	}

	ownerOpts := make([]huh.Option[string], len(owners))
	for i, o := range owners {
		ownerOpts[i] = huh.NewOption(o.Label, o.Login)
	}

	ownerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where should the repo be created?").
				Options(ownerOpts...).
				Value(&cfg.Owner),
		),
	)
	if err := ownerForm.Run(); err != nil {
		return BootstrapConfig{}, fmt.Errorf("owner form: %w", err)
	}

	// --- Owner type validation ---
	ownerType, err := detectOwnerType(cfg.Owner)
	if err != nil {
		return BootstrapConfig{}, fmt.Errorf("detecting owner type: %w", err)
	}

	if valErr := validateTopologyOwner(cfg.Topology, ownerType); valErr != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.RenderError("Federated topology requires an org account"))
		fmt.Fprintln(w, "  "+ui.Muted.Render("Personal accounts cannot host federated agentic environments."))
		fmt.Fprintln(w, "  "+ui.Muted.Render("Choose Single topology or select an org as the owner."))
		fmt.Fprintln(w)
		return BootstrapConfig{}, ErrFederatedRequiresOrg
	}

	if isPersonalSingleTopology(cfg.Topology, ownerType) {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.RenderWarning("Kanban triggering not available on personal accounts"))
		fmt.Fprintln(w, "  "+ui.Muted.Render("A project board will be created, but dragging cards will not"))
		fmt.Fprintln(w, "  "+ui.Muted.Render("trigger pipeline workflows. Apply labels manually to trigger."))
		fmt.Fprintln(w)

		var continueConfirmed bool
		warningForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Continue?").
					Value(&continueConfirmed),
			),
		)
		if err := warningForm.Run(); err != nil {
			return BootstrapConfig{}, fmt.Errorf("warning confirm form: %w", err)
		}
		if !continueConfirmed {
			return BootstrapConfig{}, ErrAborted
		}
	}

	// --- Group 3: Project details ---
	detailsForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Name").
				Value(&cfg.ProjectName).
				Validate(validateProjectName),
			huh.NewInput().
				Title("Description").
				Value(&cfg.Description),
			huh.NewMultiSelect[string]().
				Title("Stack (select all that apply)").
				Options(stackOptions...).
				Value(&cfg.Stacks).
				Validate(func(selected []string) error {
					if len(selected) == 0 {
						return errors.New("at least one stack must be selected")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Antora documentation site?").
				Value(&cfg.Antora),
		),
	)

	if err := detailsForm.Run(); err != nil {
		return BootstrapConfig{}, fmt.Errorf("project details form: %w", err)
	}

	// --- Group 4: Pipeline configuration ---
	cfg.RunnerLabel = DefaultRunnerLabel
	cfg.GooseProvider = DefaultGooseProvider
	cfg.GooseModel = DefaultGooseModel

	pipelineForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Runner label").
				Value(&cfg.RunnerLabel),
			huh.NewInput().
				Title("Goose provider").
				Value(&cfg.GooseProvider),
			huh.NewInput().
				Title("Goose model").
				Value(&cfg.GooseModel),
		),
	)
	if err := pipelineForm.Run(); err != nil {
		return BootstrapConfig{}, fmt.Errorf("pipeline config form: %w", err)
	}

	// --- Summary box ---
	fmt.Fprintln(w)
	fmt.Fprintln(w, ui.SectionHeading.Render("  Summary"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, RenderSummaryBox(cfg))
	fmt.Fprintln(w)

	// --- Final confirm ---
	var confirmed bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Create project?").
				Value(&confirmed),
		),
	)
	if err := confirmForm.Run(); err != nil {
		return BootstrapConfig{}, fmt.Errorf("confirm form: %w", err)
	}

	if !confirmed {
		return BootstrapConfig{}, ErrAborted
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

	content := fmt.Sprintf(
		"  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s\n  %s  %s",
		label("Topology   "), value(cfg.Topology),
		label("Owner      "), value(cfg.Owner),
		label("Name       "), value(cfg.ProjectName),
		label("Description"), value(cfg.Description),
		label("Stack      "), value(strings.Join(cfg.Stacks, ", ")),
		label("Antora     "), value(antoraVal),
		label("Runner     "), value(cfg.RunnerLabel),
		label("Provider   "), value(cfg.GooseProvider),
		label("Model      "), value(cfg.GooseModel),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Width(56).
		Padding(0, 1)

	return box.Render(content)
}
