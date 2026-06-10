package doctor

import (
	"fmt"
	"io"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// pendingKind classifies a failed CheckResult as a variable, secret, or
// neither, by inspecting its remediation string. Returns ("variable"|"secret",
// true) on a match, and ("", false) when the result isn't a known
// var/secret-set hint (so the caller falls back to surfacing it as
// non-auto-repairable).
func pendingKind(r CheckResult) (string, bool) {
	switch {
	case strings.HasPrefix(r.Remediation, "gh variable set "):
		return "variable", true
	case strings.HasPrefix(r.Remediation, "gh secret set "):
		return "secret", true
	}
	return "", false
}

// isLabelCreateRemediation returns true when the CheckResult's Remediation
// string is a `gh label create …` command — meaning it was produced by
// checkLabels for a missing pipeline label. RepairPipeline uses this to
// detect which Fail results it can auto-fix by running the command directly.
func isLabelCreateRemediation(r CheckResult) bool {
	return strings.HasPrefix(r.Remediation, "gh label create ")
}

// runLabelCreate executes a `gh label create` remediation command via the
// injected run function. The command string is produced by checkLabels and
// has the shape:
//
//	gh label create "<name>" --repo <owner/repo> --color <hex> --description "<desc>"
//
// Rather than re-parsing the shell-quoted command, we reconstruct the call
// from requiredPipelineLabels by matching on the name embedded in the
// remediation. This is safe because the remediation is machine-produced, not
// user-supplied.
func runLabelCreate(run auth.RunCommandFunc, remediation string) error {
	// Find the matching label definition so we can use the structured fields
	// directly instead of re-parsing the shell-quoted remediation string.
	for _, lbl := range requiredPipelineLabels {
		needle := fmt.Sprintf("gh label create %q", lbl.Name)
		if strings.HasPrefix(remediation, needle) {
			// Extract the repo from the remediation string. It appears
			// immediately after "--repo " in the command.
			repo := extractRepoFromLabelCreate(remediation)
			if repo == "" {
				return fmt.Errorf("cannot parse repo from label create remediation: %q", remediation)
			}
			_, err := run("gh", "label", "create", lbl.Name,
				"--repo", repo,
				"--color", lbl.Color,
				"--description", lbl.Description,
			)
			return err
		}
	}
	return fmt.Errorf("no matching label definition for remediation: %q", remediation)
}

// extractRepoFromLabelCreate parses the owner/repo slug from a label-create
// remediation string. The check produces the command via fmt.Sprintf so the
// slug always appears as the token immediately following "--repo".
func extractRepoFromLabelCreate(cmd string) string {
	fields := strings.Fields(cmd)
	for i, f := range fields {
		if f == "--repo" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// ApplyPendingPrompt sets a single GitHub variable or secret using the
// supplied run function. The CLI layer calls this after collecting values
// from the user via huh. Empty values are treated as a skip.
//
// Feature #824: all variables and secrets are written at --repo scope.
// The org-scope routing that existed under the old federated topology
// has been removed.
func ApplyPendingPrompt(run auth.RunCommandFunc, repoFullName string, p PendingPrompt, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("skipped (no value supplied)")
	}
	switch p.Kind {
	case "variable":
		_, err := run("gh", "variable", "set", p.Name, "--repo", repoFullName, "--body", value)
		return err
	case "secret":
		_, err := run("gh", "secret", "set", p.Name, "--repo", repoFullName, "--body", value)
		return err
	default:
		return fmt.Errorf("unknown pending kind %q", p.Kind)
	}
}

// FormatPromptApplied renders a single line summarising the outcome of an
// ApplyPendingPrompt call. Repair callers append this to the output.
func FormatPromptApplied(p PendingPrompt, err error) string {
	if err != nil {
		return fmt.Sprintf("  %s  %s — %v",
			ui.StatusDanger.Render("✗"), p.Name, err)
	}
	return fmt.Sprintf("  %s  %s set",
		ui.StatusOK.Render("✓"), p.Name)
}

// RepairResult holds the outcome of a pipeline-side repair run.
type RepairResult struct {
	Lines          []string
	Repaired       int
	Unrepaired     int
	PendingPrompts []PendingPrompt
}

// ConfirmFunc prompts the user for a yes/no confirmation. Title is the
// primary prompt; message holds any supplementary body shown below the
// title. Returns true if the user confirmed, false otherwise. Implementations
// may return an error for IO / cancellation failures.
type ConfirmFunc func(title, message string) (bool, error)

// PendingPrompt describes a variable or secret that needs a human-supplied
// value. RepairPipeline collects these so the CLI layer can prompt for values
// after the spinner phase finishes (the spinner can't share the terminal with
// an interactive form).
//
// Feature #824: scope routing is always --repo; Topology and Owner fields
// have been removed.
type PendingPrompt struct {
	Name        string // GitHub variable or secret name
	Kind        string // "variable" or "secret"
	Description string // shown in the form
	Default     string // suggested default value (may be empty)
}

// pendingDescriptions maps known variable/secret names to short descriptions
// and sensible defaults. Names absent from the map are still prompted, with
// the bare name shown as the title.
var pendingDescriptions = map[string]struct {
	Description string
	Default     string
}{
	"RUNNER_LABEL":   {"GitHub Actions runner label", ""}, // resolved via select in cli layer
	"AGENT_PROVIDER": {"The LLM provider the agent will use", "claude-code"},
	"AGENT_MODEL":    {"Agent model override", "default"},
	"PROJECT_PAT":    {"Personal Access Token for Projects v2 mutations (stored as a secret)", ""},
	"PIPELINE_PAT":   {"Fine-grained PAT for pipeline trigger operations — Issues: write, Pull requests: write, Secrets: write (stored as a secret)", ""},
}

// workflowRepairNames is the set of CheckResult names produced by checkWorkflows
// that are auto-fixable by repair — either a missing file (scaffold) or a
// version-tag mismatch (UpdateWorkflowVersions).
var workflowRepairNames = map[string]bool{
	"agentic-pipeline.yml": true,
	"release.yml":          true,
}

// workflowMissingRemediation is the exact Remediation string that
// checkWorkflows writes when a workflow file does not exist. RepairPipeline
// uses it to distinguish "file absent" from "version tag mismatch" so the
// two cases can be routed to their respective repair paths.
const workflowMissingRemediation = "Run 'gh agentic repair'"

// RepairPipeline runs all pipeline checks and attempts to auto-repair the
// failures that are mechanically fixable. Failures that require human input
// (missing variables, missing secrets) are surfaced with their remediation
// hint but counted as Unrepaired.
func RepairPipeline(deps CheckDeps, setLabel func(string)) RepairResult {
	result := RepairResult{}

	if setLabel != nil {
		setLabel("Running pipeline checks...")
	}
	report := RunAllChecksWithProgress(deps, setLabel)

	workflowRepairAttempted := false
	workflowScaffoldAttempted := false

	for _, g := range report.Groups {
		for _, r := range g.Results {
			// Warning results are normally informational and ignored by repair.
			// Exception: missing variables that have a known default still
			// expose a remediation hint — let the human accept the default or
			// supply a custom value via the interactive prompt path.
			if r.Status == Warning {
				if _, ok := pendingKind(r); !ok {
					continue
				}
			} else if r.Status != Fail {
				continue
			}

			switch {
			case r.Name == "gitignore":
				if setLabel != nil {
					setLabel("Repairing: .gitignore...")
				}
				if err := mount.RemoveAIFromGitignore(deps.Root); err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not remove .agents/ from .gitignore: %v",
							ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
				} else {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  .agents/ removed from .gitignore (legacy shallow-clone state)",
							ui.StatusOK.Render("✓")))
					result.Repaired++
				}

			case isLabelCreateRemediation(r):
				// Missing pipeline label — create it automatically via gh CLI.
				// Label creation is non-interactive and idempotent (gh label create
				// returns an error if the label already exists, which RepairPipeline
				// treats as a benign no-op: the next check run will confirm Pass).
				if setLabel != nil {
					setLabel("Repairing: pipeline labels...")
				}
				if err := runLabelCreate(deps.Run, r.Remediation); err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not create label %s: %v",
							ui.StatusDanger.Render("✗"), r.Name, err))
					result.Unrepaired++
				} else {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Label %s created",
							ui.StatusOK.Render("✓"), r.Name))
					result.Repaired++
				}

			case workflowRepairNames[r.Name] && r.Remediation == workflowMissingRemediation:
				// Workflow file is absent — scaffold it (and any other missing
				// workflow) in a single pass. One call to GenerateWorkflows
				// writes both agentic-pipeline.yml and release.yml, so dedupe.
				if workflowScaffoldAttempted {
					continue
				}
				workflowScaffoldAttempted = true
				if setLabel != nil {
					setLabel("Repairing: scaffolding missing workflow files...")
				}
				version, verr := mount.ReadAIVersionFromGit(deps.Root)
				if verr != nil || version == "" {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Cannot scaffold workflows: framework version unknown — run 'gh agentic init' first",
							ui.StatusDanger.Render("✗")))
					result.Unrepaired++
					continue
				}
				if err := mount.GenerateWorkflows(io.Discard, deps.Root, version); err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not scaffold workflow files: %v",
							ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
				} else {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Wrapper workflows created at .github/workflows/ (version %s)",
							ui.StatusOK.Render("✓"), mount.TrimVPrefix(version)))
					result.Repaired++
				}

			case workflowRepairNames[r.Name]:
				// Multiple workflow files may all be out of date; one rewrite
				// pass fixes them all, so dedupe.
				if workflowRepairAttempted {
					continue
				}
				workflowRepairAttempted = true
				if setLabel != nil {
					setLabel("Repairing: workflow version tags...")
				}
				version, verr := mount.ReadAIVersionFromGit(deps.Root)
				if verr != nil || version == "" {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not repair workflow versions: framework version unknown",
							ui.StatusDanger.Render("✗")))
					result.Unrepaired++
					continue
				}
				rewrites, err := mount.UpdateWorkflowVersionsCount(deps.Root, version)
				switch {
				case err != nil:
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not update workflow versions: %v",
							ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
				case rewrites == 0:
					// The check failed but the rewriter found nothing to change —
					// the workflows don't reference gh-agentic reusable workflows.
					// Surface honestly instead of claiming a phantom repair.
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  %s",
							ui.StatusDanger.Render("✗"), r.Message))
					if r.Remediation != "" {
						result.Lines = append(result.Lines,
							"     "+ui.Muted.Render("→ "+r.Remediation))
					}
					result.Unrepaired++
				default:
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Workflow version tags updated to %s (%d file(s))",
							ui.StatusOK.Render("✓"), mount.TrimVPrefix(version), rewrites))
					result.Repaired++
				}

			case r.Name == "status-options":
				if setLabel != nil {
					setLabel("Repairing: project status options...")
				}
				if deps.FetchProjectFields == nil || deps.UpdateStatusFieldOptions == nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Cannot repair status options — no GraphQL client available",
							ui.StatusDanger.Render("✗")))
					result.Unrepaired++
					continue
				}
				tpl, err := project.ReadProjectTemplate()
				if err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Cannot repair status options — could not read template: %v",
							ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
					continue
				}
				fields, err := deps.FetchProjectFields(deps.ProjectID)
				if err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Cannot repair status options — could not fetch fields: %v",
							ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
					continue
				}
				var statusFieldID string
				for _, f := range fields {
					if f.Name == "Status" {
						statusFieldID = f.ID
						break
					}
				}
				if statusFieldID == "" {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Cannot repair status options — Status field not found on project",
							ui.StatusDanger.Render("✗")))
					result.Unrepaired++
					continue
				}
				if err := deps.UpdateStatusFieldOptions(statusFieldID, tpl.StatusField.Options); err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not update project status options: %v",
							ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
				} else {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Project status options synced (%d options)",
							ui.StatusOK.Render("✓"), len(tpl.StatusField.Options)))
					result.Repaired++
				}

			case strings.HasPrefix(r.Name, "federation-sync:not-linked:"):
				// Manifest repo not linked to the federation project — link it
				// automatically using the repo node ID stored in r.Data at check time.
				if setLabel != nil {
					setLabel("Repairing: federation project sync...")
				}
				repoID, ok := r.Data.(string)
				if deps.LinkRepoToProject == nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Cannot repair federation link for %s — no GraphQL client available",
							ui.StatusDanger.Render("✗"), r.Name))
					result.Unrepaired++
					continue
				}
				if deps.ProjectID == "" {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Cannot repair federation link for %s — AGENTIC_PROJECT_ID not configured",
							ui.StatusDanger.Render("✗"), r.Name))
					result.Unrepaired++
					continue
				}
				if !ok || repoID == "" {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Cannot repair federation link for %s — repo node ID missing from check result",
							ui.StatusDanger.Render("✗"), r.Name))
					result.Unrepaired++
					continue
				}
				if err := deps.LinkRepoToProject(deps.ProjectID, repoID); err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not link repo to project (%s): %v",
							ui.StatusDanger.Render("✗"), r.Name, err))
					result.Unrepaired++
				} else {
					// Extract the owner/repo slug from the check-result name for the
					// success message: "federation-sync:not-linked:<owner/repo>"
					slug := strings.TrimPrefix(r.Name, "federation-sync:not-linked:")
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Linked %s to the federation project",
							ui.StatusOK.Render("✓"), slug))
					result.Repaired++
				}

			default:
				// Variable / secret failures: defer to interactive prompts so
				// the CLI layer can collect values after the spinner phase.
				if kind, ok := pendingKind(r); ok {
					meta := pendingDescriptions[r.Name]
					result.PendingPrompts = append(result.PendingPrompts, PendingPrompt{
						Name:        r.Name,
						Kind:        kind,
						Description: meta.Description,
						Default:     meta.Default,
					})
					continue
				}

				// Not auto-repairable — surface so the user sees what is left.
				line := fmt.Sprintf("  %s  %s",
					ui.StatusDanger.Render("✗"), r.Message)
				if r.Remediation != "" {
					line += " " + ui.Muted.Render("→ "+r.Remediation)
				}
				result.Lines = append(result.Lines, line)
				result.Unrepaired++
			}
		}
	}

	return result
}
