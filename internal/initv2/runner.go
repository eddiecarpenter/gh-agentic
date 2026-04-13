package initv2

import "github.com/charmbracelet/huh"

const (
	// DefaultRunnerLabel is the default GitHub Actions runner label.
	DefaultRunnerLabel = "ubuntu-latest"

	// RunnerOther is the sentinel value for the "other — enter a custom label" option.
	RunnerOther = "__other__"

	// AgentUserScopeOrg indicates the AGENT_USER variable is set at org level.
	AgentUserScopeOrg = "org"
	// AgentUserScopeRepo indicates the AGENT_USER variable is set at repo level.
	AgentUserScopeRepo = "repo"

	// DefaultGooseProvider is the default Goose LLM provider.
	DefaultGooseProvider = "claude-code"
	// DefaultGooseModel is the default Goose model override.
	DefaultGooseModel = "default"
)

// RunnerDefaultForTopology returns the smart default runner label based on topology.
// Single topology defaults to "ubuntu-latest"; Federated defaults to the org name.
func RunnerDefaultForTopology(topology, owner string) string {
	if topology == "Federated" {
		return owner
	}
	return DefaultRunnerLabel
}

// BuildRunnerOptions builds the runner select options with dynamic repo and org names.
func BuildRunnerOptions(projectName, owner string) []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("ubuntu-latest — GitHub-hosted runner", DefaultRunnerLabel),
		huh.NewOption(projectName+" — Selfhosted ARC queue", projectName),
		huh.NewOption(owner+" — Selfhosted ARC queue", owner),
		huh.NewOption("self-hosted — Self-hosted runner (not production)", "self-hosted"),
		huh.NewOption("other — enter a custom label", RunnerOther),
	}
}
