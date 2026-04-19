package doctorv2

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

// RepairResult holds the outcome of a pipeline-side repair run.
type RepairResult struct {
	Lines          []string
	Repaired       int
	Unrepaired     int
	PendingPrompts []PendingPrompt
}

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
	"AGENT_USER":      {"GitHub username the agent commits as (e.g. goose-bot)", ""},
	"RUNNER_LABEL":    {"GitHub Actions runner label", ""}, // resolved via select in cli layer
	"GOOSE_PROVIDER":  {"The LLM provider the agent will use", "claude-code"},
	"GOOSE_MODEL":     {"Goose model override", "default"},
	"GOOSE_AGENT_PAT": {"Personal Access Token for the agent (stored as a secret)", ""},
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
