package sync

// adapters.go contains the production implementations of all injected function
// types in the sync package. These functions bridge to real external services
// (GitHub API) or TTY interaction and cannot be unit tested without live
// credentials or a terminal.
// They are excluded from SonarCloud coverage measurement via **/*_adapters.go.

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// ghReleaseResp is the API response shape for GET /repos/{owner}/{repo}/releases/latest.
type ghReleaseResp struct {
	TagName string `json:"tag_name"`
}

// DefaultFetchRelease fetches the latest release tag using the authenticated go-gh/v2 REST client.
func DefaultFetchRelease(repo string) (string, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return "", fmt.Errorf("creating GitHub API client: %w", err)
	}

	var release ghReleaseResp
	endpoint := fmt.Sprintf("repos/%s/releases/latest", repo)

	if err := client.Get(endpoint, &release); err != nil {
		return "", fmt.Errorf("fetching latest release for %s: %w", repo, err)
	}

	tag := strings.TrimSpace(release.TagName)
	if tag == "" {
		return "", fmt.Errorf("no release tag found for %s", repo)
	}

	return tag, nil
}

// DefaultFetchReleases fetches all releases from the GitHub API using pagination.
// Returns releases ordered newest-first (as returned by the API).
func DefaultFetchReleases(repo string) ([]Release, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("creating GitHub API client: %w", err)
	}

	var all []Release
	page := 1

	for {
		var batch []Release
		endpoint := fmt.Sprintf("repos/%s/releases?per_page=100&page=%d", repo, page)

		if err := client.Get(endpoint, &batch); err != nil {
			return nil, fmt.Errorf("fetching releases for %s (page %d): %w", repo, page, err)
		}

		if len(batch) == 0 {
			break
		}

		all = append(all, batch...)
		page++
	}

	return all, nil
}

// DefaultSpinner is the production SpinnerFunc. Prints "⠸ label..." then
// "✔ label" or "✖ label: error".
func DefaultSpinner(w io.Writer, label string, fn func() error) error {
	fmt.Fprintln(w, "  "+ui.Muted.Render("⠸ "+label+"..."))
	if err := fn(); err != nil {
		fmt.Fprintln(w, "  "+ui.RenderError(label+": "+err.Error()))
		return err
	}
	fmt.Fprintln(w, "  "+ui.RenderOK(label))
	return nil
}

// DefaultConfirm is the production ConfirmFunc. Uses huh to present a
// yes/no confirmation to the user.
func DefaultConfirm(prompt string) (bool, error) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(prompt).
				Affirmative("Yes").
				Negative("No").
				Value(&confirmed),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

// DefaultClear is the production ClearFunc. Delegates to ui.ClearScreen
// to clear the terminal and reset the cursor position.
func DefaultClear(w io.Writer) {
	ui.ClearScreen(w)
}

// DefaultSelect is the production SelectFunc. Uses huh.Select to present an
// interactive version picker. Each option shows the tag and release name;
// the description shows the release body (notes).
func DefaultSelect(releases []Release) (Release, error) {
	opts := make([]huh.Option[int], len(releases))
	for i, r := range releases {
		label := r.TagName
		if r.Name != "" {
			label += " — " + r.Name
		}
		opts[i] = huh.NewOption(label, i)
	}

	var selected int
	sel := huh.NewSelect[int]().
		Title("Select a version to sync to").
		Options(opts...).
		Value(&selected)

	form := huh.NewForm(huh.NewGroup(sel))
	if err := form.Run(); err != nil {
		return Release{}, err
	}

	return releases[selected], nil
}
