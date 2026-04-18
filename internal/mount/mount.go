package mount

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ValidateTag checks that the requested version tag exists among available
// releases. If the tag is not found, returns an error that includes the
// latest available version for guidance.
func ValidateTag(version string, releases []Release) error {
	_, found := FindReleaseByTag(releases, version)
	if found {
		return nil
	}

	latest := "unknown"
	if len(releases) > 0 {
		latest = releases[0].TagName
	}

	return fmt.Errorf("version %s not found — latest available version is %s", version, latest)
}

// DefaultClone performs a shallow git clone of repoURL at the given tag into
// destDir. The .git/ directory is retained so that the mounted version can be
// read from git metadata and the clone is left in detached HEAD state (no branch
// to push to).
func DefaultClone(repoURL, tag, destDir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", tag, repoURL, destDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ReadAIVersionFromGit reads the mounted framework version from the .git
// metadata inside .ai/. This is the authoritative source of truth — it reflects
// exactly what was cloned, not what .ai-version says.
func ReadAIVersionFromGit(root string) (string, error) {
	aiDir := filepath.Join(root, ".ai")
	cmd := exec.Command("git", "-C", aiDir, "describe", "--tags", "--exact-match")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("reading version from .ai/.git: %w", err)
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", fmt.Errorf(".ai/.git has no version tag")
	}
	return v, nil
}

// DownloadFramework clones the framework at the given version tag into
// destRoot/.ai/ using a shallow git clone. Any existing .ai/ is removed first.
func DownloadFramework(destRoot, version string, clone CloneFunc) error {
	if version == "" {
		return fmt.Errorf("version is empty — cannot download framework")
	}

	aiDir := filepath.Join(destRoot, ".ai")

	// Remove existing .ai/ for a clean mount.
	if err := os.RemoveAll(aiDir); err != nil {
		return fmt.Errorf("removing existing .ai/: %w", err)
	}

	if err := clone(FrameworkRepoURL, version, aiDir); err != nil {
		return fmt.Errorf("cloning framework: %w", err)
	}

	return nil
}

// ReadAIVersion reads the .ai-version file from the given root directory.
func ReadAIVersion(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, ".ai-version"))
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(data))
	if v == "" {
		return "", fmt.Errorf(".ai-version is empty")
	}
	return v, nil
}

// WriteAIVersion writes the version string to .ai-version in the given root.
func WriteAIVersion(root, version string) error {
	return os.WriteFile(filepath.Join(root, ".ai-version"), []byte(version+"\n"), 0o644)
}

// EnsureGitignore ensures that ".ai/" is listed in .gitignore at root.
func EnsureGitignore(root string) error {
	return ensureGitignoreEntry(root, ".ai/")
}

// ensureGitignoreEntry appends entry to .gitignore if it is not already
// present. The entry is matched line-for-line after trimming whitespace.
func ensureGitignoreEntry(root, entry string) error {
	gitignorePath := filepath.Join(root, ".gitignore")

	var content string
	data, err := os.ReadFile(gitignorePath)
	if err == nil {
		content = string(data)
	}

	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}

	toAppend := entry + "\n"
	if content != "" && !strings.HasSuffix(content, "\n") {
		toAppend = "\n" + toAppend
	}

	return os.WriteFile(gitignorePath, []byte(content+toAppend), 0o644)
}

// versionTagPattern matches @vX.Y.Z version tags in uses: lines for
// the gh-agentic reusable workflows.
var versionTagPattern = regexp.MustCompile(`(eddiecarpenter/gh-agentic/\.github/workflows/[^@]+)@(v[0-9]+\.[0-9]+\.[0-9]+[^\s]*)`)

// UpdateWorkflowVersions scans all .yml files in .github/workflows/ and
// replaces gh-agentic version tags with the new version.
func UpdateWorkflowVersions(root, newVersion string) error {
	workflowsDir := filepath.Join(root, ".github", "workflows")

	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading workflows directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".yml" && filepath.Ext(name) != ".yaml" {
			continue
		}

		path := filepath.Join(workflowsDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}

		updated := versionTagPattern.ReplaceAllString(string(data), "${1}@"+newVersion)
		if updated != string(data) {
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", name, err)
			}
		}
	}

	return nil
}
