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
	root         string
	repoFullName string
	owner        string
	repoName     string
	run          bootstrap.RunCommandFunc
	repair       bool
	yes          bool
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

	// Read agent user from AGENT_USER file (empty string if absent).
	agentUser, _ := bootstrap.ReadAgentUser(cfg.root)

	// All checks in pipeline order.
	checks := []verify.CheckFunc{
		func() verify.CheckResult { return verify.CheckCLAUDEMD(cfg.root) },
		func() verify.CheckResult { return verify.CheckAGENTSLocalMD(cfg.root) },
		func() verify.CheckResult { return verify.CheckSkillsDir(cfg.root) },
		func() verify.CheckResult { return verify.CheckTEMPLATESOURCE(cfg.root) },
		func() verify.CheckResult { return verify.CheckTEMPLATEVERSION(cfg.root) },
		func() verify.CheckResult { return verify.CheckREPOSMD(cfg.root) },
		func() verify.CheckResult { return verify.CheckREADMEMD(cfg.root) },
		func() verify.CheckResult { return verify.CheckBaseDir(cfg.root, run) },
		func() verify.CheckResult { return verify.CheckBaseRecipes(cfg.root, run) },
		func() verify.CheckResult { return verify.CheckGooseRecipes(cfg.root) },
		func() verify.CheckResult { return verify.CheckWorkflows(cfg.root) },
		func() verify.CheckResult { return verify.CheckGhNotify(cfg.root, run) },
		func() verify.CheckResult { return verify.CheckLabels(cfg.repoFullName, run) },
		func() verify.CheckResult { return verify.CheckProject(cfg.owner, run) },
		func() verify.CheckResult { return verify.CheckProjectStatus(cfg.owner, cfg.root, run) },
		func() verify.CheckResult { return verify.CheckProjectItemStatuses(cfg.owner, cfg.root, run) },
		func() verify.CheckResult { return verify.CheckProjectCollaborator(cfg.owner, agentUser, run) },
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
			case "base/ exists and is unmodified":
				r = verify.RepairBaseDirWithWriter(w, cfg.root, run, boolConfirm)
			case "base/skills/*.md unmodified":
				r = verify.RepairBaseRecipes(cfg.root, run, boolConfirm)
			case ".goose/recipes/ exists and complete":
				r = verify.RepairGooseRecipes(cfg.root)
			case ".github/workflows/ exists and complete":
				r = verify.RepairWorkflows(cfg.root, run)
			case "gh-notify LaunchAgent installed":
				r = verify.RepairGhNotify(cfg.root, run)
			case "Standard labels present":
				r = verify.RepairLabels(cfg.repoFullName, run)
			case "GitHub Project linked":
				r = verify.RepairProject(cfg.owner, cfg.repoName, run)
			case "GitHub Project status options are standard":
				r = verify.RepairProjectStatus(cfg.owner, cfg.root, run)
			case "Agent user is a project collaborator":
				r = verify.RepairProjectCollaborator(cfg.owner, agentUser, run)
			default:
				return nil
			}
			return &r
		}
	}

	if err := verify.RunVerify(w, checks, repairFn); err != nil {
		fmt.Fprintln(w, "  Run 'gh agentic doctor --repair --yes' to attempt automatic fixes.")
		return ErrSilent
	}
	return nil
}

// newDoctorCmd constructs the `gh agentic doctor` subcommand.
func newDoctorCmd() *cobra.Command {
	var repair bool
	var yes bool

	cmd := &cobra.Command{
		Use:          "doctor",
		Short:        "Check an agentic environment for correctness",
		SilenceUsage: true,
		Long: "Checks an existing agentic environment for correctness and repairs\n" +
			"what it can automatically. Each check shows ✔ pass, ⚠ warning, or ✖ fail.\n" +
			"Pass --repair to attempt automatic fixes for failed checks.\n" +
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

			return runDoctor(w, cmd.InOrStdin(), doctorConfig{
				root:         root,
				repoFullName: currentRepo.Owner + "/" + currentRepo.Name,
				owner:        currentRepo.Owner,
				repoName:     currentRepo.Name,
				run:          bootstrap.DefaultRunCommand,
				repair:       repair,
				yes:          yes,
			})
		},
	}

	cmd.Flags().BoolVar(&repair, "repair", false, "attempt automatic repair of failed checks")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "automatically confirm all repair prompts")
	return cmd
}
