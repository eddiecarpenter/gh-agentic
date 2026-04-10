package verify

import (
	"bytes"
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
//
// Deprecated: TEMPLATE_SOURCE is replaced by .ai/config.yml.
// TODO(deprecated): remove in next major version
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
//
// Deprecated: TEMPLATE_VERSION is replaced by .ai/config.yml.
// TODO(deprecated): remove in next major version
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

// orgOnlyWorkflows is the set of workflow files that are only applicable to
// GitHub organisation accounts. They are excluded from checks and repairs
// when the repo owner is a personal account.
var orgOnlyWorkflows = map[string]bool{
	"sync-status-to-label.yml": true,
}

// CheckAIDir verifies that the .ai/ directory exists and has no uncommitted
// modifications. Uses RunCommandFunc for git operations.
func CheckAIDir(root string, run bootstrap.RunCommandFunc) CheckResult {
	aiPath := filepath.Join(root, ".ai")
	if _, err := os.Stat(aiPath); os.IsNotExist(err) {
		return CheckResult{
			Name:    ".ai/ exists and is unmodified",
			Status:  Fail,
			Message: ".ai/ directory not found",
		}
	}

	// Check for uncommitted modifications via git diff.
	out, err := run("bash", "-c", fmt.Sprintf("cd '%s' && git diff HEAD -- .ai/", strings.ReplaceAll(root, "'", "'\\''")))
	if err != nil {
		// If git fails (e.g. not a git repo), treat as warning.
		return CheckResult{
			Name:    ".ai/ exists and is unmodified",
			Status:  Warning,
			Message: "could not check git status: " + strings.TrimSpace(fmt.Sprintf("%v", err)),
		}
	}

	if strings.TrimSpace(out) != "" {
		return CheckResult{
			Name:    ".ai/ exists and is unmodified",
			Status:  Fail,
			Message: ".ai/ has uncommitted modifications",
		}
	}

	return CheckResult{
		Name:   ".ai/ exists and is unmodified",
		Status: Pass,
	}
}

// CheckOldLayout detects repos still using the old base/ directory layout.
// Returns Warning if base/ exists without .ai/ (needs migration).
func CheckOldLayout(root string) CheckResult {
	const checkName = "no legacy base/ layout"
	aiPath := filepath.Join(root, ".ai")
	basePath := filepath.Join(root, "base")

	_, aiErr := os.Stat(aiPath)
	aiExists := aiErr == nil

	_, baseErr := os.Stat(basePath)
	baseExists := baseErr == nil

	if baseExists && !aiExists {
		return CheckResult{
			Name:    checkName,
			Status:  Warning,
			Message: "base/ directory found without .ai/ — run gh agentic sync to migrate",
		}
	}

	return CheckResult{
		Name:   checkName,
		Status: Pass,
	}
}

// CheckAISkills verifies that .ai/skills/*.md files exist and are unmodified.
// Uses RunCommandFunc for git operations.
func CheckAISkills(root string, run bootstrap.RunCommandFunc) CheckResult {
	recipesPath := filepath.Join(root, ".ai", "skills")
	if _, err := os.Stat(recipesPath); os.IsNotExist(err) {
		return CheckResult{
			Name:    ".ai/skills/*.md unmodified",
			Status:  Warning,
			Message: ".ai/skills/ directory not found",
		}
	}

	// Check for modifications via git diff on .ai/skills/.
	out, err := run("bash", "-c", fmt.Sprintf("cd '%s' && git diff HEAD -- .ai/skills/", strings.ReplaceAll(root, "'", "'\\''")))
	if err != nil {
		return CheckResult{
			Name:    ".ai/skills/*.md unmodified",
			Status:  Warning,
			Message: "could not check git status: " + strings.TrimSpace(fmt.Sprintf("%v", err)),
		}
	}

	if strings.TrimSpace(out) != "" {
		return CheckResult{
			Name:    ".ai/skills/*.md unmodified",
			Status:  Warning,
			Message: ".ai/skills/ has local modifications",
		}
	}

	return CheckResult{
		Name:   ".ai/skills/*.md unmodified",
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
// files. If .ai/.github/workflows/ exists, it verifies content matches
// byte-for-byte. Otherwise falls back to existence-only checks using
// expectedWorkflowYMLs.
//
// ownerType is bootstrap.OwnerTypeUser or bootstrap.OwnerTypeOrg. Org-only
// workflows (e.g. sync-status-to-label.yml) are skipped for personal accounts.
func CheckWorkflows(root, ownerType string) CheckResult {
	const checkName = ".github/workflows/ exists and complete"

	workflowsPath := filepath.Join(root, ".github", "workflows")
	aiWorkflowsPath := filepath.Join(root, ".ai", ".github", "workflows")

	// If .ai/.github/workflows/ exists, use content comparison.
	if info, err := os.Stat(aiWorkflowsPath); err == nil && info.IsDir() {
		entries, err := os.ReadDir(aiWorkflowsPath)
		if err != nil {
			return CheckResult{
				Name:    checkName,
				Status:  Fail,
				Message: fmt.Sprintf("reading .ai workflows: %v", err),
			}
		}

		var missing []string
		var differs []string

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			// Skip org-only workflows for personal accounts.
			if orgOnlyWorkflows[name] && ownerType == bootstrap.OwnerTypeUser {
				continue
			}
			aiPath := filepath.Join(aiWorkflowsPath, name)
			deployedPath := filepath.Join(workflowsPath, name)

			aiContent, err := os.ReadFile(aiPath)
			if err != nil {
				missing = append(missing, name)
				continue
			}

			deployedContent, err := os.ReadFile(deployedPath)
			if err != nil {
				missing = append(missing, name)
				continue
			}

			if !bytes.Equal(aiContent, deployedContent) {
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


// checkProjectStatusName is the check name used for project status verification.
const checkProjectStatusName = "GitHub Project status options are standard"

// CheckProjectStatus verifies that the GitHub Project has the canonical status
// options in the correct order. Loads canonical options from .ai/project-template.json.
// Mismatch is reported as Warning (not Fail) — local customisation is permitted.
func CheckProjectStatus(owner, repoName, root string, run bootstrap.RunCommandFunc) CheckResult {
	// Step 1: Find the project node ID.
	projectNodeID := resolveProjectNodeIDViaRun(owner, repoName, run)
	if projectNodeID == "" {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: "no GitHub Project found for owner " + owner,
		}
	}

	// Step 2: Fetch the Status field options (name + color) via GraphQL.
	query := fmt.Sprintf(`{ node(id: "%s") { ... on ProjectV2 { field(name: "Status") { ... on ProjectV2SingleSelectField { id options { name color } } } } } }`, projectNodeID)
	out, err := run("gh", "api", "graphql", "-f", "query="+query, "--jq", `.data.node.field.options[] | "\(.name)|\(.color)"`)
	if err != nil {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to fetch status options: %v", err),
		}
	}

	// Parse "name|color" lines from response.
	type liveOption struct{ name, color string }
	var got []liveOption
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			got = append(got, liveOption{parts[0], parts[1]})
		}
	}

	// Step 3: Load canonical set from .ai/project-template.json.
	tmpl, loadErr := bootstrap.LoadProjectTemplate(root)
	if loadErr != nil {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: fmt.Sprintf("could not load project template: %v", loadErr),
		}
	}

	if len(got) != len(tmpl.ResolvedStatusOptions()) {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Warning,
			Message: fmt.Sprintf("expected %d options, got %d", len(tmpl.ResolvedStatusOptions()), len(got)),
		}
	}

	for i, want := range tmpl.ResolvedStatusOptions() {
		if got[i].name != want.Name || got[i].color != want.Color {
			return CheckResult{
				Name:    checkProjectStatusName,
				Status:  Warning,
				Message: fmt.Sprintf("option %d: expected %q (%s), got %q (%s)", i+1, want.Name, want.Color, got[i].name, got[i].color),
			}
		}
	}

	return CheckResult{
		Name:   checkProjectStatusName,
		Status: Pass,
	}
}

// checkProjectViewsName is the check name for required project views.
const checkProjectViewsName = "GitHub Project has required views"

// CheckProjectViews verifies that the GitHub Project contains all required views
// defined in .ai/project-template.json. Only checks for presence — existing
// views are never flagged for layout or filter differences (user customisation
// is intentional).
func CheckProjectViews(owner, repoName, root string, run bootstrap.RunCommandFunc) CheckResult {
	projectNodeID := resolveProjectNodeIDViaRun(owner, repoName, run)
	if projectNodeID == "" {
		return CheckResult{Name: checkProjectViewsName, Status: Fail, Message: "no GitHub Project found for owner " + owner}
	}

	// Fetch all view names from the live project.
	query := fmt.Sprintf(`{ node(id: "%s") { ... on ProjectV2 { views(first: 20) { nodes { name } } } } }`, projectNodeID)
	out, err := run("gh", "api", "graphql", "-f", "query="+query, "--jq", `.data.node.views.nodes[].name`)
	if err != nil {
		return CheckResult{Name: checkProjectViewsName, Status: Fail, Message: fmt.Sprintf("failed to fetch views: %v", err)}
	}

	live := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if t := strings.TrimSpace(line); t != "" {
			live[t] = true
		}
	}

	// Load required views from template.
	tmpl, loadErr := bootstrap.LoadProjectTemplate(root)
	if loadErr != nil {
		return CheckResult{Name: checkProjectViewsName, Status: Fail, Message: fmt.Sprintf("could not load project template: %v", loadErr)}
	}

	var missing []string
	for _, req := range tmpl.RequiredViews {
		if !live[req.Name] {
			missing = append(missing, fmt.Sprintf("%q", req.Name))
		}
	}

	if len(missing) > 0 {
		return CheckResult{Name: checkProjectViewsName, Status: Warning,
			Message: "missing: " + strings.Join(missing, ", ")}
	}

	return CheckResult{Name: checkProjectViewsName, Status: Pass}
}

// projectListResponse represents the JSON output from `gh project list --format json`.
type projectListResponse struct {
	Projects []struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Number int    `json:"number"`
		URL    string `json:"url"`
		Owner  struct {
			Login string `json:"login"`
			Type  string `json:"type"` // "User" or "Organization"
		} `json:"owner"`
	} `json:"projects"`
}

// projectEntry holds the resolved project details needed for both GraphQL and REST API calls.
type projectEntry struct {
	NodeID    string // GraphQL node ID (PVT_...)
	Number    int    // REST API project number
	OwnerType string // "User" or "Organization"
	URL       string // web URL for display
}

// resolveProjectEntry resolves the project matching repoName for the given owner
// and returns full details needed for both GraphQL and REST operations.
// Falls back to the first project if no title match. Returns nil if no projects exist.
func resolveProjectEntry(owner, repoName string, run bootstrap.RunCommandFunc) *projectEntry {
	out, err := run("gh", "project", "list", "--owner", owner, "--format", "json", "--limit", "100")
	if err != nil {
		return nil
	}
	var resp projectListResponse
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); jsonErr != nil || len(resp.Projects) == 0 {
		return nil
	}
	p := resp.Projects[0]
	for _, proj := range resp.Projects {
		if proj.Title == repoName {
			p = proj
			break
		}
	}
	return &projectEntry{
		NodeID:    p.ID,
		Number:    p.Number,
		OwnerType: p.Owner.Type,
		URL:       p.URL,
	}
}

// resolveProjectNodeIDViaRun resolves the project node ID for an owner using
// `gh project list --owner`. It matches the project whose title equals repoName.
// If no title matches, it falls back to the first project's ID (preserving
// behaviour for single-project owners). Returns "" if no projects exist.
func resolveProjectNodeIDViaRun(owner, repoName string, run bootstrap.RunCommandFunc) string {
	out, err := run("gh", "project", "list", "--owner", owner, "--format", "json", "--limit", "100")
	if err != nil {
		return ""
	}

	var resp projectListResponse
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); jsonErr != nil {
		return ""
	}

	if len(resp.Projects) == 0 {
		return ""
	}

	// Match by title first.
	for _, p := range resp.Projects {
		if p.Title == repoName {
			return p.ID
		}
	}

	// Fallback: return the first project's ID.
	return resp.Projects[0].ID
}


// checkProjectItemStatusesName is the check name used for project item status verification.
const checkProjectItemStatusesName = "Project items have status assigned"

// CheckProjectItemStatuses verifies that all project items have a status
// field value assigned. Returns Warning if any items have no status, Pass
// otherwise. Uses the same project resolution pattern as CheckProjectStatus.
func CheckProjectItemStatuses(owner, repoName string, run bootstrap.RunCommandFunc) CheckResult {
	// Resolve project node ID.
	projectNodeID := resolveProjectNodeIDViaRun(owner, repoName, run)
	if projectNodeID == "" {
		return CheckResult{
			Name:    checkProjectItemStatusesName,
			Status:  Fail,
			Message: "no GitHub Project found for owner " + owner,
		}
	}

	// Fetch Status field ID.
	fieldQuery := fmt.Sprintf(`{ node(id: "%s") { ... on ProjectV2 { field(name: "Status") { ... on ProjectV2SingleSelectField { id } } } } }`, projectNodeID)
	out, err := run("gh", "api", "graphql", "-f", "query="+fieldQuery, "--jq", ".data.node.field.id")
	if err != nil {
		return CheckResult{
			Name:    checkProjectItemStatusesName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to fetch Status field ID: %v", err),
		}
	}

	fieldID := strings.TrimSpace(out)
	if fieldID == "" || fieldID == "null" {
		return CheckResult{
			Name:    checkProjectItemStatusesName,
			Status:  Fail,
			Message: "Status field not found on project",
		}
	}

	// Fetch all project items.
	items, fetchErr := fetchAllProjectItems(projectNodeID, fieldID, run)
	if fetchErr != nil {
		return CheckResult{
			Name:    checkProjectItemStatusesName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to fetch project items: %v", fetchErr),
		}
	}

	// Count items with no status assigned.
	noStatus := 0
	for _, item := range items {
		if item.CurrentStatus == "" {
			noStatus++
		}
	}

	if noStatus > 0 {
		return CheckResult{
			Name:    checkProjectItemStatusesName,
			Status:  Warning,
			Message: fmt.Sprintf("%d project items have no status — run --repair to fix", noStatus),
		}
	}

	return CheckResult{
		Name:   checkProjectItemStatusesName,
		Status: Pass,
	}
}

// checkProjectCollaboratorName is the check name used for project collaborator verification.
const checkProjectCollaboratorName = "Agent user is a project collaborator"

// CheckProjectCollaborator verifies that the configured agent user is a collaborator
// on the GitHub Project. When ownerType is Organization, it queries org membership
// via `gh api orgs/{owner}/members` instead of the GraphQL project collaborators
// query (which fails for org projects). Returns Pass with note when agentUser is
// empty (skips gracefully).
func CheckProjectCollaborator(owner, repoName, agentUser, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	if agentUser == "" {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Pass,
			Message: "no agent user configured — skipping",
		}
	}

	// Organisation path: query org membership via REST API.
	if ownerType == bootstrap.OwnerTypeOrg {
		out, err := run("gh", "api", fmt.Sprintf("orgs/%s/members", owner), "--jq", ".[].login")
		if err != nil {
			return CheckResult{
				Name:    checkProjectCollaboratorName,
				Status:  Fail,
				Message: fmt.Sprintf("failed to fetch org members: %v", err),
			}
		}

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
			Message: agentUser + " is not an org member of " + owner,
		}
	}

	// User path: query project collaborators via GraphQL.
	projectNodeID := resolveProjectNodeIDViaRun(owner, repoName, run)
	if projectNodeID == "" {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Fail,
			Message: "no GitHub Project found for owner " + owner,
		}
	}

	query := fmt.Sprintf(`{ node(id: "%s") { ... on ProjectV2 { collaborators(first: 100) { nodes { login } } } } }`, projectNodeID)
	out, err := run("gh", "api", "graphql", "-f", "query="+query, "--jq", ".data.node.collaborators.nodes[].login")
	if err != nil {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to fetch collaborators: %v", err),
		}
	}

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

// checkAgenticProjectIDName is the check name for the AGENTIC_PROJECT_ID variable.
const checkAgenticProjectIDName = "AGENTIC_PROJECT_ID is configured"

// CheckAgenticProjectID verifies that the AGENTIC_PROJECT_ID GitHub Actions
// variable is set with topology-aware dual-scope logic. For Organization
// (federated) repos the variable must exist at org level; if found only at
// repo level it fails with a topology message. For User (single) repos it
// may exist at either scope.
func CheckAgenticProjectID(repoFullName, owner, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	if ownerType == bootstrap.OwnerTypeOrg {
		// Federated: must be at org level.
		orgArgs := []string{"variable", "get", "AGENTIC_PROJECT_ID", "--org", owner}
		out, err := run("gh", orgArgs...)
		if err == nil && strings.TrimSpace(out) != "" {
			return CheckResult{Name: checkAgenticProjectIDName, Status: Pass}
		}
		// Not at org — check repo level to detect misplacement.
		repoArgs := []string{"variable", "get", "AGENTIC_PROJECT_ID", "--repo", repoFullName}
		repoOut, repoErr := run("gh", repoArgs...)
		if repoErr == nil && strings.TrimSpace(repoOut) != "" {
			return CheckResult{
				Name:    checkAgenticProjectIDName,
				Status:  Fail,
				Message: "AGENTIC_PROJECT_ID is set at repo level but must be at org level for federated topology",
			}
		}
		return CheckResult{
			Name:    checkAgenticProjectIDName,
			Status:  Fail,
			Message: "AGENTIC_PROJECT_ID variable is not set",
		}
	}

	// Single (User): check repo level first, then org level.
	repoArgs := []string{"variable", "get", "AGENTIC_PROJECT_ID", "--repo", repoFullName}
	out, err := run("gh", repoArgs...)
	if err == nil && strings.TrimSpace(out) != "" {
		return CheckResult{Name: checkAgenticProjectIDName, Status: Pass}
	}
	// Not at repo — check org level (acceptable for single topology).
	orgArgs := []string{"variable", "get", "AGENTIC_PROJECT_ID", "--org", owner}
	orgOut, orgErr := run("gh", orgArgs...)
	if orgErr == nil && strings.TrimSpace(orgOut) != "" {
		return CheckResult{Name: checkAgenticProjectIDName, Status: Pass}
	}
	return CheckResult{
		Name:    checkAgenticProjectIDName,
		Status:  Fail,
		Message: "AGENTIC_PROJECT_ID variable is not set",
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

// ──────────────────────────────────────────────────────────────────────────────
// Stale open issue checks
// ──────────────────────────────────────────────────────────────────────────────

const checkStaleRequirementsName = "No stale open requirements"
const checkStaleFeaturesName = "No stale open features"

type staleIssue struct {
	Number int
	Title  string
}

// fetchStaleOpenIssues returns open issues with the given label whose every
// sub-issue is closed. repoFullName is "owner/repo".
func fetchStaleOpenIssues(repoFullName, label string, run bootstrap.RunCommandFunc) ([]staleIssue, error) {
	// Fetch open issues with the given label.
	out, err := run("gh", "issue", "list",
		"--repo", repoFullName,
		"--label", label,
		"--state", "open",
		"--json", "number,title",
		"--limit", "200",
	)
	if err != nil {
		return nil, fmt.Errorf("listing open %s issues: %w", label, err)
	}

	var issues []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &issues); err != nil {
		return nil, fmt.Errorf("parsing issue list: %w", err)
	}

	var stale []staleIssue
	for _, iss := range issues {
		// Fetch sub-issues for this issue.
		subOut, subErr := run("gh", "issue", "view",
			fmt.Sprintf("%d", iss.Number),
			"--repo", repoFullName,
			"--json", "subIssues",
		)
		if subErr != nil {
			continue // skip if we can't fetch
		}

		var resp struct {
			SubIssues []struct {
				State string `json:"state"`
			} `json:"subIssues"`
		}
		if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(subOut)), &resp); jsonErr != nil {
			continue
		}

		// Skip issues with no sub-issues.
		if len(resp.SubIssues) == 0 {
			continue
		}

		// Check if all sub-issues are closed.
		allClosed := true
		for _, sub := range resp.SubIssues {
			if !strings.EqualFold(sub.State, "CLOSED") {
				allClosed = false
				break
			}
		}
		if allClosed {
			stale = append(stale, staleIssue{Number: iss.Number, Title: iss.Title})
		}
	}
	return stale, nil
}

// CheckStaleOpenRequirements warns when open requirement issues have all their
// feature sub-issues closed.
func CheckStaleOpenRequirements(repoFullName string, run bootstrap.RunCommandFunc) CheckResult {
	stale, err := fetchStaleOpenIssues(repoFullName, "requirement", run)
	if err != nil {
		return CheckResult{Name: checkStaleRequirementsName, Status: Fail, Message: err.Error()}
	}
	if len(stale) == 0 {
		return CheckResult{Name: checkStaleRequirementsName, Status: Pass}
	}
	var msgs []string
	for _, s := range stale {
		msgs = append(msgs, fmt.Sprintf("#%d \"%s\"", s.Number, s.Title))
	}
	return CheckResult{
		Name:    checkStaleRequirementsName,
		Status:  Warning,
		Message: fmt.Sprintf("open requirements with all features closed: %s", strings.Join(msgs, ", ")),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GitHub Actions variable checks
// ──────────────────────────────────────────────────────────────────────────────

// checkAgentUserVarName is the check name used for AGENT_USER variable verification.
const checkAgentUserVarName = "AGENT_USER variable configured"

// Check name constants for pipeline variable and secret checks.
const (
	checkRunnerLabelVarName          = "RUNNER_LABEL variable configured"
	checkGooseProviderVarName        = "GOOSE_PROVIDER variable configured"
	checkGooseModelVarName           = "GOOSE_MODEL variable configured"
	checkGooseAgentPATSecretName     = "GOOSE_AGENT_PAT secret configured"
	checkClaudeCredentialsSecretName = "CLAUDE_CREDENTIALS_JSON secret configured"
)

// variableEntry is used to unmarshal JSON output from `gh variable list`.
type variableEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CheckAgentUserVar verifies that AGENT_USER is configured as a GitHub Actions
// variable with topology-aware dual-scope logic. For Organization (federated)
// repos the variable must exist at org level; if found only at repo level it
// fails with a topology message. For User (single) repos it may exist at
// either scope.
func CheckAgentUserVar(owner, repoName, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	if ownerType == bootstrap.OwnerTypeOrg {
		// Federated: must be at org level.
		found, err := variableExistsAtScope(owner, repoName, "AGENT_USER", "org", run)
		if err == nil && found {
			return CheckResult{
				Name:    checkAgentUserVarName,
				Status:  Pass,
				Message: "configured at org level",
			}
		}
		// Not at org — check repo level to detect misplacement.
		repoFound, _ := variableExistsAtScope(owner, repoName, "AGENT_USER", "repo", run)
		if repoFound {
			return CheckResult{
				Name:    checkAgentUserVarName,
				Status:  Fail,
				Message: "AGENT_USER is set at repo level but must be at org level for federated topology",
			}
		}
		return CheckResult{
			Name:    checkAgentUserVarName,
			Status:  Fail,
			Message: "AGENT_USER variable not set at org or repo level",
		}
	}

	// Single (User): check repo level first, then org level.
	found, err := variableExistsAtScope(owner, repoName, "AGENT_USER", "repo", run)
	if err == nil && found {
		return CheckResult{
			Name:    checkAgentUserVarName,
			Status:  Pass,
			Message: "configured at repo level",
		}
	}
	// Not at repo — check org level (acceptable for single topology).
	orgFound, orgErr := variableExistsAtScope(owner, repoName, "AGENT_USER", "org", run)
	if orgErr == nil && orgFound {
		return CheckResult{
			Name:    checkAgentUserVarName,
			Status:  Pass,
			Message: "configured at org level",
		}
	}
	return CheckResult{
		Name:    checkAgentUserVarName,
		Status:  Fail,
		Message: "AGENT_USER variable not set at org or repo level",
	}
}

// ReadAgentUserVar reads the current AGENT_USER value from GitHub Actions
// variables, checking org level first then repo level. Returns the value or
// empty string if not found. Used by runDoctor to pass agent user to
// downstream checks like CheckProjectCollaborator.
func ReadAgentUserVar(owner, repoName string, run bootstrap.RunCommandFunc) string {
	// Try org-level first.
	out, err := run("gh", "variable", "list", "--org", owner, "--json", "name,value")
	if err == nil {
		var vars []variableEntry
		if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &vars); jsonErr == nil {
			for _, v := range vars {
				if v.Name == "AGENT_USER" {
					return v.Value
				}
			}
		}
	}

	// Try repo-level.
	repoFullName := owner + "/" + repoName
	out, err = run("gh", "variable", "list", "--repo", repoFullName, "--json", "name,value")
	if err == nil {
		var vars []variableEntry
		if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &vars); jsonErr == nil {
			for _, v := range vars {
				if v.Name == "AGENT_USER" {
					return v.Value
				}
			}
		}
	}

	return ""
}

// CheckStaleOpenFeatures warns when open feature issues have all their task
// sub-issues closed.
func CheckStaleOpenFeatures(repoFullName string, run bootstrap.RunCommandFunc) CheckResult {
	stale, err := fetchStaleOpenIssues(repoFullName, "feature", run)
	if err != nil {
		return CheckResult{Name: checkStaleFeaturesName, Status: Fail, Message: err.Error()}
	}
	if len(stale) == 0 {
		return CheckResult{Name: checkStaleFeaturesName, Status: Pass}
	}
	var msgs []string
	for _, s := range stale {
		msgs = append(msgs, fmt.Sprintf("#%d \"%s\"", s.Number, s.Title))
	}
	return CheckResult{
		Name:    checkStaleFeaturesName,
		Status:  Warning,
		Message: fmt.Sprintf("open features with all tasks closed: %s", strings.Join(msgs, ", ")),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Pipeline variable checks
// ──────────────────────────────────────────────────────────────────────────────

// secretEntry is used to unmarshal JSON output from `gh secret list`.
type secretEntry struct {
	Name string `json:"name"`
}

// variableExistsAtScope checks whether varName exists in a variable list at the given scope.
// scope is either "org" or "repo". Returns (found bool, error).
func variableExistsAtScope(owner, repoName, varName, scope string, run bootstrap.RunCommandFunc) (bool, error) {
	var args []string
	if scope == "org" {
		args = []string{"gh", "variable", "list", "--org", owner, "--json", "name"}
	} else {
		repoFullName := owner + "/" + repoName
		args = []string{"gh", "variable", "list", "--repo", repoFullName, "--json", "name"}
	}
	out, err := run(args[0], args[1:]...)
	if err != nil {
		return false, err
	}
	var vars []variableEntry
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &vars); jsonErr != nil {
		return false, jsonErr
	}
	for _, v := range vars {
		if v.Name == varName {
			return true, nil
		}
	}
	return false, nil
}

// checkRepoVariable checks whether a named variable exists with topology-aware
// dual-scope logic. For Organization (federated) repos: the variable must exist
// at org level; if found only at repo level, it fails with a topology message.
// For User (single) repos: the variable may exist at either repo or org level.
// Returns Pass if found at the correct scope, Fail if misplaced, Warning if missing.
func checkRepoVariable(owner, repoName, varName, checkName, defaultValue, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	if ownerType == bootstrap.OwnerTypeOrg {
		// Federated: must be at org level.
		found, err := variableExistsAtScope(owner, repoName, varName, "org", run)
		if err != nil {
			return CheckResult{
				Name:    checkName,
				Status:  Warning,
				Message: fmt.Sprintf("could not list org variables: %v", err),
			}
		}
		if found {
			return CheckResult{Name: checkName, Status: Pass}
		}
		// Not at org — check repo level to detect misplacement.
		repoFound, repoErr := variableExistsAtScope(owner, repoName, varName, "repo", run)
		if repoErr != nil {
			// Could not check repo level; report as missing from org.
			return CheckResult{
				Name:    checkName,
				Status:  Warning,
				Message: fmt.Sprintf("%s not set — repair will set to %q", varName, defaultValue),
			}
		}
		if repoFound {
			return CheckResult{
				Name:    checkName,
				Status:  Fail,
				Message: fmt.Sprintf("%s is set at repo level but must be at org level for federated topology", varName),
			}
		}
		return CheckResult{
			Name:    checkName,
			Status:  Warning,
			Message: fmt.Sprintf("%s not set — repair will set to %q", varName, defaultValue),
		}
	}

	// Single (User): check repo level first, then org level.
	found, err := variableExistsAtScope(owner, repoName, varName, "repo", run)
	if err != nil {
		return CheckResult{
			Name:    checkName,
			Status:  Warning,
			Message: fmt.Sprintf("could not list repo variables: %v", err),
		}
	}
	if found {
		return CheckResult{Name: checkName, Status: Pass}
	}
	// Not at repo — check org level (acceptable for single topology).
	orgFound, orgErr := variableExistsAtScope(owner, repoName, varName, "org", run)
	if orgErr == nil && orgFound {
		return CheckResult{Name: checkName, Status: Pass}
	}
	return CheckResult{
		Name:    checkName,
		Status:  Warning,
		Message: fmt.Sprintf("%s not set — repair will set to %q", varName, defaultValue),
	}
}

// secretExistsAtScope checks whether secretName exists in a secret list at the given scope.
// scope is either "org" or "repo". Returns (found bool, error).
func secretExistsAtScope(owner, repoName, secretName, scope string, run bootstrap.RunCommandFunc) (bool, error) {
	var args []string
	if scope == "org" {
		args = []string{"gh", "secret", "list", "--org", owner, "--json", "name"}
	} else {
		repoFullName := owner + "/" + repoName
		args = []string{"gh", "secret", "list", "--repo", repoFullName, "--json", "name"}
	}
	out, err := run(args[0], args[1:]...)
	if err != nil {
		return false, err
	}
	var secrets []secretEntry
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(out)), &secrets); jsonErr != nil {
		return false, jsonErr
	}
	for _, s := range secrets {
		if s.Name == secretName {
			return true, nil
		}
	}
	return false, nil
}

// checkRepoSecret checks whether a named secret exists with topology-aware
// dual-scope logic. For Organization (federated) repos: the secret must exist
// at org level; if found only at repo level, it fails with a topology message.
// For User (single) repos: the secret may exist at either repo or org level.
// Returns Pass if found at the correct scope, Fail if misplaced, Warning if missing.
func checkRepoSecret(owner, repoName, secretName, checkName, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	if ownerType == bootstrap.OwnerTypeOrg {
		// Federated: must be at org level.
		found, err := secretExistsAtScope(owner, repoName, secretName, "org", run)
		if err != nil {
			return CheckResult{
				Name:    checkName,
				Status:  Warning,
				Message: fmt.Sprintf("could not list org secrets: %v", err),
			}
		}
		if found {
			return CheckResult{Name: checkName, Status: Pass}
		}
		// Not at org — check repo level to detect misplacement.
		repoFound, repoErr := secretExistsAtScope(owner, repoName, secretName, "repo", run)
		if repoErr != nil {
			return CheckResult{
				Name:    checkName,
				Status:  Warning,
				Message: fmt.Sprintf("%s secret not set", secretName),
			}
		}
		if repoFound {
			return CheckResult{
				Name:    checkName,
				Status:  Fail,
				Message: fmt.Sprintf("%s is set at repo level but must be at org level for federated topology", secretName),
			}
		}
		return CheckResult{
			Name:    checkName,
			Status:  Warning,
			Message: fmt.Sprintf("%s secret not set", secretName),
		}
	}

	// Single (User): check repo level first, then org level.
	found, err := secretExistsAtScope(owner, repoName, secretName, "repo", run)
	if err != nil {
		return CheckResult{
			Name:    checkName,
			Status:  Warning,
			Message: fmt.Sprintf("could not list repo secrets: %v", err),
		}
	}
	if found {
		return CheckResult{Name: checkName, Status: Pass}
	}
	// Not at repo — check org level (acceptable for single topology).
	orgFound, orgErr := secretExistsAtScope(owner, repoName, secretName, "org", run)
	if orgErr == nil && orgFound {
		return CheckResult{Name: checkName, Status: Pass}
	}
	return CheckResult{
		Name:    checkName,
		Status:  Warning,
		Message: fmt.Sprintf("%s secret not set", secretName),
	}
}

// CheckRunnerLabelVar verifies that RUNNER_LABEL is configured as a variable.
// When ownerType is Organization, checks at org level; otherwise repo level.
func CheckRunnerLabelVar(owner, repoName, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	return checkRepoVariable(owner, repoName, "RUNNER_LABEL", checkRunnerLabelVarName, bootstrap.DefaultRunnerLabel, ownerType, run)
}

// CheckGooseProviderVar verifies that GOOSE_PROVIDER is configured as a variable.
// When ownerType is Organization, checks at org level; otherwise repo level.
func CheckGooseProviderVar(owner, repoName, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	return checkRepoVariable(owner, repoName, "GOOSE_PROVIDER", checkGooseProviderVarName, bootstrap.DefaultGooseProvider, ownerType, run)
}

// CheckGooseModelVar verifies that GOOSE_MODEL is configured as a variable.
// When ownerType is Organization, checks at org level; otherwise repo level.
func CheckGooseModelVar(owner, repoName, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	return checkRepoVariable(owner, repoName, "GOOSE_MODEL", checkGooseModelVarName, bootstrap.DefaultGooseModel, ownerType, run)
}

// CheckGooseAgentPATSecret verifies that GOOSE_AGENT_PAT is configured as a secret.
// When ownerType is Organization, checks at org level; otherwise repo level.
func CheckGooseAgentPATSecret(owner, repoName, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	return checkRepoSecret(owner, repoName, "GOOSE_AGENT_PAT", checkGooseAgentPATSecretName, ownerType, run)
}

// CheckClaudeCredentialsSecret verifies that CLAUDE_CREDENTIALS_JSON is configured as a secret.
// When ownerType is Organization, checks at org level; otherwise repo level.
func CheckClaudeCredentialsSecret(owner, repoName, ownerType string, run bootstrap.RunCommandFunc) CheckResult {
	return checkRepoSecret(owner, repoName, "CLAUDE_CREDENTIALS_JSON", checkClaudeCredentialsSecretName, ownerType, run)
}
