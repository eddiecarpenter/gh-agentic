package inception

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/sync"
)

// EnvContext holds context extracted from the agentic environment.
// It is returned by ValidateEnvironment and passed to subsequent inception steps.
type EnvContext struct {
	// Owner is the GitHub account or organisation that owns this agentic environment.
	Owner string

	// OwnerType is the detected GitHub owner type: bootstrap.OwnerTypeUser or bootstrap.OwnerTypeOrg.
	OwnerType string

	// AgenticRepoRoot is the absolute path to the agentic repo root directory.
	AgenticRepoRoot string

	// Module is the Go module path if a go.mod exists, otherwise empty.
	Module string

	// TemplateRepo is the GitHub owner/repo of the template, read from TEMPLATE_SOURCE.
	TemplateRepo string

	// DetectOwnerType is the function used to resolve owner type. Injectable for testing.
	DetectOwnerType bootstrap.DetectOwnerTypeFunc
}

// ValidateEnvironment checks that the current directory is an agentic environment
// by verifying REPOS.md exists. It extracts the owner from AGENTS.local.md or
// the git remote, detects the owner type, and returns an EnvContext for use by
// subsequent steps.
//
// run and detectOwnerType are injected so tests can substitute fake implementations.
func ValidateEnvironment(run bootstrap.RunCommandFunc, detectOwnerType bootstrap.DetectOwnerTypeFunc) (*EnvContext, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	// Check REPOS.md exists.
	reposPath := filepath.Join(wd, "REPOS.md")
	if _, err := os.Stat(reposPath); err != nil {
		return nil, fmt.Errorf("not an agentic environment: REPOS.md not found in %s", wd)
	}

	ctx := &EnvContext{
		AgenticRepoRoot: wd,
	}

	// Try to extract owner from AGENTS.local.md.
	ctx.Owner = extractOwnerFromAgentsLocal(wd)

	// Fall back to git remote if owner not found.
	if ctx.Owner == "" {
		ctx.Owner = extractOwnerFromGitRemote(run)
	}

	if ctx.Owner == "" {
		return nil, fmt.Errorf("could not determine GitHub owner — set it in AGENTS.local.md (## Repo → **Owner:**)")
	}

	// Detect owner type (personal account vs organisation).
	ctx.DetectOwnerType = detectOwnerType
	ownerType, err := detectOwnerType(ctx.Owner)
	if err != nil {
		return nil, fmt.Errorf("detecting owner type for %q: %w", ctx.Owner, err)
	}
	ctx.OwnerType = ownerType

	// Try to extract Go module path.
	ctx.Module = extractGoModule(wd)

	// Read template repo from TEMPLATE_SOURCE.
	templateRepo, err := sync.ReadTemplateSource(wd)
	if err != nil {
		// Fall back to the default if TEMPLATE_SOURCE is missing or unreadable.
		ctx.TemplateRepo = bootstrap.DefaultTemplateRepo
	} else {
		ctx.TemplateRepo = templateRepo
	}

	return ctx, nil
}

// extractOwnerFromAgentsLocal parses AGENTS.local.md looking for the GitHub owner.
// It looks for lines like "- **GitHub:** https://github.com/<owner>/<repo>" or
// "- **Owner:** <owner>".
func extractOwnerFromAgentsLocal(root string) string {
	path := filepath.Join(root, "AGENTS.local.md")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Try "- **GitHub:** https://github.com/<owner>/<repo>"
		if strings.Contains(line, "**GitHub:**") && strings.Contains(line, "github.com") {
			parts := strings.Split(line, "github.com/")
			if len(parts) >= 2 {
				rest := strings.TrimSpace(parts[1])
				ownerRepo := strings.SplitN(rest, "/", 2)
				if len(ownerRepo) >= 1 && ownerRepo[0] != "" {
					return ownerRepo[0]
				}
			}
		}

		// Try "- **Owner:** <owner>"
		if strings.Contains(line, "**Owner:**") {
			parts := strings.SplitN(line, "**Owner:**", 2)
			if len(parts) == 2 {
				owner := strings.TrimSpace(parts[1])
				if owner != "" {
					return owner
				}
			}
		}
	}

	return ""
}

// extractOwnerFromGitRemote tries to extract the owner from the git remote origin URL.
func extractOwnerFromGitRemote(run bootstrap.RunCommandFunc) string {
	out, err := run("git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(out)

	// SSH format: git@github.com:<owner>/<repo>.git
	if strings.Contains(url, "git@github.com:") {
		parts := strings.Split(url, "git@github.com:")
		if len(parts) >= 2 {
			ownerRepo := strings.SplitN(parts[1], "/", 2)
			if len(ownerRepo) >= 1 {
				return ownerRepo[0]
			}
		}
	}

	// HTTPS format: https://github.com/<owner>/<repo>.git
	if strings.Contains(url, "github.com/") {
		parts := strings.Split(url, "github.com/")
		if len(parts) >= 2 {
			ownerRepo := strings.SplitN(parts[1], "/", 2)
			if len(ownerRepo) >= 1 {
				return ownerRepo[0]
			}
		}
	}

	return ""
}

// extractGoModule reads go.mod and returns the module path, or empty string.
func extractGoModule(root string) string {
	path := filepath.Join(root, "go.mod")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
