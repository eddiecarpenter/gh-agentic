package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/initv2"
	"github.com/eddiecarpenter/gh-agentic/internal/tarball"

	ghAPI "github.com/cli/go-gh/v2/pkg/api"
)

// initDeps holds injectable dependencies for the init command.
type initDeps struct {
	run          initv2.RunCommandFunc
	fetchTarball initv2.Deps
}

// defaultDetectOwnerType detects whether a GitHub owner is a user or org via the API.
func defaultDetectOwnerType(owner string) (string, error) {
	client, err := ghAPI.DefaultRESTClient()
	if err != nil {
		return "", fmt.Errorf("creating GitHub API client: %w", err)
	}

	var resp struct {
		Type string `json:"type"`
	}
	if err := client.Get(fmt.Sprintf("users/%s", owner), &resp); err != nil {
		return "", fmt.Errorf("detecting owner type for %q: %w", owner, err)
	}
	return resp.Type, nil
}

// newInitCmd constructs the `gh agentic --v2 init` command with production deps.
func newInitCmd() *cobra.Command {
	return newInitCmdWithDeps(initv2.Deps{
		Run:          bootstrap.DefaultRunCommand,
		FetchTarball: tarball.DefaultFetch,
		CollectConfig: func(w io.Writer, repo string) (*initv2.InitConfig, error) {
			return initv2.CollectConfigInteractive(w, repo, initv2.FormDeps{
				RunForm:         initv2.DefaultFormRun,
				RunCommand:      bootstrap.DefaultRunCommand,
				DetectOwnerType: defaultDetectOwnerType,
			})
		},
	})
}

// newInitCmdWithDeps constructs the init command with injectable dependencies.
func newInitCmdWithDeps(deps initv2.Deps) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise a new agentic environment (v2)",
		Long: "Interactive wizard to configure a new agentic environment.\n" +
			"Detects the current repo, collects configuration, mounts the framework,\n" +
			"and configures secrets and variables.\n" +
			"Blocked if .ai-version exists unless --force is passed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !v2FlagValue {
				return fmt.Errorf("init requires the --v2 flag: gh agentic --v2 init")
			}

			w := cmd.OutOrStdout()

			root, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}

			return initv2.Run(w, root, force, deps)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing configuration")

	return cmd
}
