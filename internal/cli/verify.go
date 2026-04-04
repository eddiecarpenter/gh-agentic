package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
	"github.com/eddiecarpenter/gh-agentic/internal/verify"
)

// newVerifyCmd constructs the `gh agentic verify` subcommand.
func newVerifyCmd() *cobra.Command {
	var repair bool

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify an agentic environment for correctness",
		Long: "Checks an existing agentic environment for correctness and repairs\n" +
			"what it can automatically. Each check shows ✔ pass, ⚠ warning, or ✖ fail.\n" +
			"Pass --repair to attempt automatic fixes for failed checks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			fmt.Fprintln(w, ui.SectionHeading.Render("  Verify — check agentic environment"))
			fmt.Fprintln(w)

			// Resolve repo root.
			root, err := findRepoRoot()
			if err != nil {
				return err
			}

			// Resolve repo full name (owner/repo) and owner from gh.
			repoFullName, err := bootstrap.DefaultRunCommand("gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
			if err != nil {
				return fmt.Errorf("resolving repo name: %w", err)
			}
			repoFullName = strings.TrimSpace(repoFullName)
			parts := strings.SplitN(repoFullName, "/", 2)
			owner := parts[0]
			repoName := ""
			if len(parts) == 2 {
				repoName = parts[1]
			}

			// Confirm functions for repair interactions.
			textConfirm := func(prompt string) (string, error) {
				fmt.Fprintf(w, "  %s: ", prompt)
				scanner := bufio.NewScanner(cmd.InOrStdin())
				if scanner.Scan() {
					return strings.TrimSpace(scanner.Text()), nil
				}
				return "", scanner.Err()
			}
			boolConfirm := func(prompt string) (bool, error) {
				fmt.Fprintf(w, "  %s [y/N]: ", prompt)
				scanner := bufio.NewScanner(cmd.InOrStdin())
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
					case "base/ directory integrity":
						r = verify.RepairBaseDir(root, run, boolConfirm)
					case "base/skills/ integrity":
						r = verify.RepairBaseRecipes(root, run, boolConfirm)
					case ".goose/recipes/ exists and complete":
						r = verify.RepairGooseRecipes(root)
					case ".github/workflows/ exists and complete":
						r = verify.RepairWorkflows(root)
					case "GitHub labels configured":
						r = verify.RepairLabels(repoFullName, run)
					case "GitHub Project exists":
						r = verify.RepairProject(owner, repoName, run)
					default:
						return nil
					}
					return &r
				}
			}

			return verify.RunVerify(w, checks, repairFn)
		},
	}

	cmd.Flags().BoolVar(&repair, "repair", false, "attempt automatic repair of failed checks")
	return cmd
}

