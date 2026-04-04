package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// newVersionCmd constructs the `gh agentic version` subcommand.
func newVersionCmd(version, date string) *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        "Show version, release date and installation date",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			fmt.Fprintln(w, ui.SectionHeading.Render("  gh agentic — version info"))
			fmt.Fprintln(w)

			fmt.Fprintf(w, "  %-16s %s\n", "Version:", version)

			// Release date — injected by GoReleaser as RFC3339; blank for dev builds.
			if date != "" {
				if t, err := time.Parse(time.RFC3339, date); err == nil {
					fmt.Fprintf(w, "  %-16s %s\n", "Released:", t.UTC().Format("2006-01-02"))
				} else {
					fmt.Fprintf(w, "  %-16s %s\n", "Released:", date)
				}
			} else {
				fmt.Fprintf(w, "  %-16s %s\n", "Released:", "n/a (dev build)")
			}

			// Install date — modification time of the running binary.
			if exe, err := os.Executable(); err == nil {
				if info, err := os.Stat(exe); err == nil {
					fmt.Fprintf(w, "  %-16s %s\n", "Installed:", info.ModTime().Local().Format("2006-01-02 15:04:05"))
				}
			}

			fmt.Fprintln(w)
			return nil
		},
	}
}
