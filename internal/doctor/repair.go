package doctor

import (
	"fmt"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/scope"
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

// ApplyPendingPrompt sets a single GitHub variable or secret using the
// supplied run function. The CLI layer calls this after collecting values
// from the user via huh. Empty values are treated as a skip.
//
// Scope routing follows scope.ScopeFor: under federated topology the
// shared names are written at `--org <owner>`, while per-repo identity
// names and all names under single topology stay at `--repo
// <owner/repo>`. The federated-cp/federated-domain distinction is carried
// on the PendingPrompt (which RepairPipeline fills from CheckDeps).
func ApplyPendingPrompt(run auth.RunCommandFunc, repoFullName string, p PendingPrompt, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("skipped (no value supplied)")
	}
	flag, target := scope.ScopeFor(p.Name, p.Topology, p.Owner, repoFullName)
	switch p.Kind {
	case "variable":
		_, err := run("gh", "variable", "set", p.Name, flag, target, "--body", value)
		return err
	case "secret":
		_, err := run("gh", "secret", "set", p.Name, flag, target, "--body", value)
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

// shadowConfirmTitle is the exact sentence shown to the human before the
// shadow-vars batch delete. Kept as a constant so tests can assert on it.
const shadowConfirmTitle = "Remove these N shadow values? They will fall back to the org-level values."

// shadowConfirmPlaceholder is the literal placeholder in shadowConfirmTitle
// that RepairShadowValues substitutes with the actual count.
const shadowConfirmPlaceholder = "N"

// RepairShadowValues drives the shadow-vars batch repair. It presents a
// single ConfirmFunc invocation (never one-per-item), and — on Yes —
// iterates the items running each delete command via run. A delete failure
// does not abort the batch; the remaining items are still attempted and
// surfaced in the returned result.
//
// On No, no delete is issued and every item surfaces as Unrepaired with
// its original delete command so the human has an exact copy-paste remedy.
//
// An empty items slice is a no-op — no prompt, no work.
//
// Tests inject fake run and confirm implementations; production wires huh
// and auth.DefaultRunCommand.
func RepairShadowValues(items []ShadowValue, run func(string, ...string) (string, error), confirm ConfirmFunc) RepairResult {
	res := RepairResult{}
	if len(items) == 0 {
		return res
	}

	// Build the bulleted body shown beneath the title so the human can see
	// exactly which names are about to be deleted.
	var body strings.Builder
	for _, it := range items {
		fmt.Fprintf(&body, "  • %s (%s)\n", it.Name, it.Kind)
	}

	title := strings.Replace(shadowConfirmTitle, shadowConfirmPlaceholder,
		fmt.Sprintf("%d", len(items)), 1)

	ok, err := confirm(title, body.String())
	if err != nil {
		// Surface the cancellation and leave the items unrepaired with
		// their manual commands.
		res.Lines = append(res.Lines,
			fmt.Sprintf("  %s  Shadow-vars confirm cancelled: %v",
				ui.StatusWarning.Render("⚠"), err))
		for _, it := range items {
			res.Lines = append(res.Lines, shadowManualLine(it))
			res.Unrepaired++
		}
		return res
	}

	if !ok {
		// Explicit No — keep the items as unrepaired with the manual
		// delete command the human already saw in the check output.
		for _, it := range items {
			res.Lines = append(res.Lines, shadowManualLine(it))
			res.Unrepaired++
		}
		return res
	}

	// Yes — iterate. Never abort on a single failure.
	for _, it := range items {
		if err := runShadowDelete(run, it); err != nil {
			res.Lines = append(res.Lines,
				fmt.Sprintf("  %s  %s %q — %v",
					ui.StatusDanger.Render("✗"), it.Kind, it.Name, err))
			res.Unrepaired++
			continue
		}
		res.Lines = append(res.Lines,
			fmt.Sprintf("  %s  %s %q removed from repo scope",
				ui.StatusOK.Render("✓"), it.Kind, it.Name))
		res.Repaired++
	}
	return res
}

// runShadowDelete executes the gh delete for a single shadow item using
// the injected runner. The delete command layout mirrors the string the
// check attached as Remediation (so the human sees the same thing).
func runShadowDelete(run func(string, ...string) (string, error), it ShadowValue) error {
	switch it.Kind {
	case "variable":
		_, err := run("gh", "variable", "delete", it.Name, "--repo", extractRepoFromDelete(it.DeleteCommand))
		return err
	case "secret":
		_, err := run("gh", "secret", "delete", it.Name, "--repo", extractRepoFromDelete(it.DeleteCommand))
		return err
	default:
		return fmt.Errorf("unknown shadow kind %q", it.Kind)
	}
}

// extractRepoFromDelete parses the owner/repo slug from a shadow item's
// delete command. The check produces the command via
// `fmt.Sprintf("gh %s delete --repo %s %s", kind, repoFullName, name)`, so
// the slug sits immediately after `--repo`.
func extractRepoFromDelete(cmd string) string {
	fields := strings.Fields(cmd)
	for i, f := range fields {
		if f == "--repo" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// shadowManualLine renders an unrepaired shadow as a single output line
// that surfaces the original delete command.
func shadowManualLine(it ShadowValue) string {
	return fmt.Sprintf("  %s  %s %q — not removed: %s",
		ui.StatusDanger.Render("✗"), it.Kind, it.Name, it.DeleteCommand)
}

// RepairResult holds the outcome of a pipeline-side repair run.
type RepairResult struct {
	Lines          []string
	Repaired       int
	Unrepaired     int
	PendingPrompts []PendingPrompt
	// ShadowBatch carries the list of shadow values collected during the
	// RepairPipeline spinner phase. The CLI layer processes it after the
	// spinner closes via RepairShadowValues, which shows the single batch
	// confirmation prompt and iterates the deletes. Nil when there are no
	// shadow values to repair.
	ShadowBatch *PendingShadowBatch
}

// PendingShadowBatch collects shadow values detected by the shadow-vars
// check so the CLI layer can drive a single batch confirmation after the
// non-interactive spinner phase finishes.
type PendingShadowBatch struct {
	Items []ShadowValue
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
// Topology and Owner are carried so ApplyPendingPrompt can route the eventual
// write through scope.ScopeFor without re-deriving them. They are populated
// by RepairPipeline from CheckDeps.
type PendingPrompt struct {
	Name        string // GitHub variable or secret name
	Kind        string // "variable" or "secret"
	Description string // shown in the form
	Default     string // suggested default value (may be empty)
	Topology    string // topology string accepted by scope.ScopeFor
	Owner       string // GitHub owner login (--org target under federated)
}

// pendingDescriptions maps known variable/secret names to short descriptions
// and sensible defaults. Names absent from the map are still prompted, with
// the bare name shown as the title.
var pendingDescriptions = map[string]struct {
	Description string
	Default     string
}{
	"AGENT_USER":              {"GitHub username the agent commits as (e.g. goose-bot)", ""},
	"RUNNER_LABEL":            {"GitHub Actions runner label", ""}, // resolved via select in cli layer
	"AGENT_PROVIDER":          {"The LLM provider the agent will use", "claude-code"},
	"AGENT_MODEL":             {"Agent model override", "default"},
	"AGENTIC_APP_CLIENT_ID":   {"GitHub App client ID for the agentic identity", ""},
	"AGENTIC_APP_PRIVATE_KEY": {"GitHub App private key (stored as a secret) used to mint installation tokens", ""},
	"PROJECT_PAT":             {"Personal Access Token for Projects v2 mutations (stored as a secret)", ""},
}

// workflowRepairNames is the set of CheckResult names produced by checkWorkflows
// that map to a workflow-tag mismatch (auto-fixable via UpdateWorkflowVersions).
var workflowRepairNames = map[string]bool{
	"agentic-pipeline.yml": true,
	"release.yml":          true,
}

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

	for _, g := range report.Groups {
		for _, r := range g.Results {
			if r.Status != Fail {
				continue
			}

			switch {
			case r.Name == "gitignore":
				if setLabel != nil {
					setLabel("Repairing: .gitignore...")
				}
				if err := mount.EnsureGitignore(deps.Root); err != nil {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  Could not add .ai/ to .gitignore: %v",
							ui.StatusDanger.Render("✗"), err))
					result.Unrepaired++
				} else {
					result.Lines = append(result.Lines,
						fmt.Sprintf("  %s  .ai/ added to .gitignore",
							ui.StatusOK.Render("✓")))
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
							ui.StatusOK.Render("✓"), version, rewrites))
					result.Repaired++
				}

			case r.Name == "shadow-vars":
				// The shadow-vars summary result carries the structured
				// []ShadowValue on its Data field. Collect it for the CLI
				// layer to confirm + delete after the spinner phase.
				// Per-item child results (shadow-vars:<kind>:<name>) are
				// ignored here because the batch already includes them —
				// processing each individually would double-count.
				items, ok := r.Data.([]ShadowValue)
				if !ok || len(items) == 0 {
					continue
				}
				if result.ShadowBatch == nil {
					result.ShadowBatch = &PendingShadowBatch{}
				}
				result.ShadowBatch.Items = append(result.ShadowBatch.Items, items...)
				continue

			case strings.HasPrefix(r.Name, "shadow-vars:"):
				// Per-item children — already accounted for via the
				// summary's Data slice above. Skip so we don't emit a
				// redundant "no repair" line.
				continue

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
						Topology:    deps.Topology,
						Owner:       deps.Owner,
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
