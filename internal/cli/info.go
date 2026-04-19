package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/eddiecarpenter/gh-agentic/internal/mount"
	"github.com/eddiecarpenter/gh-agentic/internal/project"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

const infoLabelWidth = 24

// infoData holds all remotely-fetched data for the info display.
type infoData struct {
	// extension
	version   string
	date      string
	installed string

	// project
	repoLabel    string
	projectLine  string
	projectHint  string
	topology     string
	controlPlane string

	// framework
	localVersion  string
	remoteVersion string
	latestVersion string

	// computed status
	noRepo      bool
	noProject   bool
	syncStatus  string // inline suffix for remote row
	latestStatus string // inline suffix for latest row
}

// newInfoCmd constructs the top-level `gh agentic info` command.
// It replaces both `gh agentic version` and `gh agentic project info`.
func newInfoCmd(version, date string) *cobra.Command {
	return newInfoCmdWithDeps(version, date, ui.RunWithSpinner, mount.DefaultFetchReleases)
}

// newInfoCmdWithDeps is the injectable form used in tests.
func newInfoCmdWithDeps(
	version, date string,
	spinner ui.SpinnerFunc,
	fetchReleases func(repo string) ([]mount.Release, error),
) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show environment info: CLI version, project, topology, and framework versions",
		Long: `Display a complete overview of the current agentic environment.

Shows the extension version and installation date, the current repo's project
affiliation and topology, and the framework versions: locally mounted, remote
(control plane authoritative), and the latest available release.`,
		Example: `  gh agentic info`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			var data infoData

			// Gather everything remotely before printing.
			_ = spinner(w, "Fetching info from remote...", func() error {
				collectInfo(&data, version, date, fetchReleases)
				return nil
			})

			// Print everything at once.
			printInfo(w, &data)
			return nil
		},
	}
}

// collectInfo populates data with all local and remote information.
// Never returns an error — all failures are reflected in the data fields.
func collectInfo(data *infoData, version, date string, fetchReleases func(repo string) ([]mount.Release, error)) {
	data.version = version

	// Release date.
	if date != "" {
		if t, err := time.Parse(time.RFC3339, date); err == nil {
			data.date = t.UTC().Format("2006-01-02")
		} else {
			data.date = date
		}
	}

	// Installation date — mod time of the running binary.
	if exe, err := os.Executable(); err == nil {
		if info, err := os.Stat(exe); err == nil {
			data.installed = info.ModTime().Local().Format("2006-01-02 15:04:05")
		}
	}

	// Project state.
	deps, err := resolveProjectDeps()
	if err != nil {
		data.noRepo = true
		// Still try to get framework (latest) even without a repo context.
		data.latestVersion, data.latestStatus = fetchLatest(fetchReleases, "")
		return
	}

	data.repoLabel = deps.Owner + "/" + deps.RepoName

	// Single canonical read — no direct AGENTIC_* access in this file.
	ctx, err := project.Resolve(deps)
	if err != nil || ctx == nil || ctx.ProjectID == "" {
		data.noProject = true
		data.localVersion, data.remoteVersion = localFrameworkVersion(deps.Root), ""
		data.latestVersion, data.latestStatus = fetchLatest(fetchReleases, data.localVersion)
		return
	}

	// Project fields.
	if ctx.ProjectDeleted {
		data.projectLine = ui.StatusWarning.Render("⚠ agentic project not found — may have been deleted") + " (" + ctx.ProjectID + ")"
		data.projectHint = "→ run 'gh agentic project unlink' or 'gh agentic project init'"
	} else {
		data.projectLine = ctx.ProjectName + " (" + ctx.ProjectID + ")"
	}
	// The info UI uses the legacy Topology enum string for display ("Single"
	// vs "Federated"). Derive it from the graph — RoleDomain is the only
	// case where the CP is a different repo.
	if ctx.Role == project.RoleDomain {
		data.topology = string(project.TopologyFederated)
	} else {
		data.topology = string(project.TopologySingle)
	}
	if ctx.Role == project.RoleDomain && ctx.ControlPlane.NameWithOwner != "" {
		data.controlPlane = ctx.ControlPlane.NameWithOwner
	}

	// Framework versions.
	data.localVersion = ctx.LocalAIVersion
	if data.localVersion == "" {
		data.localVersion = localFrameworkVersion(deps.Root)
	}
	data.remoteVersion = ctx.FrameworkVersion

	// Sync status (remote vs local).
	if data.remoteVersion != "" && data.localVersion != "" {
		if data.localVersion == data.remoteVersion {
			data.syncStatus = "  " + ui.StatusOK.Render("✓ in sync")
		} else {
			data.syncStatus = "  " + ui.StatusWarning.Render("⚠ run 'gh agentic mount' to sync")
		}
	}

	// Latest release.
	data.latestVersion, data.latestStatus = fetchLatest(fetchReleases, data.localVersion)
}

// fetchLatest retrieves the latest framework release tag and computes a status suffix.
func fetchLatest(fetchReleases func(repo string) ([]mount.Release, error), localVersion string) (string, string) {
	if fetchReleases == nil {
		return "", ""
	}
	releases, err := fetchReleases(mount.FrameworkRepo)
	if err != nil || len(releases) == 0 {
		return ui.Muted.Render("unavailable"), ""
	}
	latest := releases[0].TagName
	if localVersion == "" {
		return latest, ""
	}
	if latest == localVersion {
		return latest, "  " + ui.StatusOK.Render("✓ up to date")
	}
	return latest, "  " + ui.StatusWarning.Render("⚠ update available")
}

// localFrameworkVersion reads the locally mounted framework version from
// the .ai/ git metadata — the only local source of truth now that the
// .ai-version flat file has been removed (#585).
func localFrameworkVersion(root string) string {
	if v, err := mount.ReadAIVersionFromGit(root); err == nil {
		return v
	}
	return ""
}

// printInfo renders the collected data to w.
func printInfo(w io.Writer, data *infoData) {
	fmt.Fprintln(w, "")

	// --- Extension section ---
	fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Extension"))
	fmt.Fprintln(w, "  "+ui.Divider(48))
	fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Version:", data.version)
	if data.date != "" {
		fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Released:", data.date)
	} else {
		fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Released:", ui.Muted.Render("n/a (dev build)"))
	}
	if data.installed != "" {
		fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Installed:", data.installed)
	}

	// --- Agentic Project section ---
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Agentic Project"))
	fmt.Fprintln(w, "  "+ui.Divider(48))

	if data.noRepo {
		fmt.Fprintf(w, "  %s\n", ui.Muted.Render("Not in a GitHub repository or no remote configured"))
	} else {
		fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Repo:", data.repoLabel)
		if data.noProject {
			fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Agentic project:", ui.Muted.Render("not part of an agentic project — run 'gh agentic project init'"))
		} else {
			fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Agentic project:", data.projectLine)
			if data.projectHint != "" {
				fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "", ui.Muted.Render(data.projectHint))
			}
			fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Topology:", data.topology)
			if data.controlPlane != "" {
				fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Control plane:", data.controlPlane)
			}
		}
	}

	// --- Framework section ---
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  "+ui.SectionHeading.Render("Framework"))
	fmt.Fprintln(w, "  "+ui.Divider(48))

	if data.localVersion != "" {
		fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Framework (local):", data.localVersion)
	} else {
		fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Framework (local):", ui.Muted.Render("not mounted"))
	}

	if data.remoteVersion != "" {
		fmt.Fprintf(w, "  %-*s %s%s\n", infoLabelWidth, "Framework (remote):", data.remoteVersion, data.syncStatus)
	} else {
		fmt.Fprintf(w, "  %-*s %s\n", infoLabelWidth, "Framework (remote):", ui.Muted.Render("not set"))
	}

	fmt.Fprintf(w, "  %-*s %s%s\n", infoLabelWidth, "Framework (latest):", data.latestVersion, data.latestStatus)

	fmt.Fprintln(w, "")
}

// printInfoToString is a test helper that renders info into a string buffer.
func printInfoToString(data *infoData) string {
	var buf bytes.Buffer
	printInfo(&buf, data)
	return buf.String()
}
