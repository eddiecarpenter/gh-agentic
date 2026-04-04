package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
	"github.com/eddiecarpenter/gh-agentic/internal/verify"
)

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

			fmt.Fprintln(w, ui.SectionHeading.Render("  Doctor — check agentic environment"))
			fmt.Fprintln(w)

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
			owner := currentRepo.Owner
			repoName := currentRepo.Name
			repoFullName := owner + "/" + repoName

			// Single scanner shared across all confirm functions — creating
			// multiple scanners on the same stdin causes buffering issues
			// where the first scanner consumes input meant for later calls.
			scanner := bufio.NewScanner(cmd.InOrStdin())

			// Confirm functions for repair interactions.
			// When --yes is set, all prompts are auto-confirmed.
			textConfirm := func(prompt string) (string, error) {
				if yes {
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
				if yes {
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

			run := bootstrap.DefaultRunCommand

			// All checks in pipeline order.
			checks := []verify.CheckFunc{
				func() verify.CheckResult { return verify.CheckCLAUDEMD(root) },
				func() verify.CheckResult { return verify.CheckAGENTSLocalMD(root) },
				func() verify.CheckResult { return verify.CheckTEMPLATESOURCE(root) },
				func() verify.CheckResult { return verify.CheckTEMPLATEVERSION(root) },
				func() verify.CheckResult { return verify.CheckREPOSMD(root) },
				func() verify.CheckResult { return verify.CheckREADMEMD(root) },
				func() verify.CheckResult { return verify.CheckBaseDir(root, run) },
				func() verify.CheckResult { return verify.CheckBaseRecipes(root, run) },
				func() verify.CheckResult { return verify.CheckGooseRecipes(root) },
				func() verify.CheckResult { return verify.CheckWorkflows(root) },
				func() verify.CheckResult { return verify.CheckLabels(repoFullName, run) },
				func() verify.CheckResult { return verify.CheckProject(owner, run) },
			}

			// Repair function — only active when --repair flag is set.
			var repairFn verify.RepairFunc
			if repair {
				repairFn = func(result verify.CheckResult) *verify.CheckResult {
					var r verify.CheckResult
					switch result.Name {
					case "CLAUDE.md exists":
						r = verify.RepairCLAUDEMD(root)
					case "AGENTS.local.md exists":
						r = verify.RepairAGENTSLocalMD(root)
					case "TEMPLATE_SOURCE exists":
						r = verify.RepairTEMPLATESOURCE(root, textConfirm)
					case "TEMPLATE_VERSION exists":
						r = verify.RepairTEMPLATEVERSION(root, run)
					case "REPOS.md exists":
						r = verify.RepairREPOSMD(root)
					case "README.md exists":
						r = verify.RepairREADMEMD(root)
					case "base/ exists and is unmodified":
						r = verify.RepairBaseDirWithWriter(w, root, run, boolConfirm)
					case "base/skills/*.md unmodified":
						r = verify.RepairBaseRecipes(root, run, boolConfirm)
					case ".goose/recipes/ exists and complete":
						r = verify.RepairGooseRecipes(root)
					case ".github/workflows/ exists and complete":
						r = verify.RepairWorkflows(root)
					case "Standard labels present":
						r = verify.RepairLabels(repoFullName, run)
					case "GitHub Project linked":
						r = verify.RepairProject(owner, repoName, run)
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
		},
	}

	cmd.Flags().BoolVar(&repair, "repair", false, "attempt automatic repair of failed checks")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "automatically confirm all repair prompts")
	return cmd
}
