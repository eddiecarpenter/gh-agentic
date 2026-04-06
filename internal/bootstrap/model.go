// Package bootstrap implements the business logic for gh agentic bootstrap.
package bootstrap

// BootstrapConfig holds all values collected by the bootstrap form.
// It is populated by RunForm and passed to the execution steps.
type BootstrapConfig struct {
	// Topology is the project structure: "Single" or "Federated".
	Topology string

	// Owner is the GitHub account or organisation login where the repo will be created.
	Owner string

	// ProjectName is the short name of the project, validated to be lowercase
	// with hyphens only and no spaces.
	ProjectName string

	// Description is a short human-readable description of the project.
	Description string

	// Stack is the primary language/framework: "Go", "Java Quarkus",
	// "Java Spring Boot", "TypeScript Node.js", "Python", "Rust", or "Other".
	Stack string

	// Antora indicates whether an Antora documentation site should be scaffolded.
	Antora bool

	// OwnerType is the detected GitHub owner type: OwnerTypeUser or OwnerTypeOrg.
	// Set after form completion and before RunSteps is called.
	OwnerType string

	// AgentUser is the GitHub username for the agent (e.g. "goose-agent").
	// Optional — if empty, it will be collected interactively during bootstrap.
	AgentUser string

	// AgentUserScope is the scope for the AGENT_USER variable: "org" or "repo".
	// Optional — if empty, it will be collected interactively during bootstrap.
	AgentUserScope string
}

const (
	// AgentUserScopeOrg indicates the AGENT_USER variable is set at org level.
	AgentUserScopeOrg = "org"
	// AgentUserScopeRepo indicates the AGENT_USER variable is set at repo level.
	AgentUserScopeRepo = "repo"
)
