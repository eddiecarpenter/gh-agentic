package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eddiecarpenter/gh-agentic/internal/sync"
	"github.com/eddiecarpenter/gh-agentic/internal/tarball"
)

// ValidateTag checks that the requested version tag exists among available
// releases. If the tag is not found, returns an error that includes the
// latest available version for guidance.
func ValidateTag(version string, releases []sync.Release) error {
	_, found := sync.FindReleaseByTag(releases, version)
	if found {
		return nil
	}

	// Determine latest for the error message.
	latest := "unknown"
	if len(releases) > 0 {
		latest = releases[0].TagName
	}

	return fmt.Errorf("version %s not found — latest available version is %s", version, latest)
}

// DownloadFramework downloads the framework release tarball and extracts
// only framework files to the .ai/ directory under destRoot.
//
// Framework files are: RULEBOOK.md, skills/, recipes/, standards/, concepts/.
// These are extracted from the repo root of the release tarball into
// destRoot/.ai/.
func DownloadFramework(destRoot, version string, fetch FetchTarballFunc) error {
	if version == "" {
		return fmt.Errorf("version is empty — cannot download framework")
	}

	aiDir := filepath.Join(destRoot, ".ai")

	// Remove existing .ai/ directory for a clean mount.
	if err := os.RemoveAll(aiDir); err != nil {
		return fmt.Errorf("removing existing .ai/: %w", err)
	}

	// Use the tarball package to download and extract.
	// We extract framework prefixes into destRoot/.ai/.
	err := tarball.ExtractFromTemplate(FrameworkRepo, version, aiDir, FrameworkPrefixes, fetch)
	if err != nil {
		return fmt.Errorf("downloading framework: %w", err)
	}

	return nil
}

// ReadAIVersion reads the .ai-version file from the given root directory.
// Returns the version string (trimmed) and nil error if the file exists.
// Returns empty string and error if the file does not exist or is empty.
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
// Creates .gitignore if it does not exist. Appends the entry if not present.
func EnsureGitignore(root string) error {
	gitignorePath := filepath.Join(root, ".gitignore")

	var content string
	data, err := os.ReadFile(gitignorePath)
	if err == nil {
		content = string(data)
	}

	// Check if .ai/ is already in .gitignore.
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == ".ai/" {
			return nil // Already present.
		}
	}

	// Append .ai/ entry.
	entry := ".ai/\n"
	if content != "" && !strings.HasSuffix(content, "\n") {
		entry = "\n" + entry
	}

	return os.WriteFile(gitignorePath, []byte(content+entry), 0o644)
}
