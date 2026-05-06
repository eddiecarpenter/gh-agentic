package init

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

	// DefaultAgentProvider is the default agent LLM provider. The value
	// stays "claude-code" — this is the identifier the Goose CLI recognises
	// for the Claude Code provider; only the constant name changes.
	DefaultAgentProvider = "claude-code"
	// DefaultAgentModel is the default agent model. Uses claude-sonnet-4.6 as
	// the canonical model for the agentic pipeline.
	DefaultAgentModel = "claude-sonnet-4.6"
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
