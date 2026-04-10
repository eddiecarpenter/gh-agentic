package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
	"github.com/eddiecarpenter/gh-agentic/internal/verify"
)

// doctorConfig holds the injectable dependencies for the doctor command.
// Tests can construct this directly with fake values; the production path
// resolves real values in newDoctorCmd.
type doctorConfig struct {
	root             string
	repoFullName     string
	owner            string
	repoName         string
	ownerType        string
	run              bootstrap.RunCommandFunc
	repair           bool
	yes              bool
	agentUser        string
	agentUserScope   string
	forceCredentials bool
}

// runDoctor executes the doctor check/repair pipeline. It accepts an io.Writer
// for output, an io.Reader for interactive input, and a doctorConfig with all
// dependencies injected. This makes the function testable without requiring a
// real git repo, GitHub API, or TTY.
func runDoctor(w io.Writer, in io.Reader, cfg doctorConfig) error {
	fmt.Fprintln(w, ui.SectionHeading.Render("  Doctor — check agentic environment"))
	fmt.Fprintln(w)

	// Single scanner shared across all confirm functions — creating
	// multiple scanners on the same stdin causes buffering issues
	// where the first scanner consumes input meant for later calls.
	scanner := bufio.NewScanner(in)

	// Confirm functions for repair interactions.
	// When --yes is set, all prompts are auto-confirmed.
	textConfirm := func(prompt string) (string, error) {
		if cfg.yes {
			// Text prompts require a real value — --yes cannot supply one.
			fmt.Fprintf(w, "  %s: [skipped — provide value manually]\n", prompt)
			return "", nil
		}
		fmt.Fprintf(w, "  %s: ", prompt)
		if scanner.Scan() {
			return strings.TrimSpace(scanner.Text()), nil
		}
		return "", scanner.Err()
	}
	boolConfirm := func(prompt string) (bool, error) {
		if cfg.yes {
			fmt.Fprintf(w, "  %s [y/N]: y\n", prompt)
			return true, nil
		}
		fmt.Fprintf(w, "  %s [y/N]: ", prompt)
		if scanner.Scan() {
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			return answer == "y" || answer == "yes", nil
		}
		return false, scanner.Err()
	}

	run := cfg.run

	// Read agent user from GitHub variable (flag value takes precedence).
	agentUser := cfg.agentUser
	if agentUser == "" {
		agentUser = verify.ReadAgentUserVar(cfg.owner, cfg.repoName, run)
	}

	// All checks in pipeline order.
	checks := []verify.CheckFunc{
		func() verify.CheckResult { return verify.CheckCLAUDEMD(cfg.root) },
		func() verify.CheckResult { return verify.CheckAGENTSLocalMD(cfg.root) },
		func() verify.CheckResult { return verify.CheckSkillsDir(cfg.root) },
		func() verify.CheckResult { return verify.CheckTEMPLATESOURCE(cfg.root) },
		func() verify.CheckResult { return verify.CheckTEMPLATEVERSION(cfg.root) },
		func() verify.CheckResult { return verify.CheckREPOSMD(cfg.root) },
		func() verify.CheckResult { return verify.CheckREADMEMD(cfg.root) },
		func() verify.CheckResult { return verify.CheckOldLayout(cfg.root) },
		func() verify.CheckResult { return verify.CheckAIDir(cfg.root, run) },
		func() verify.CheckResult { return verify.CheckAISkills(cfg.root, run) },
		func() verify.CheckResult { return verify.CheckGooseRecipes(cfg.root) },
		func() verify.CheckResult { return verify.CheckWorkflows(cfg.root, cfg.ownerType) },
		func() verify.CheckResult { return verify.CheckLabels(cfg.repoFullName, run) },
		func() verify.CheckResult { return verify.CheckProject(cfg.owner, run) },
		func() verify.CheckResult {
			return verify.CheckAgenticProjectID(cfg.repoFullName, cfg.owner, cfg.ownerType, run)
		},
		func() verify.CheckResult { return verify.CheckProjectStatus(cfg.owner, cfg.repoName, cfg.root, run) },
		func() verify.CheckResult { return verify.CheckProjectViews(cfg.owner, cfg.repoName, cfg.root, run) },
		func() verify.CheckResult {
			return verify.CheckProjectItemStatuses(cfg.owner, cfg.repoName, run)
		},
		func() verify.CheckResult { return verify.CheckAgentUserVar(cfg.owner, cfg.repoName, cfg.ownerType, run) },
		func() verify.CheckResult { return verify.CheckRunnerLabelVar(cfg.owner, cfg.repoName, cfg.ownerType, run) },
		func() verify.CheckResult { return verify.CheckGooseProviderVar(cfg.owner, cfg.repoName, cfg.ownerType, run) },
		func() verify.CheckResult { return verify.CheckGooseModelVar(cfg.owner, cfg.repoName, cfg.ownerType, run) },
		func() verify.CheckResult { return verify.CheckGooseAgentPATSecret(cfg.owner, cfg.repoName, cfg.ownerType, run) },
		func() verify.CheckResult { return verify.CheckClaudeCredentialsSecret(cfg.owner, cfg.repoName, cfg.ownerType, run) },
		func() verify.CheckResult {
			return verify.CheckProjectCollaborator(cfg.owner, cfg.repoName, agentUser, cfg.ownerType, run)
		},
		func() verify.CheckResult { return verify.CheckStaleOpenRequirements(cfg.repoFullName, run) },
		func() verify.CheckResult { return verify.CheckStaleOpenFeatures(cfg.repoFullName, run) },
	}

	// Repair function — only active when --repair flag is set.
	var repairFn verify.RepairFunc
	if cfg.repair {
		repairFn = func(result verify.CheckResult) *verify.CheckResult {
			var r verify.CheckResult
			switch result.Name {
			case "CLAUDE.md exists":
				r = verify.RepairCLAUDEMD(cfg.root)
			case "AGENTS.local.md exists":
				r = verify.RepairAGENTSLocalMD(cfg.root)
			case "skills/ directory exists":
				r = verify.RepairSkillsDir(cfg.root, run)
			case "TEMPLATE_SOURCE exists":
				r = verify.RepairTEMPLATESOURCE(cfg.root, textConfirm)
			case "TEMPLATE_VERSION exists":
				r = verify.RepairTEMPLATEVERSION(cfg.root, run)
			case "REPOS.md exists":
				r = verify.RepairREPOSMD(cfg.root)
			case "README.md exists":
				r = verify.RepairREADMEMD(cfg.root)
			case ".ai/ exists and is unmodified":
				r = verify.RepairAIDirWithWriter(w, cfg.root, run, boolConfirm)
			case ".ai/skills/*.md unmodified":
				r = verify.RepairAISkills(cfg.root, boolConfirm, nil)
			case ".goose/recipes/ exists and complete":
				r = verify.RepairGooseRecipes(cfg.root, nil)
			case ".github/workflows/ exists and complete":
				r = verify.RepairWorkflows(cfg.root, cfg.ownerType, run, nil)
			case "Standard labels present":
				r = verify.RepairLabels(cfg.repoFullName, run)
			case "GitHub Project linked":
				r = verify.RepairProject(cfg.owner, cfg.repoName, run)
			case "AGENTIC_PROJECT_ID is configured":
				r = verify.RepairAgenticProjectID(cfg.repoFullName, cfg.owner, cfg.repoName, cfg.ownerType, run)
			case "GitHub Project status options are standard":
				r = verify.RepairProjectStatus(cfg.owner, cfg.repoName, cfg.root, run)
			case "GitHub Project has required views":
				r = verify.RepairProjectViews(cfg.owner, cfg.repoName, cfg.root, run)
			case "Project items have status assigned":
				r = verify.RepairProjectItemStatuses(cfg.owner, cfg.repoName, run)
			case "AGENT_USER variable configured":
				r = verify.RepairAgentUserVar(cfg.owner, cfg.repoName, cfg.agentUser, cfg.agentUserScope, run, textConfirm)
			case "RUNNER_LABEL variable configured":
				r = verify.RepairRunnerLabelVar(cfg.owner, cfg.repoName, cfg.ownerType, run)
			case "GOOSE_PROVIDER variable configured":
				r = verify.RepairGooseProviderVar(cfg.owner, cfg.repoName, cfg.ownerType, run)
			case "GOOSE_MODEL variable configured":
				r = verify.RepairGooseModelVar(cfg.owner, cfg.repoName, cfg.ownerType, run)
			case "GOOSE_AGENT_PAT secret configured":
				r = verify.RepairGooseAgentPATSecret(cfg.owner, cfg.repoName, cfg.ownerType)
			case "CLAUDE_CREDENTIALS_JSON secret configured":
				r = verify.RepairClaudeCredentialsSecret(cfg.owner, cfg.repoName, cfg.ownerType, run)
			case "Agent user is a project collaborator":
				r = verify.RepairProjectCollaborator(cfg.owner, cfg.repoName, agentUser, cfg.ownerType, run)
			case "No stale open requirements":
				r = verify.RepairStaleOpenRequirements(cfg.repoFullName, run)
			case "No stale open features":
				r = verify.RepairStaleOpenFeatures(cfg.repoFullName, run)
			default:
				return nil
			}
			return &r
		}
	}

	if err := verify.RunVerify(w, checks, repairFn); err != nil {
		fmt.Fprintln(w, "  Run 'gh agentic doctor --repair' to attempt automatic fixes.")
		fmt.Fprintln(w, "  For AGENT_USER repair, add: --agent-user <username> --agent-user-scope org|repo")
		if !cfg.forceCredentials {
			return ErrSilent
		}
	}

	// --force-credentials: unconditionally re-upload Claude credentials.
	if cfg.forceCredentials {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.SectionHeading.Render("  Force credentials refresh"))
		result := verify.RepairClaudeCredentialsSecret(cfg.owner, cfg.repoName, cfg.ownerType, run)
		switch result.Status {
		case verify.Pass:
			fmt.Fprintln(w, "  "+ui.RenderOK(result.Name))
		case verify.Warning:
			fmt.Fprintln(w, "  "+ui.RenderWarning(result.Name+": "+result.Message))
		case verify.Fail:
			fmt.Fprintln(w, "  "+ui.RenderError(result.Name+": "+result.Message))
		case verify.ManualAction:
			fmt.Fprintln(w, "  "+ui.RenderInfo(result.Name+": "+result.Message))
		}
	}

	return nil
}

// newDoctorCmd constructs the `gh agentic doctor` subcommand.
func newDoctorCmd() *cobra.Command {
	var repair bool
	var yes bool
	var agentUser string
	var agentUserScope string
	var forceCredentials bool

	cmd := &cobra.Command{
		Use:          "doctor",
		Short:        "Check an agentic environment for correctness",
		SilenceUsage: true,
		Long: "Checks an existing agentic environment for correctness and repairs\n" +
			"what it can automatically. Each check shows ✔ pass, ⚠ warning, or ✖ fail.\n" +
			"Pass --repair to attempt automatic fixes for all warnings and failures.\n" +
			"Pass --yes to automatically confirm all repair prompts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// Resolve repo root. If TEMPLATE_SOURCE is not found (repo predates
			// the extension), fall back to cwd — TEMPLATE_SOURCE missing will
			// surface as a failed check that --repair can fix.
			root, err := findRepoRoot()
			if err != nil {
				root, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("resolving working directory: %w", err)
				}
				fmt.Fprintln(w, "  "+ui.RenderWarning("TEMPLATE_SOURCE not found — using current directory as root"))
				fmt.Fprintln(w)
			}

			// Resolve repo full name (owner/repo) and owner via go-gh,
			// which reads from git remote without shelling out to gh.
			currentRepo, err := repository.Current()
			if err != nil {
				return fmt.Errorf("resolving repo name: %w", err)
			}

			// Detect owner type to know which workflows apply.
			// Default to org (stricter) on error so personal-only workflows
			// are not silently skipped on genuine org repos.
			ownerType, err := bootstrap.DefaultDetectOwnerType(currentRepo.Owner)
			if err != nil {
				ownerType = bootstrap.OwnerTypeOrg
			}

			return runDoctor(w, cmd.InOrStdin(), doctorConfig{
				root:             root,
				repoFullName:     currentRepo.Owner + "/" + currentRepo.Name,
				owner:            currentRepo.Owner,
				repoName:         currentRepo.Name,
				ownerType:        ownerType,
				run:              bootstrap.DefaultRunCommand,
				repair:           repair,
				yes:              yes,
				agentUser:        agentUser,
				agentUserScope:   agentUserScope,
				forceCredentials: forceCredentials,
			})
		},
	}

	cmd.Flags().BoolVar(&repair, "repair", false, "attempt automatic repair of all warnings and failures")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "automatically confirm all repair prompts")
	cmd.Flags().StringVar(&agentUser, "agent-user", "", "agent username for repair (skips prompt)")
	cmd.Flags().StringVar(&agentUserScope, "agent-user-scope", "", "variable scope: org or repo (skips prompt)")
	cmd.Flags().BoolVar(&forceCredentials, "force-credentials", false, "unconditionally re-upload Claude credentials after checks complete")
	return cmd
}
