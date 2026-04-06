package verify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/sync"
)

// ConfirmFunc prompts the user for a text input and returns the value.
type ConfirmFunc func(prompt string) (string, error)

// standardCLAUDEMD is the standard content for CLAUDE.md.
const standardCLAUDEMD = `# CLAUDE.md

This project uses AGENTS.md as the single source of truth for agent instructions.
All development rules, workflows, and session protocols are defined there.

@base/AGENTS.md
@AGENTS.local.md
`

// skeletonAGENTSLocalMD is the minimal skeleton for AGENTS.local.md.
const skeletonAGENTSLocalMD = `# AGENTS.local.md — Local Overrides

This file contains project-specific rules and overrides that extend or
supersede the global protocol defined in ` + "`base/AGENTS.md`" + `.

This file is never overwritten by a template sync.

---

## Template Source

<!-- TODO: Add template source -->

## Project

<!-- TODO: Add project details -->
`

// emptyREPOSMD is the standard empty REPOS.md for embedded topology.
const emptyREPOSMD = `# REPOS.md — Repository Registry

This is an embedded topology project — a single repo containing both the agentic
control plane and the project codebase. There are no separate domain or tool repos.

For organisation topology projects, this file lists all repos in the solution.

---

<!-- No entries — embedded topology -->
`

// minimalREADMEMD is a minimal README.md placeholder.
const minimalREADMEMD = `# Project

<!-- TODO: Add project name and description -->

## Setup

See ` + "`docs/PROJECT_BRIEF.md`" + ` for project context.

## Agent sessions

This repo uses the [agentic development framework](https://github.com/eddiecarpenter/agentic-development).
See ` + "`base/AGENTS.md`" + ` and ` + "`AGENTS.local.md`" + ` for session protocols.
`

// RepairCLAUDEMD restores CLAUDE.md with standard content.
func RepairCLAUDEMD(root string) CheckResult {
	path := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(path, []byte(standardCLAUDEMD), 0o644); err != nil {
		return CheckResult{
			Name:    "CLAUDE.md exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "CLAUDE.md exists",
		Status: Pass,
	}
}

// RepairAGENTSLocalMD restores AGENTS.local.md with a minimal skeleton.
func RepairAGENTSLocalMD(root string) CheckResult {
	path := filepath.Join(root, "AGENTS.local.md")
	if err := os.WriteFile(path, []byte(skeletonAGENTSLocalMD), 0o644); err != nil {
		return CheckResult{
			Name:    "AGENTS.local.md exists",
			Status:  Warning,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "AGENTS.local.md exists",
		Status: Pass,
	}
}

// RepairSkillsDir creates the skills/ directory with a .gitkeep file and stages it.
// run is injected so tests can substitute a fake implementation.
func RepairSkillsDir(root string, run bootstrap.RunCommandFunc) CheckResult {
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return CheckResult{
			Name:    "skills/ directory exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}

	gitkeepPath := filepath.Join(skillsDir, ".gitkeep")
	if err := os.WriteFile(gitkeepPath, []byte{}, 0o644); err != nil {
		return CheckResult{
			Name:    "skills/ directory exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}

	// Stage the file via git add.
	_, err := run("bash", "-c", fmt.Sprintf("cd '%s' && git add skills/.gitkeep", strings.ReplaceAll(root, "'", "'\\''")))
	if err != nil {
		return CheckResult{
			Name:    "skills/ directory exists",
			Status:  Fail,
			Message: fmt.Sprintf("git add failed: %v", err),
		}
	}

	return CheckResult{
		Name:   "skills/ directory exists",
		Status: Pass,
	}
}

// RepairTEMPLATESOURCE prompts the user for the template source value and writes it.
// confirmFn is injected so tests can substitute a fake implementation.
func RepairTEMPLATESOURCE(root string, confirmFn ConfirmFunc) CheckResult {
	if confirmFn == nil {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: "cannot prompt for value — no confirm function provided",
		}
	}

	value, err := confirmFn("Enter template source repo (e.g. eddiecarpenter/agentic-development)")
	if err != nil {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: fmt.Sprintf("prompt failed: %v", err),
		}
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: "no value provided",
		}
	}

	path := filepath.Join(root, "TEMPLATE_SOURCE")
	if err := os.WriteFile(path, []byte(value+"\n"), 0o644); err != nil {
		return CheckResult{
			Name:    "TEMPLATE_SOURCE exists",
			Status:  Warning,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "TEMPLATE_SOURCE exists",
		Status: Pass,
	}
}

// RepairTEMPLATEVERSION fetches the latest tag from the template repo and writes it.
// run is injected so tests can substitute a fake implementation.
func RepairTEMPLATEVERSION(root string, run bootstrap.RunCommandFunc) CheckResult {
	// First check if TEMPLATE_SOURCE exists — we need it to fetch the version.
	sourcePath := filepath.Join(root, "TEMPLATE_SOURCE")
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: "cannot determine version — TEMPLATE_SOURCE is missing",
		}
	}

	repo := strings.TrimSpace(string(data))
	if repo == "" {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: "TEMPLATE_SOURCE is empty",
		}
	}

	// Fetch latest release tag.
	out, err := run("gh", "release", "list", "--repo", repo, "--limit", "1", "--json", "tagName", "--jq", ".[0].tagName")
	if err != nil {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: fmt.Sprintf("failed to fetch latest tag: %v", err),
		}
	}

	tag := strings.TrimSpace(out)
	if tag == "" {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: "no releases found in template repo",
		}
	}

	path := filepath.Join(root, "TEMPLATE_VERSION")
	if err := os.WriteFile(path, []byte(tag+"\n"), 0o644); err != nil {
		return CheckResult{
			Name:    "TEMPLATE_VERSION exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}

	return CheckResult{
		Name:   "TEMPLATE_VERSION exists",
		Status: Pass,
	}
}

// RepairREPOSMD creates an empty REPOS.md with embedded topology comment.
func RepairREPOSMD(root string) CheckResult {
	path := filepath.Join(root, "REPOS.md")
	if err := os.WriteFile(path, []byte(emptyREPOSMD), 0o644); err != nil {
		return CheckResult{
			Name:    "REPOS.md exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "REPOS.md exists",
		Status: Pass,
	}
}

// RepairREADMEMD creates a minimal README.md with placeholder content.
func RepairREADMEMD(root string) CheckResult {
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte(minimalREADMEMD), 0o644); err != nil {
		return CheckResult{
			Name:    "README.md exists",
			Status:  Fail,
			Message: fmt.Sprintf("repair failed: %v", err),
		}
	}
	return CheckResult{
		Name:   "README.md exists",
		Status: Pass,
	}
}

// RepairGhNotify runs install-gh-notify.sh to install or repair the
// gh-notify LaunchAgent. root is the repo root, run is injected for
// command execution.
func RepairGhNotify(root string, run bootstrap.RunCommandFunc) CheckResult {
	const checkName = "gh-notify LaunchAgent installed"

	scriptPath := filepath.Join(root, "base", "scripts", "install-gh-notify.sh")
	_, err := run("bash", scriptPath)
	if err != nil {
		return CheckResult{
			Name:    checkName,
			Status:  Fail,
			Message: fmt.Sprintf("install script failed: %v", err),
		}
	}

	return CheckResult{
		Name:    checkName,
		Status:  Pass,
		Message: "gh-notify installed and running",
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GitHub Project status repair
// ──────────────────────────────────────────────────────────────────────────────

// RepairProjectStatus applies the canonical status options from base/project-template.json
// to the GitHub Project via GraphQL mutation. Uses run to shell out to gh api graphql.
func RepairProjectStatus(owner, repoName, root string, run bootstrap.RunCommandFunc) CheckResult {
	// Load canonical options from project template.
	tmpl, loadErr := bootstrap.LoadProjectTemplate(root)
	if loadErr != nil {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: fmt.Sprintf("could not load project template: %v", loadErr),
		}
	}

	// Step 1: Find the project node ID.
	projectNodeID := resolveProjectNodeIDViaRun(owner, repoName, run)
	if projectNodeID == "" {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: "no GitHub Project found for owner " + owner,
		}
	}

	// Step 2: Fetch the Status field ID.
	fieldQuery := fmt.Sprintf(`{ node(id: "%s") { ... on ProjectV2 { field(name: "Status") { ... on ProjectV2SingleSelectField { id } } } } }`, projectNodeID)
	out, err := run("gh", "api", "graphql", "-f", "query="+fieldQuery, "--jq", ".data.node.field.id")
	if err != nil {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to fetch Status field ID: %v", err),
		}
	}

	fieldID := strings.TrimSpace(out)
	if fieldID == "" || fieldID == "null" {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: "Status field not found on project",
		}
	}

	// Step 3: Build the mutation with options from project template.
	var optionEntries []string
	for _, opt := range tmpl.StatusOptions {
		optionEntries = append(optionEntries, fmt.Sprintf(`{name: "%s", color: %s, description: "%s"}`, opt.Name, opt.Color, opt.Description))
	}
	optionsStr := strings.Join(optionEntries, ", ")

	mutation := fmt.Sprintf(`mutation { updateProjectV2Field(input: { fieldId: "%s", projectId: "%s", singleSelectOptions: [%s] }) { field { ... on ProjectV2SingleSelectField { id } } } }`,
		fieldID, projectNodeID, optionsStr)

	out, err = run("gh", "api", "graphql", "-f", "query="+mutation)
	if err != nil {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to update status options: %v — %s", err, strings.TrimSpace(out)),
		}
	}

	// Phase 2: resync individual item statuses.
	resyncUpdated, _, resyncErr := resyncProjectItemStatuses(owner, projectNodeID, fieldID, tmpl, run)
	if resyncErr != nil {
		return CheckResult{
			Name:    checkProjectStatusName,
			Status:  Fail,
			Message: fmt.Sprintf("status options updated but item resync failed: %v", resyncErr),
		}
	}

	msg := ""
	if resyncUpdated > 0 {
		msg = fmt.Sprintf("%d item(s) resynced", resyncUpdated)
	}
	return CheckResult{
		Name:    checkProjectStatusName,
		Status:  Pass,
		Message: msg,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Project item status resync
// ──────────────────────────────────────────────────────────────────────────────

// stalePipelineLabels are labels that must be removed from a CLOSED issue
// and replaced with "done".
var stalePipelineLabels = []string{"backlog", "scoping", "scheduled", "in-design", "in-development", "in-review"}

// pipelineLabelPriority maps pipeline labels to their canonical status name.
// The order defines priority: earlier entries win. CLOSED state is handled
// first in resolveStatus and always returns Done.
var pipelineLabelPriority = []struct {
	label  string
	status string
}{
	{"done", "Done"},
	{"in-review", "In Review"},
	{"in-development", "In Development"},
	{"in-design", "In Design"},
	{"scheduled", "Scheduled"},
	{"scoping", "Scoping"},
}

// resolveStatus determines the canonical status name for a project item based
// on its issue labels and state. Priority order:
//  1. CLOSED issue state → Done (always; overrides all labels)
//  2. done label → Done
//  3. in-review → In Review
//  4. in-development → In Development
//  5. in-design → In Design
//  6. scheduled → Scheduled
//  7. scoping → Scoping
//  8. backlog label or default → Backlog
func resolveStatus(labels []string, issueState string) string {
	// Rule 1: CLOSED state is always Done, regardless of labels.
	if strings.EqualFold(issueState, "CLOSED") {
		return "Done"
	}

	labelSet := make(map[string]bool, len(labels))
	for _, l := range labels {
		labelSet[l] = true
	}

	for _, entry := range pipelineLabelPriority {
		if labelSet[entry.label] {
			return entry.status
		}
	}

	// Rule 8: backlog label or default → Backlog.
	return "Backlog"
}

// projectItem represents a single item from a ProjectV2 items query.
type projectItem struct {
	ID            string
	IssueNumber   int
	RepoFullName  string
	IssueState    string
	Labels        []string
	CurrentStatus string
}

// resyncProjectItemStatuses fetches all project items (paginated) and updates
// each item's status field to match the canonical status derived from its issue
// labels and state. Returns counts of updated and already-correct items.
func resyncProjectItemStatuses(owner, projectID, fieldID string, tmpl *bootstrap.ProjectTemplate, run bootstrap.RunCommandFunc) (updated int, correct int, err error) {
	// Build option name → ID map by fetching status field options.
	optionMap, optErr := fetchStatusOptionMap(projectID, fieldID, run)
	if optErr != nil {
		return 0, 0, optErr
	}

	// Fetch all project items with pagination.
	items, fetchErr := fetchAllProjectItems(projectID, fieldID, run)
	if fetchErr != nil {
		return 0, 0, fetchErr
	}

	for _, item := range items {
		wantStatus := resolveStatus(item.Labels, item.IssueState)
		wantOptionID, ok := optionMap[wantStatus]
		if !ok {
			// Status not found in options — skip.
			continue
		}

		// For CLOSED issues: repair stale pipeline labels.
		// Remove any active pipeline labels and ensure "done" is present.
		if strings.EqualFold(item.IssueState, "CLOSED") && item.IssueNumber > 0 && item.RepoFullName != "" {
			labelSet := make(map[string]bool, len(item.Labels))
			for _, l := range item.Labels {
				labelSet[l] = true
			}
			var toRemove []string
			for _, l := range stalePipelineLabels {
				if labelSet[l] {
					toRemove = append(toRemove, l)
				}
			}
			needsDone := !labelSet["done"]
			if len(toRemove) > 0 || needsDone {
				args := []string{"issue", "edit",
					fmt.Sprintf("%d", item.IssueNumber),
					"--repo", item.RepoFullName,
				}
				for _, l := range toRemove {
					args = append(args, "--remove-label", l)
				}
				if needsDone {
					args = append(args, "--add-label", "done")
				}
				if _, labelErr := run("gh", args...); labelErr != nil {
					return updated, correct, fmt.Errorf("fixing labels on issue %d: %w", item.IssueNumber, labelErr)
				}
			}
		}

		// Check if current status already matches.
		if item.CurrentStatus == wantStatus {
			correct++
			continue
		}

		// Update the item's status.
		mutation := fmt.Sprintf(`mutation { updateProjectV2ItemFieldValue(input: { projectId: "%s", itemId: "%s", fieldId: "%s", value: { singleSelectOptionId: "%s" } }) { clientMutationId } }`,
			projectID, item.ID, fieldID, wantOptionID)
		_, mutErr := run("gh", "api", "graphql", "-f", "query="+mutation)
		if mutErr != nil {
			return updated, correct, fmt.Errorf("updating item %s: %w", item.ID, mutErr)
		}
		updated++
	}

	return updated, correct, nil
}

// fetchStatusOptionMap fetches the Status field options and returns a map of
// option name → option ID.
func fetchStatusOptionMap(projectID, fieldID string, run bootstrap.RunCommandFunc) (map[string]string, error) {
	query := fmt.Sprintf(`{ node(id: "%s") { ... on ProjectV2 { field(name: "Status") { ... on ProjectV2SingleSelectField { options { id name } } } } } }`, projectID)
	out, err := run("gh", "api", "graphql", "-f", "query="+query, "--jq", `.data.node.field.options[] | "\(.id)|\(.name)"`)
	if err != nil {
		return nil, fmt.Errorf("fetching status options: %w", err)
	}

	optionMap := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			optionMap[parts[1]] = parts[0]
		}
	}
	return optionMap, nil
}

// fetchAllProjectItems fetches all items from a ProjectV2 via paginated GraphQL.
// Each item includes its ID, content (issue state + labels), and current status.
func fetchAllProjectItems(projectID, fieldID string, run bootstrap.RunCommandFunc) ([]projectItem, error) {
	var items []projectItem
	cursor := ""

	for {
		afterClause := ""
		if cursor != "" {
			afterClause = fmt.Sprintf(`, after: "%s"`, cursor)
		}

		query := fmt.Sprintf(`{ node(id: "%s") { ... on ProjectV2 { items(first: 100%s) { pageInfo { hasNextPage endCursor } nodes { id content { ... on Issue { number repository { nameWithOwner } state labels(first: 20) { nodes { name } } } } fieldValues(first: 20) { nodes { ... on ProjectV2ItemFieldSingleSelectValue { field { ... on ProjectV2SingleSelectField { id } } name } } } } } } } }`,
			projectID, afterClause)

		out, err := run("gh", "api", "graphql", "-f", "query="+query)
		if err != nil {
			return nil, fmt.Errorf("fetching project items: %w", err)
		}

		// Parse the JSON response.
		type gqlResponse struct {
			Data struct {
				Node struct {
					Items struct {
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []struct {
							ID      string `json:"id"`
							Content struct {
								Number     int    `json:"number"`
								Repository struct {
									NameWithOwner string `json:"nameWithOwner"`
								} `json:"repository"`
								State  string `json:"state"`
								Labels struct {
									Nodes []struct {
										Name string `json:"name"`
									} `json:"nodes"`
								} `json:"labels"`
							} `json:"content"`
							FieldValues struct {
								Nodes []struct {
									Field struct {
										ID string `json:"id"`
									} `json:"field"`
									Name string `json:"name"`
								} `json:"nodes"`
							} `json:"fieldValues"`
						} `json:"nodes"`
					} `json:"items"`
				} `json:"node"`
			} `json:"data"`
		}

		var resp gqlResponse
		if jsonErr := json.Unmarshal([]byte(out), &resp); jsonErr != nil {
			return nil, fmt.Errorf("parsing project items response: %w", jsonErr)
		}

		for _, node := range resp.Data.Node.Items.Nodes {
			item := projectItem{
				ID:           node.ID,
				IssueNumber:  node.Content.Number,
				RepoFullName: node.Content.Repository.NameWithOwner,
				IssueState:   node.Content.State,
			}
			for _, label := range node.Content.Labels.Nodes {
				item.Labels = append(item.Labels, label.Name)
			}
			// Find current status from field values.
			for _, fv := range node.FieldValues.Nodes {
				if fv.Field.ID == fieldID {
					item.CurrentStatus = fv.Name
					break
				}
			}
			items = append(items, item)
		}

		if !resp.Data.Node.Items.PageInfo.HasNextPage {
			break
		}
		cursor = resp.Data.Node.Items.PageInfo.EndCursor
	}

	return items, nil
}

// ResyncProjectItemStatuses is the exported entry point for resyncing all project
// item statuses. It resolves the project node ID, fetches the status field ID and
// options, and calls resyncProjectItemStatuses.
func ResyncProjectItemStatuses(owner, repoName, root string, run bootstrap.RunCommandFunc) (updated int, correct int, err error) {
	tmpl, loadErr := bootstrap.LoadProjectTemplate(root)
	if loadErr != nil {
		return 0, 0, fmt.Errorf("loading project template: %w", loadErr)
	}

	projectNodeID := resolveProjectNodeIDViaRun(owner, repoName, run)
	if projectNodeID == "" {
		return 0, 0, fmt.Errorf("no GitHub Project found for owner %s", owner)
	}

	// Fetch Status field ID.
	fieldQuery := fmt.Sprintf(`{ node(id: "%s") { ... on ProjectV2 { field(name: "Status") { ... on ProjectV2SingleSelectField { id } } } } }`, projectNodeID)
	out, fErr := run("gh", "api", "graphql", "-f", "query="+fieldQuery, "--jq", ".data.node.field.id")
	if fErr != nil {
		return 0, 0, fmt.Errorf("fetching Status field ID: %w", fErr)
	}

	fieldID := strings.TrimSpace(out)
	if fieldID == "" || fieldID == "null" {
		return 0, 0, fmt.Errorf("Status field not found on project")
	}

	return resyncProjectItemStatuses(owner, projectNodeID, fieldID, tmpl, run)
}

// RepairProjectItemStatuses resyncs all project item statuses from issue labels
// and state. It wraps ResyncProjectItemStatuses as a repair action.
func RepairProjectItemStatuses(owner, repoName, root string, run bootstrap.RunCommandFunc) CheckResult {
	updated, correct, err := ResyncProjectItemStatuses(owner, repoName, root, run)
	if err != nil {
		return CheckResult{
			Name:    checkProjectItemStatusesName,
			Status:  Fail,
			Message: fmt.Sprintf("resync failed: %v", err),
		}
	}
	return CheckResult{
		Name:    checkProjectItemStatusesName,
		Status:  Pass,
		Message: fmt.Sprintf("%d item(s) updated, %d already correct", updated, correct),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// GitHub Project collaborator repair
// ──────────────────────────────────────────────────────────────────────────────

// RepairProjectCollaborator adds the configured agent user as a WRITER on the
// GitHub Project. Uses run to shell out to gh api graphql.
func RepairProjectCollaborator(owner, repoName, agentUser string, run bootstrap.RunCommandFunc) CheckResult {
	if agentUser == "" {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Pass,
			Message: "no agent user configured",
		}
	}

	// Find the project node ID.
	projectNodeID := resolveProjectNodeIDViaRun(owner, repoName, run)
	if projectNodeID == "" {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Fail,
			Message: "no GitHub Project found for owner " + owner,
		}
	}

	// Resolve the agent user's GitHub node ID.
	userQuery := fmt.Sprintf(`{ user(login: "%s") { id } }`, agentUser)
	out, err := run("gh", "api", "graphql", "-f", "query="+userQuery, "--jq", ".data.user.id")
	if err != nil {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to resolve user %s: %v", agentUser, err),
		}
	}

	userID := strings.TrimSpace(out)
	if userID == "" || userID == "null" {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Fail,
			Message: "user " + agentUser + " not found on GitHub",
		}
	}

	// Invite as project collaborator with WRITER role.
	mutation := fmt.Sprintf(`mutation { updateProjectV2Collaborators(input: { projectId: "%s", collaborators: [{ userId: "%s", role: WRITER }] }) { clientMutationId } }`,
		projectNodeID, userID)
	out, err = run("gh", "api", "graphql", "-f", "query="+mutation)
	if err != nil {
		return CheckResult{
			Name:    checkProjectCollaboratorName,
			Status:  Fail,
			Message: fmt.Sprintf("failed to add collaborator: %v — %s", err, strings.TrimSpace(out)),
		}
	}

	return CheckResult{
		Name:   checkProjectCollaboratorName,
		Status: Pass,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Directory integrity repairs
// ──────────────────────────────────────────────────────────────────────────────

// BoolConfirmFunc prompts the user for a yes/no answer.
type BoolConfirmFunc func(prompt string) (bool, error)

// RepairBaseDir re-syncs base/ from the template after prompting the user.
// run is injected for git operations, confirmFn for user prompt.
func RepairBaseDir(root string, run bootstrap.RunCommandFunc, confirmFn BoolConfirmFunc) CheckResult {
	return RepairBaseDirWithWriter(io.Discard, root, run, confirmFn)
}

// RepairBaseDirWithWriter is like RepairBaseDir but writes sync output to w.
func RepairBaseDirWithWriter(w io.Writer, root string, run bootstrap.RunCommandFunc, confirmFn BoolConfirmFunc) CheckResult {
	if confirmFn != nil {
		ok, err := confirmFn("base/ has issues — re-sync from template?")
		if err != nil || !ok {
			return CheckResult{
				Name:    "base/ exists and is unmodified",
				Status:  Fail,
				Message: "user declined re-sync",
			}
		}
	}

	baseDir := filepath.Join(root, "base")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		// base/ has never been synced — call sync.RunSync directly with force=true
		// and an auto-confirm so it runs non-interactively.
		autoConfirm := func(_ string) (bool, error) { return true, nil }
		if syncErr := sync.RunSync(w, root, run, sync.DefaultFetchRelease, sync.DefaultSpinner, autoConfirm, true); syncErr != nil {
			return CheckResult{
				Name:    "base/ exists and is unmodified",
				Status:  Fail,
				Message: fmt.Sprintf("sync failed: %v", syncErr),
			}
		}
	} else {
		// base/ exists but has local modifications — restore from git.
		_, err := run("bash", "-c", fmt.Sprintf("cd '%s' && git checkout HEAD -- base/", strings.ReplaceAll(root, "'", "'\\''")))
		if err != nil {
			return CheckResult{
				Name:    "base/ exists and is unmodified",
				Status:  Fail,
				Message: fmt.Sprintf("git checkout failed: %v", err),
			}
		}
	}

	return CheckResult{
		Name:   "base/ exists and is unmodified",
		Status: Pass,
	}
}

// RepairBaseRecipes restores base/skills/ to its committed state after prompting.
func RepairBaseRecipes(root string, run bootstrap.RunCommandFunc, confirmFn BoolConfirmFunc) CheckResult {
	skillsDir := filepath.Join(root, "base", "skills")

	// If base/skills/ is simply absent (e.g. base/ was just synced this run
	// and the directory now exists on disk via the sync commit), there is
	// nothing to restore — the files are already correct.
	if _, err := os.Stat(skillsDir); err == nil {
		return CheckResult{
			Name:   "base/skills/*.md unmodified",
			Status: Pass,
		}
	}

	// base/skills/ is absent and not on disk — try to restore from git.
	if confirmFn != nil {
		ok, err := confirmFn("base/skills/ is missing — restore from git?")
		if err != nil || !ok {
			return CheckResult{
				Name:    "base/skills/*.md unmodified",
				Status:  Warning,
				Message: "user declined restore",
			}
		}
	}

	_, err := run("bash", "-c", fmt.Sprintf("cd '%s' && git checkout HEAD -- base/skills/", strings.ReplaceAll(root, "'", "'\\''")))
	if err != nil {
		return CheckResult{
			Name:    "base/skills/*.md unmodified",
			Status:  Warning,
			Message: fmt.Sprintf("git checkout failed: %v", err),
		}
	}

	return CheckResult{
		Name:   "base/skills/*.md unmodified",
		Status: Pass,
	}
}

// RepairGooseRecipes fetches missing recipe YAML files from the template repo
// and writes them into .goose/recipes/. Reads TEMPLATE_SOURCE to know which
// repo to fetch from. Uses run to shell out to `gh api`.
func RepairGooseRecipes(root string) CheckResult {
	recipesPath := filepath.Join(root, ".goose", "recipes")
	if err := os.MkdirAll(recipesPath, 0o755); err != nil {
		return CheckResult{
			Name:    ".goose/recipes/ exists and complete",
			Status:  Fail,
			Message: fmt.Sprintf("could not create directory: %v", err),
		}
	}

	// Read TEMPLATE_SOURCE to know which repo to fetch from.
	sourceData, err := os.ReadFile(filepath.Join(root, "TEMPLATE_SOURCE"))
	if err != nil {
		return CheckResult{
			Name:    ".goose/recipes/ exists and complete",
			Status:  Fail,
			Message: "TEMPLATE_SOURCE missing — cannot fetch recipes from template",
		}
	}
	templateRepo := strings.TrimSpace(string(sourceData))

	var stillMissing []string
	for _, name := range expectedRecipeYAMLs {
		dst := filepath.Join(recipesPath, name)
		// Always fetch and overwrite — recipe updates from the template must
		// flow through to deployed repos (see issue #127).
		content, fetchErr := fetchFileFn(templateRepo, ".goose/recipes/"+name)
		if fetchErr != nil {
			stillMissing = append(stillMissing, name)
			continue
		}
		if writeErr := os.WriteFile(dst, content, 0o644); writeErr != nil {
			stillMissing = append(stillMissing, name)
		}
	}

	if len(stillMissing) > 0 {
		return CheckResult{
			Name:    ".goose/recipes/ exists and complete",
			Status:  Fail,
			Message: fmt.Sprintf("could not restore: %s", strings.Join(stillMissing, ", ")),
		}
	}
	return CheckResult{
		Name:   ".goose/recipes/ exists and complete",
		Status: Pass,
	}
}

// RepairWorkflows copies workflow files from base/.github/workflows/ to
// .github/workflows/ (overwriting existing), then stages them with git add.
// Falls back to fetching from the template repo when base/.github/workflows/
// is absent. run is injected for git operations.
func RepairWorkflows(root string, run bootstrap.RunCommandFunc) CheckResult {
	const checkName = ".github/workflows/ exists and complete"

	workflowsPath := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowsPath, 0o755); err != nil {
		return CheckResult{
			Name:    checkName,
			Status:  Fail,
			Message: fmt.Sprintf("could not create directory: %v", err),
		}
	}

	baseWorkflowsPath := filepath.Join(root, "base", ".github", "workflows")

	// If base/.github/workflows/ exists, copy ALL files from it (overwriting).
	if info, err := os.Stat(baseWorkflowsPath); err == nil && info.IsDir() {
		if err := copyDir(baseWorkflowsPath, workflowsPath); err != nil {
			return CheckResult{
				Name:    checkName,
				Status:  Fail,
				Message: fmt.Sprintf("copying from base: %v", err),
			}
		}

		// Stage the changes.
		quotedRoot := "'" + strings.ReplaceAll(root, "'", "'\\''") + "'"
		_, err := run("bash", "-c", "cd "+quotedRoot+" && git add .github/workflows/")
		if err != nil {
			return CheckResult{
				Name:    checkName,
				Status:  Fail,
				Message: fmt.Sprintf("git add failed: %v", err),
			}
		}

		return CheckResult{
			Name:   checkName,
			Status: Pass,
		}
	}

	// Fallback: fetch missing files from template repo.
	sourceData, _ := os.ReadFile(filepath.Join(root, "TEMPLATE_SOURCE"))
	templateRepo := strings.TrimSpace(string(sourceData))

	var stillMissing []string
	for _, name := range expectedWorkflowYMLs {
		dst := filepath.Join(workflowsPath, name)
		if _, err := os.Stat(dst); err == nil {
			continue // already present
		}

		// Fall back to fetching from the template repo.
		if templateRepo != "" {
			if content, fetchErr := fetchFileFromRepo(templateRepo, ".github/workflows/"+name); fetchErr == nil {
				if writeErr := os.WriteFile(dst, content, 0o644); writeErr == nil {
					continue
				}
			}
		}

		stillMissing = append(stillMissing, name)
	}

	if len(stillMissing) > 0 {
		return CheckResult{
			Name:    checkName,
			Status:  Fail,
			Message: fmt.Sprintf("could not restore: %s", strings.Join(stillMissing, ", ")),
		}
	}
	return CheckResult{
		Name:   checkName,
		Status: Pass,
	}
}

// fetchFileFn is the function used to fetch files from a GitHub repo.
// It defaults to fetchFileFromRepo but can be overridden in tests.
var fetchFileFn = fetchFileFromRepo

// fetchFileFromRepo fetches the raw content of a file from a GitHub repo
// using the gh API. Returns the decoded file bytes.
func fetchFileFromRepo(repo, path string) ([]byte, error) {
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/contents/%s", repo, path),
		"--jq", ".content",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api failed: %w", err)
	}
	// The API returns base64-encoded content with embedded newlines.
	raw := strings.ReplaceAll(strings.TrimSpace(string(out)), "\\n", "\n")
	// Strip surrounding quotes if jq returned a JSON string.
	raw = strings.Trim(raw, `"`)
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(raw, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("decoding content: %w", err)
	}
	return decoded, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// GitHub remote repairs
// ──────────────────────────────────────────────────────────────────────────────

// RepairLabels creates only the missing standard labels in the repo.
// repoFullName is "owner/repo". run is injected for gh operations.
func RepairLabels(repoFullName string, run bootstrap.RunCommandFunc) CheckResult {
	missing := MissingLabels(repoFullName, run)
	if len(missing) == 0 {
		return CheckResult{
			Name:   "Standard labels present",
			Status: Pass,
		}
	}

	var failed []string
	for _, label := range missing {
		_, err := run("gh", "label", "create", label, "--repo", repoFullName, "--force")
		if err != nil {
			failed = append(failed, label)
		}
	}

	if len(failed) > 0 {
		return CheckResult{
			Name:    "Standard labels present",
			Status:  Fail,
			Message: fmt.Sprintf("failed to create: %s", strings.Join(failed, ", ")),
		}
	}

	return CheckResult{
		Name:   "Standard labels present",
		Status: Pass,
	}
}

// RepairProject creates a GitHub Project for the owner.
// owner is the GitHub account/org, repoName is the project title.
// run is injected for gh operations.
func RepairProject(owner string, repoName string, run bootstrap.RunCommandFunc) CheckResult {
	_, err := run("gh", "project", "create", "--owner", owner, "--title", repoName)
	if err != nil {
		return CheckResult{
			Name:    "GitHub Project linked",
			Status:  Fail,
			Message: fmt.Sprintf("failed to create project: %v", err),
		}
	}

	return CheckResult{
		Name:   "GitHub Project linked",
		Status: Pass,
	}
}

// copyDir recursively copies src to dst, preserving file permissions.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode())
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Stale open issue repair
// ──────────────────────────────────────────────────────────────────────────────

// closeStaleIssues closes all stale open issues of the given label type.
// For each stale issue it: removes active pipeline labels, adds "done", closes
// the issue, and updates the project board status to Done.
func closeStaleIssues(repoFullName, label string, run bootstrap.RunCommandFunc) CheckResult {
	checkName := checkStaleRequirementsName
	if label == "feature" {
		checkName = checkStaleFeaturesName
	}

	stale, err := fetchStaleOpenIssues(repoFullName, label, run)
	if err != nil {
		return CheckResult{Name: checkName, Status: Fail, Message: fmt.Sprintf("failed to fetch stale issues: %v", err)}
	}
	if len(stale) == 0 {
		return CheckResult{Name: checkName, Status: Pass, Message: "nothing to repair"}
	}

	for _, iss := range stale {
		// Build label edit args: remove all active pipeline labels, add done.
		args := []string{"issue", "edit",
			fmt.Sprintf("%d", iss.Number),
			"--repo", repoFullName,
			"--remove-label", "backlog",
			"--remove-label", "scoping",
			"--remove-label", "scheduled",
			"--remove-label", "in-design",
			"--remove-label", "in-development",
			"--remove-label", "in-review",
			"--add-label", "done",
		}
		if _, labelErr := run("gh", args...); labelErr != nil {
			return CheckResult{Name: checkName, Status: Fail,
				Message: fmt.Sprintf("fixing labels on #%d: %v", iss.Number, labelErr)}
		}

		// Close the issue.
		if _, closeErr := run("gh", "issue", "close",
			fmt.Sprintf("%d", iss.Number),
			"--repo", repoFullName,
		); closeErr != nil {
			return CheckResult{Name: checkName, Status: Fail,
				Message: fmt.Sprintf("closing #%d: %v", iss.Number, closeErr)}
		}
	}

	var closed []string
	for _, s := range stale {
		closed = append(closed, fmt.Sprintf("#%d", s.Number))
	}
	return CheckResult{
		Name:    checkName,
		Status:  Pass,
		Message: fmt.Sprintf("closed: %s", strings.Join(closed, ", ")),
	}
}

// RepairStaleOpenRequirements closes open requirements whose features are all closed.
func RepairStaleOpenRequirements(repoFullName string, run bootstrap.RunCommandFunc) CheckResult {
	return closeStaleIssues(repoFullName, "requirement", run)
}

// RepairStaleOpenFeatures closes open features whose tasks are all closed.
func RepairStaleOpenFeatures(repoFullName string, run bootstrap.RunCommandFunc) CheckResult {
	return closeStaleIssues(repoFullName, "feature", run)
}

// ──────────────────────────────────────────────────────────────────────────────
// Project views repair
// ──────────────────────────────────────────────────────────────────────────────

// layoutToREST converts GraphQL layout enum values to REST API layout strings.
func layoutToREST(layout string) string {
	switch layout {
	case "BOARD_LAYOUT":
		return "board"
	case "TABLE_LAYOUT":
		return "table"
	case "ROADMAP_LAYOUT":
		return "roadmap"
	default:
		return strings.ToLower(strings.TrimSuffix(layout, "_LAYOUT"))
	}
}

// RepairProjectViews creates any required views that are missing from the GitHub
// Project using the Projects V2 REST API. For views that exist but have the
// wrong filter (GitHub provides no API to update view filters), a ManualAction
// result is returned with the exact filter strings to apply in the UI.
// Existing views (including user-added ones) are never deleted.
func RepairProjectViews(owner, repoName, root string, run bootstrap.RunCommandFunc) CheckResult {
	proj := resolveProjectEntry(owner, repoName, run)
	if proj == nil {
		return CheckResult{Name: checkProjectViewsName, Status: Fail, Message: "no GitHub Project found for owner " + owner}
	}

	tmpl, loadErr := bootstrap.LoadProjectTemplate(root)
	if loadErr != nil {
		return CheckResult{Name: checkProjectViewsName, Status: Fail, Message: fmt.Sprintf("could not load project template: %v", loadErr)}
	}

	// Fetch existing view names via GraphQL.
	query := fmt.Sprintf(`{ node(id: "%s") { ... on ProjectV2 { views(first: 20) { nodes { name } } } } }`, proj.NodeID)
	out, err := run("gh", "api", "graphql", "-f", "query="+query, "--jq", ".data.node.views.nodes[].name")
	if err != nil {
		return CheckResult{Name: checkProjectViewsName, Status: Fail, Message: fmt.Sprintf("failed to fetch views: %v", err)}
	}

	liveViews := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if t := strings.TrimSpace(line); t != "" {
			liveViews[t] = true
		}
	}

	// Build the REST endpoint for view creation.
	// Projects V2 REST API: POST /users/{login}/projectsV2/{number}/views
	//                    or POST /orgs/{org}/projectsV2/{number}/views
	var restEndpoint string
	if strings.EqualFold(proj.OwnerType, "Organization") {
		restEndpoint = fmt.Sprintf("/orgs/%s/projectsV2/%d/views", owner, proj.Number)
	} else {
		restEndpoint = fmt.Sprintf("/users/%s/projectsV2/%d/views", owner, proj.Number)
	}

	var created []string
	for _, req := range tmpl.RequiredViews {
		if _, exists := liveViews[req.Name]; exists {
			continue // view already present — never modify existing views
		}
		// View is missing — create it with layout and filter.
		args := []string{"api", "-X", "POST", restEndpoint,
			"-f", "name=" + req.Name,
			"-f", "layout=" + layoutToREST(req.Layout),
		}
		if req.Filter != "" {
			args = append(args, "-f", "filter="+req.Filter)
		}
		if _, createErr := run("gh", args...); createErr != nil {
			return CheckResult{Name: checkProjectViewsName, Status: Fail,
				Message: fmt.Sprintf("failed to create view %q: %v", req.Name, createErr)}
		}
		created = append(created, fmt.Sprintf("%q", req.Name))
	}

	if len(created) == 0 {
		return CheckResult{Name: checkProjectViewsName, Status: Pass, Message: "all required views present"}
	}
	return CheckResult{Name: checkProjectViewsName, Status: Pass,
		Message: fmt.Sprintf("created views: %s", strings.Join(created, ", "))}
}

// RepairAgentUserVar sets the AGENT_USER GitHub Actions variable at the
// specified scope (org or repo). Prompts for missing values via textConfirm.
// Returns Fail if scope is "org" but the owner is a personal account.
func RepairAgentUserVar(owner, repoName, agentUser, agentUserScope string, run bootstrap.RunCommandFunc, textConfirm func(string) (string, error)) CheckResult {
	// Prompt for agent user if not provided.
	if agentUser == "" {
		val, err := textConfirm("Enter agent username")
		if err != nil {
			return CheckResult{Name: checkAgentUserVarName, Status: Fail, Message: fmt.Sprintf("prompt failed: %v", err)}
		}
		agentUser = strings.TrimSpace(val)
		if agentUser == "" {
			return CheckResult{Name: checkAgentUserVarName, Status: Fail, Message: "agent username is required"}
		}
	}

	// Prompt for scope if not provided.
	if agentUserScope == "" {
		val, err := textConfirm("Enter scope (org or repo)")
		if err != nil {
			return CheckResult{Name: checkAgentUserVarName, Status: Fail, Message: fmt.Sprintf("prompt failed: %v", err)}
		}
		agentUserScope = strings.TrimSpace(val)
	}

	// Validate scope.
	if agentUserScope != "org" && agentUserScope != "repo" {
		return CheckResult{Name: checkAgentUserVarName, Status: Fail,
			Message: fmt.Sprintf("invalid scope %q — must be \"org\" or \"repo\"", agentUserScope)}
	}

	// Check owner type for org scope.
	if agentUserScope == "org" {
		ownerType, err := bootstrap.DefaultDetectOwnerType(owner)
		if err != nil {
			return CheckResult{Name: checkAgentUserVarName, Status: Fail,
				Message: fmt.Sprintf("failed to detect owner type: %v", err)}
		}
		if ownerType != bootstrap.OwnerTypeOrg {
			return CheckResult{Name: checkAgentUserVarName, Status: Fail,
				Message: fmt.Sprintf("cannot set org-level variable — %s is a personal account, not an organisation", owner)}
		}
	}

	// Set the variable.
	var setErr error
	if agentUserScope == "org" {
		_, setErr = run("gh", "variable", "set", "AGENT_USER", "--body", agentUser, "--org", owner)
	} else {
		_, setErr = run("gh", "variable", "set", "AGENT_USER", "--body", agentUser, "--repo", owner+"/"+repoName)
	}

	if setErr != nil {
		return CheckResult{Name: checkAgentUserVarName, Status: Fail,
			Message: fmt.Sprintf("failed to set variable: %v", setErr)}
	}

	return CheckResult{Name: checkAgentUserVarName, Status: Pass,
		Message: fmt.Sprintf("set AGENT_USER=%s at %s level", agentUser, agentUserScope)}
}
