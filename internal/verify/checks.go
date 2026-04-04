package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// CheckCLAUDEMD verifies that CLAUDE.md exists in the repo root.
// Returns Fail if the file is missing.
func CheckCLAUDEMD(root string) CheckResult {
	path := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "CLAUDE.md exists",
			Status:  Fail,
			Message: "file not found",
		}
	}
	return CheckResult{
		Name:   "CLAUDE.md exists",
		Status: Pass,
	}
}

// CheckAGENTSLocalMD verifies that AGENTS.local.md exists in the repo root.
// Returns Warning if the file is missing (it can be restored as a skeleton).
func CheckAGENTSLocalMD(root string) CheckResult {
	path := filepath.Join(root, "AGENTS.local.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "AGENTS.local.md exists",
			Status:  Warning,
			Message: "file not found — will restore minimal skeleton",
		}
	}
	return CheckResult{
		Name:   "AGENTS.local.md exists",
		Status: Pass,
	}
}

// CheckTEMPLATESOURCE verifies that TEMPLATE_SOURCE exists in the repo root.
// Returns Warning if the file is missing (requires user input to repair).
func CheckTEMPLATESOURCE(root string) CheckResult {
	path := filepath.Join(root, "TEMPLATE_SOURCE")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: "file not found — value must be provided",
		}
	}
	return CheckResult{
		Name:   "TEMPLATE_SOURCE exists",
		Status: Pass,
	}
}

// CheckTEMPLATEVERSION verifies that TEMPLATE_VERSION exists in the repo root.
// Returns Fail if the file is missing.
func CheckTEMPLATEVERSION(root string) CheckResult {
	path := filepath.Join(root, "TEMPLATE_VERSION")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: "file not found",
		}
	}
	return CheckResult{
		Name:   "TEMPLATE_VERSION exists",
		Status: Pass,
	}
}

// CheckREPOSMD verifies that REPOS.md exists in the repo root.
// Returns Fail if the file is missing.
func CheckREPOSMD(root string) CheckResult {
	path := filepath.Join(root, "REPOS.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "REPOS.md exists",
			Status:  Fail,
			Message: "file not found",
		}
	}
	return CheckResult{
		Name:   "REPOS.md exists",
		Status: Pass,
	}
}

// CheckREADMEMD verifies that README.md exists in the repo root.
// Returns Fail if the file is missing.
func CheckREADMEMD(root string) CheckResult {
	path := filepath.Join(root, "README.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return CheckResult{
			Name:    "README.md exists",
			Status:  Fail,
			Message: "file not found",
		}
	}
	return CheckResult{
		Name:   "README.md exists",
		Status: Pass,
	}
}

// ──────────���───────────────────────────────────────────────────────────────────
// Directory integrity checks
// ──────────────��────────────────────────────────���──────────────────────────────

// expectedRecipeYAMLs are the standard Goose recipe files expected in .goose/recipes/.
var expectedRecipeYAMLs = []string{
	"dev-session.yaml",
	"feature-design.yaml",
	"feature-scoping.yaml",
	"foreground-recovery.yaml",
	"issue-session.yaml",
	"pr-review-session.yaml",
	"requirements-session.yaml",
}

// expectedWorkflowYMLs are the standard pipeline workflow files expected in .github/workflows/.
// ci.yml is project-specific and excluded from this list.
var expectedWorkflowYMLs = []string{
	"agentic-pipeline.yml",
}

// CheckBaseDir verifies that the base/ directory exists and has no uncommitted
// modifications. Uses RunCommandFunc for git operations.
func CheckBaseDir(root string, run bootstrap.RunCommandFunc) CheckResult {
	basePath := filepath.Join(root, "base")
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return CheckResult{
			Name:    "base/ exists and is unmodified",
			Status:  Fail,
			Message: "base/ directory not found",
		}
	}

	// Check for uncommitted modifications via git diff.
	out, err := run("bash", "-c", fmt.Sprintf("cd '%s' && git diff HEAD -- base/", strings.ReplaceAll(root, "'", "'\\''")))
	if err != nil {
		// If git fails (e.g. not a git repo), treat as warning.
		return CheckResult{
			Name:    "base/ exists and is unmodified",
			Status:  Warning,
			Message: "could not check git status: " + strings.TrimSpace(fmt.Sprintf("%v", err)),
		}
	}

	if strings.TrimSpace(out) != "" {
		return CheckResult{
			Name:    "base/ exists and is unmodified",
			Status:  Fail,
			Message: "base/ has uncommitted modifications",
		}
	}

	return CheckResult{
		Name:   "base/ exists and is unmodified",
		Status: Pass,
	}
}

// CheckBaseRecipes verifies that base/skills/*.md files exist and are unmodified.
// Uses RunCommandFunc for git operations.
func CheckBaseRecipes(root string, run bootstrap.RunCommandFunc) CheckResult {
	recipesPath := filepath.Join(root, "base", "skills")
	if _, err := os.Stat(recipesPath); os.IsNotExist(err) {
		return CheckResult{
			Name:    "base/skills/*.md unmodified",
			Status:  Warning,
			Message: "base/skills/ directory not found",
		}
	}

	// Check for modifications via git diff on base/skills/.
	out, err := run("bash", "-c", fmt.Sprintf("cd '%s' && git diff HEAD -- base/skills/", strings.ReplaceAll(root, "'", "'\\''")))
	if err != nil {
		return CheckResult{
			Name:    "base/skills/*.md unmodified",
			Status:  Warning,
			Message: "could not check git status: " + strings.TrimSpace(fmt.Sprintf("%v", err)),
		}
	}

	if strings.TrimSpace(out) != "" {
		return CheckResult{
			Name:    "base/skills/*.md unmodified",
			Status:  Warning,
			Message: "base/skills/ has local modifications",
		}
	}

	return CheckResult{
		Name:   "base/skills/*.md unmodified",
		Status: Pass,
	}
}

// CheckGooseRecipes verifies that .goose/recipes/ contains all expected YAML files.
func CheckGooseRecipes(root string) CheckResult {
	recipesPath := filepath.Join(root, ".goose", "recipes")
	if _, err := os.Stat(recipesPath); os.IsNotExist(err) {
		return CheckResult{
			Name:    ".goose/recipes/ exists and complete",
			Status:  Fail,
			Message: ".goose/recipes/ directory not found",
		}
	}

	var missing []string
	for _, name := range expectedRecipeYAMLs {
		path := filepath.Join(recipesPath, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:    ".goose/recipes/ exists and complete",
			Status:  Fail,
			Message: fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
		}
	}

	return CheckResult{
		Name:   ".goose/recipes/ exists and complete",
		Status: Pass,
	}
}

// CheckWorkflows verifies that .github/workflows/ contains all expected pipeline files.
func CheckWorkflows(root string) CheckResult {
	workflowsPath := filepath.Join(root, ".github", "workflows")
	if _, err := os.Stat(workflowsPath); os.IsNotExist(err) {
		return CheckResult{
			Name:    ".github/workflows/ exists and complete",
			Status:  Fail,
			Message: ".github/workflows/ directory not found",
		}
	}

	var missing []string
	for _, name := range expectedWorkflowYMLs {
		path := filepath.Join(workflowsPath, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:    ".github/workflows/ exists and complete",
			Status:  Fail,
			Message: fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
		}
	}

	return CheckResult{
		Name:   ".github/workflows/ exists and complete",
		Status: Pass,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GitHub remote checks
// ──────────────────────────────────────────────────────────────────────────────

// standardLabels are the 9 labels required in every agentic repo.
var standardLabels = []string{
	"requirement", "feature", "task", "backlog", "draft",
	"in-design", "in-development", "in-review", "done",
}

// labelEntry is used to unmarshal JSON output from `gh label list`.
type labelEntry struct {
	Name string `json:"name"`
}

// CheckLabels verifies that all 9 standard labels exist in the repo.
// repoFullName is "owner/repo". run is injected for gh operations.
func CheckLabels(repoFullName string, run bootstrap.RunCommandFunc) CheckResult {
	out, err := run("gh", "label", "list", "--repo", repoFullName, "--json", "name", "--limit", "100")
	if err != nil {
		return CheckResult{
			Name:    "Standard labels present",
			Status:  Fail,
			Message: fmt.Sprintf("failed to list labels: %v", err),
		}
	}

	var labels []labelEntry
	if err := json.Unmarshal([]byte(out), &labels); err != nil {
		return CheckResult{
			Name:    "Standard labels present",
			Status:  Fail,
			Message: fmt.Sprintf("failed to parse label JSON: %v", err),
		}
	}

	existing := make(map[string]bool, len(labels))
	for _, l := range labels {
		existing[l.Name] = true
	}

	var missing []string
	for _, name := range standardLabels {
		if !existing[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return CheckResult{
			Name:    "Standard labels present",
			Status:  Fail,
			Message: fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
		}
	}

	return CheckResult{
		Name:   "Standard labels present",
		Status: Pass,
	}
}

// MissingLabels returns the list of standard labels missing from the repo.
// This is a helper used by RepairLabels to determine which labels to create.
func MissingLabels(repoFullName string, run bootstrap.RunCommandFunc) []string {
	out, err := run("gh", "label", "list", "--repo", repoFullName, "--json", "name", "--limit", "100")
	if err != nil {
		return standardLabels // If we can't list, assume all missing.
	}

	var labels []labelEntry
	if err := json.Unmarshal([]byte(out), &labels); err != nil {
		return standardLabels
	}

	existing := make(map[string]bool, len(labels))
	for _, l := range labels {
		existing[l.Name] = true
	}

	var missing []string
	for _, name := range standardLabels {
		if !existing[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// CheckProject verifies that a GitHub Project exists for the repo owner.
// owner is the GitHub account/org. run is injected for gh operations.
func CheckProject(owner string, run bootstrap.RunCommandFunc) CheckResult {
	out, err := run("gh", "project", "list", "--owner", owner, "--format", "json", "--limit", "100")
	if err != nil {
		return CheckResult{
			Name:    "GitHub Project linked",
			Status:  Fail,
			Message: fmt.Sprintf("failed to list projects: %v", err),
		}
	}

	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" || strings.TrimSpace(out) == "{\"projects\":[]}" {
		return CheckResult{
			Name:    "GitHub Project linked",
			Status:  Fail,
			Message: "no GitHub Project found for owner " + owner,
		}
	}

	return CheckResult{
		Name:   "GitHub Project linked",
		Status: Pass,
	}
}
