// Package inception implements the business logic for gh agentic inception (Phase 0b).
// It registers a new domain, tool, or other repo in an existing agentic environment.
package inception

// InceptionConfig holds all values collected by the inception form.
// It is populated by RunForm and passed to the execution steps.
type InceptionConfig struct {
	// RepoType is the kind of repo being created: "domain", "tool", or "other".
	RepoType string

	// RepoName is the short name entered by the user (e.g. "charging").
	// The full GitHub repo name is derived by appending a suffix based on RepoType.
	RepoName string

	// Description is a short human-readable description of the repo.
	Description string

	// Stacks is the list of selected language/frameworks. Each entry is one of:
	// "Go", "Java Quarkus", "Java Spring Boot", "TypeScript Node.js",
	// "Python", "Rust", or "Other".
	Stacks []string

	// Owner is the GitHub account or organisation login where the repo will be created.
	// Extracted from the environment context.
	Owner string
}

// FullRepoName returns the GitHub repo name derived from RepoName and RepoType.
// Domain repos are suffixed with "-domain", tool repos with "-tool",
// and other repos use the name as-is.
func FullRepoName(cfg InceptionConfig) string {
	switch cfg.RepoType {
	case "domain":
		return cfg.RepoName + "-domain"
	case "tool":
		return cfg.RepoName + "-tool"
	default:
		return cfg.RepoName
	}
}
