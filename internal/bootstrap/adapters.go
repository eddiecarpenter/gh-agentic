package bootstrap

// adapters.go contains the production implementations of all injected function
// types in the bootstrap package. These functions bridge to real external services
// (GitHub API, shell, filesystem) and cannot be unit tested without live credentials.
// They are excluded from SonarCloud coverage measurement via **/*_adapters.go.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// ghOwnerTypeResp is the API response shape for GET /users/<owner>.
type ghOwnerTypeResp struct {
	Type string `json:"type"`
}

// DefaultDetectOwnerType calls GET users/<owner> via the authenticated go-gh/v2 REST client
// and returns OwnerTypeUser or OwnerTypeOrg based on the response "type" field.
func DefaultDetectOwnerType(owner string) (string, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return "", fmt.Errorf("creating GitHub API client: %w", err)
	}

	var resp ghOwnerTypeResp
	if err := client.Get(fmt.Sprintf("users/%s", owner), &resp); err != nil {
		return "", fmt.Errorf("fetching owner type for %q: %w", owner, err)
	}

	switch resp.Type {
	case OwnerTypeUser:
		return OwnerTypeUser, nil
	case OwnerTypeOrg:
		return OwnerTypeOrg, nil
	default:
		return "", fmt.Errorf("unexpected owner type %q for %q", resp.Type, owner)
	}
}

// DefaultLookPath is the production implementation of LookPathFunc.
// It delegates to exec.LookPath.
func DefaultLookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// DefaultRunCommand is the production implementation of RunCommandFunc.
// It runs the command and returns its combined stdout+stderr output.
func DefaultRunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// DefaultWorkDirOrHome returns the working directory, falling back to the user's
// home directory if os.Getwd() fails (e.g. in a deleted directory).
func DefaultWorkDirOrHome() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Development")
	}
	return "."
}

// DefaultSpinner is the production SpinnerFunc. It uses a simple inline approach:
// print the step label, run fn, then overwrite with ✔ or ✖.
func DefaultSpinner(w io.Writer, label string, fn func() error) error {
	fmt.Fprintln(w, "  "+ui.Muted.Render("⠸ "+label+"..."))
	if err := fn(); err != nil {
		fmt.Fprintln(w, "  "+ui.RenderError(label+": "+err.Error()))
		return err
	}
	fmt.Fprintln(w, "  "+ui.RenderOK(label))
	return nil
}

// DefaultGraphQLDo returns a GraphQLDoFunc backed by the go-gh/v2 GraphQL client.
func DefaultGraphQLDo() (GraphQLDoFunc, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client: %w", err)
	}
	return func(query string, variables map[string]interface{}, response interface{}) error {
		return client.Do(query, variables, response)
	}, nil
}

// DefaultResolveCloneConflict checks whether the target directory exists. If it does,
// it presents the user with recovery options: rename to backup or abort.
// Returns the resolved clone path or an error.
func DefaultResolveCloneConflict(w io.Writer, clonePath string) (string, error) {
	if _, err := os.Stat(clonePath); os.IsNotExist(err) {
		return clonePath, nil // No conflict — proceed normally.
	}

	fmt.Fprintln(w, "  "+ui.RenderWarning(fmt.Sprintf("Directory %q already exists", filepath.Base(clonePath))))

	var choice string
	conflictForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Directory already exists").
				Description("Choose how to resolve the conflict").
				Options(
					huh.NewOption("Rename existing to .backup and continue", cloneConflictRename),
					huh.NewOption("Abort", cloneConflictAbort),
				).
				Value(&choice),
		),
	)
	if err := conflictForm.Run(); err != nil {
		return "", fmt.Errorf("clone conflict form: %w", err)
	}

	switch choice {
	case cloneConflictRename:
		backupPath := findBackupPath(clonePath)
		if err := os.Rename(clonePath, backupPath); err != nil {
			return "", fmt.Errorf("renaming %s to %s: %w", clonePath, backupPath, err)
		}
		fmt.Fprintln(w, "  "+ui.Muted.Render(fmt.Sprintf("· Renamed existing directory to %s", filepath.Base(backupPath))))
		return clonePath, nil
	case cloneConflictAbort:
		return "", ErrCloneAborted
	default:
		return "", ErrCloneAborted
	}
}
