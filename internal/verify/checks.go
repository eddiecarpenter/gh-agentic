package verify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// ghNotifyGOOS is the OS identifier used by CheckGhNotify.
// It defaults to runtime.GOOS but can be overridden in tests.
var ghNotifyGOOS = runtime.GOOS

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

// CheckSkillsDir verifies that the skills/ directory exists in the repo root.
// Returns Warning (not Fail) if absent — it is optional but recommended for
// local project-specific skills.
func CheckSkillsDir(root string) CheckResult {
	path := filepath.Join(root, "skills")
	info, err := os.Stat(path)
	if os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		return CheckResult{
			Name:    "skills/ directory exists",
			Status:  Warning,
			Message: "directory not found — recommended for local project-specific skills",
		}
	}
	return CheckResult{
		Name:   "skills/ directory exists",
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

// CheckWorkflows verifies that .github/workflows/ contains all expected pipeline
// files. If base/.github/workflows/ exists, it verifies content matches
// byte-for-byte. Otherwise falls back to existence-only checks using
// expectedWorkflowYMLs.
func CheckWorkflows(root string) CheckResult {
	const checkName = ".github/workflows/ exists and complete"

	workflowsPath := filepath.Join(root, ".github", "workflows")
	baseWorkflowsPath := filepath.Join(root, "base", ".github", "workflows")

	// If base/.github/workflows/ exists, use content comparison.
	if info, err := os.Stat(baseWorkflowsPath); err == nil && info.IsDir() {
		entries, err := os.ReadDir(baseWorkflowsPath)
		if err != nil {
			return CheckResult{
				Name:    checkName,
				Status:  Fail,
				Message: fmt.Sprintf("reading base workflows: %v", err),
			}
		}

		var missing []string
		var differs []string

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			basePath := filepath.Join(baseWorkflowsPath, name)
			deployedPath := filepath.Join(workflowsPath, name)

			baseContent, err := os.ReadFile(basePath)
			if err != nil {
				missing = append(missing, name)
				continue
			}

			deployedContent, err := os.ReadFile(deployedPath)
			if err != nil {
				missing = append(missing, name)
				continue
			}

			if !bytes.Equal(baseContent, deployedContent) {
				differs = append(differs, name)
			}
		}

		if len(missing) > 0 || len(differs) > 0 {
			var parts []string
			if len(missing) > 0 {
				parts = append(parts, "missing: "+strings.Join(missing, ", "))
			}
			if len(differs) > 0 {
				parts = append(parts, "content differs: "+strings.Join(differs, ", "))
			}
			return CheckResult{
				Name:    checkName,
				Status:  Fail,
				Message: strings.Join(parts, "; "),
			}
		}

		return CheckResult{
			Name:   checkName,
			Status: Pass,
		}
	}

	// Fallback: existence-only check using expectedWorkflowYMLs.
	if _, err := os.Stat(workflowsPath); os.IsNotExist(err) {
		return CheckResult{
			Name:    checkName,
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
			Name:    checkName,
			Status:  Fail,
			Message: fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
		}
	}

	return CheckResult{
		Name:   checkName,
		Status: Pass,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GitHub remote checks
// ──────────────────────────────────────────────────────────────────────────────

// standardLabels are the 11 labels required in every agentic repo.
var standardLabels = []string{
	"requirement", "feature", "task", "backlog", "draft",
	"scoping", "scheduled",
	"in-design", "in-development", "in-review", "done",
}

// labelEntry is used to unmarshal JSON output from `gh label list`.
type labelEntry struct {
	Name string `json:"name"`
}

// CheckLabels verifies that all 11 standard labels exist in the repo.
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

// CheckGhNotify verifies that the gh-notify LaunchAgent is installed and running.
// On non-darwin systems the check passes immediately as not applicable.
// root is the repo root, run is injected for launchctl operations.
func CheckGhNotify(root string, run bootstrap.RunCommandFunc) CheckResult {
	const checkName = "gh-notify LaunchAgent installed"

	if ghNotifyGOOS != "darwin" {
		return CheckResult{
			Name:    checkName,
			Status:  Pass,
			Message: "not applicable on this OS",
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return CheckResult{
			Name:    checkName,
			Status:  Fail,
			Message: fmt.Sprintf("could not determine home directory: %v", err),
		}
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.user.gh-notify.plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return CheckResult{
			Name:    checkName,
			Status:  Fail,
			Message: "plist not found at ~/Library/LaunchAgents/com.user.gh-notify.plist",
		}
	}

	_, err = run("launchctl", "list", "com.user.gh-notify")
	if err != nil {
		return CheckResult{
			Name:    checkName,
			Status:  Fail,
			Message: "LaunchAgent not loaded",
		}
	}

	return CheckResult{
		Name:    checkName,
		Status:  Pass,
		Message: "gh-notify LaunchAgent installed and running",
	}
}

// checkProjectStatusName is the check name used for project status verification.
const checkProjectStatusName = "GitHub Project status options are standard"

// CheckProjectStatus verifies that the GitHub Project has the canonical status
// options in the correct order. Uses run to shell out to gh api graphql.
func CheckProjectStatus(owner string, run bootstrap.RunCommandFunc) CheckResult {
	// Step 1: Find the project node ID.
	projectNodeID := resolveProjectNodeIDViaRun(owner, run)
	if projectNodeID == "" {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: "no GitHub Project found for owner " + owner,
		}
	}

	// Step 2: Fetch the Status field and its options via GraphQL.
	query := fmt.Sprintf(`{ node(id: \"%s\") { ... on ProjectV2 { field(name: \"Status\") { ... on ProjectV2SingleSelectField { id options { name } } } } } }`, projectNodeID)
	out, err := run("gh", "api", "graphql", "-f", "query="+query, "--jq", ".data.node.field.options[].name")
	if err != nil {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to fetch status options: %v", err),
		}
	}

	// Parse the returned option names (one per line).
	var gotNames []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			gotNames = append(gotNames, line)
		}
	}

	// Step 3: Compare against canonical set.
	canonical := bootstrap.StatusOptionNames()
	if len(gotNames) != len(canonical) {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: fmt.Sprintf("expected %d options, got %d: %v", len(canonical), len(gotNames), gotNames),
		}
	}

	for i, name := range canonical {
		if gotNames[i] != name {
			return CheckResult{
				Name:    checkProjectStatusName,
				Status:  Fail,
				Message: fmt.Sprintf("option %d: expected %q, got %q", i+1, name, gotNames[i]),
			}
		}
	}

	return CheckResult{
		Name:   checkProjectStatusName,
		Status: Pass,
	}
}

// resolveProjectNodeIDViaRun resolves the project node ID for an owner by
// trying user first, then org. Uses run to shell out to gh api graphql.
func resolveProjectNodeIDViaRun(owner string, run bootstrap.RunCommandFunc) string {
	// Try user query.
	userQuery := fmt.Sprintf(`{ user(login: \"%s\") { projectsV2(first: 1) { nodes { id } } } }`, owner)
	out, err := run("gh", "api", "graphql", "-f", "query="+userQuery, "--jq", ".data.user.projectsV2.nodes[0].id")
	if err == nil {
		id := strings.TrimSpace(out)
		if id != "" && id != "null" {
			return id
		}
	}

	// Try org query.
	orgQuery := fmt.Sprintf(`{ organization(login: \"%s\") { projectsV2(first: 1) { nodes { id } } } }`, owner)
	out, err = run("gh", "api", "graphql", "-f", "query="+orgQuery, "--jq", ".data.organization.projectsV2.nodes[0].id")
	if err == nil {
		id := strings.TrimSpace(out)
		if id != "" && id != "null" {
			return id
		}
	}

	return ""
}

// checkProjectCollaboratorName is the check name used for project collaborator verification.
const checkProjectCollaboratorName = "Agent user is a project collaborator"

// CheckProjectCollaborator verifies that the configured agent user is a collaborator
// on the GitHub Project. Returns Pass with note when agentUser is empty (skips gracefully).
func CheckProjectCollaborator(owner string, agentUser string, run bootstrap.RunCommandFunc) CheckResult {
	if agentUser == "" {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Pass,
			Message: "no agent user configured — skipping",
		}
	}

	// Find the project node ID.
	projectNodeID := resolveProjectNodeIDViaRun(owner, run)
	if projectNodeID == "" {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Fail,
			Message: "no GitHub Project found for owner " + owner,
		}
	}

	// Query project collaborators.
	query := fmt.Sprintf(`{ node(id: \"%s\") { ... on ProjectV2 { collaborators(first: 100) { nodes { login } } } } }`, projectNodeID)
	out, err := run("gh", "api", "graphql", "-f", "query="+query, "--jq", ".data.node.collaborators.nodes[].login")
	if err != nil {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to fetch collaborators: %v", err),
		}
	}

	// Check if agent user is present in the list.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == agentUser {
			return CheckResult{
				Name:   checkProjectCollaboratorName,
				Status: Pass,
			}
		}
	}

	return CheckResult{
		Name:    checkProjectCollaboratorName,
		Status:  Fail,
		Message: agentUser + " is not a project collaborator",
	}
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
