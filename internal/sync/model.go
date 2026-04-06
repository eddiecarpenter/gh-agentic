// Package sync implements the business logic for gh agentic sync.
// It syncs the base/ directory from the upstream agentic-development template.
package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SyncConfig holds all state needed to execute a template sync.
type SyncConfig struct {
	// TemplateRepo is the GitHub repo slug (e.g. "eddiecarpenter/ai-native-delivery").
	TemplateRepo string

	// CurrentVersion is the semver tag last synced (e.g. "v0.1.0").
	CurrentVersion string

	// LatestVersion is the newest release tag from the upstream template.
	LatestVersion string

	// RepoRoot is the absolute path to the agentic repo root.
	RepoRoot string
}

// ReadTemplateSource reads and trims the TEMPLATE_SOURCE file from the given repo root.
// Returns the GitHub repo slug (e.g. "eddiecarpenter/ai-native-delivery").
func ReadTemplateSource(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "TEMPLATE_SOURCE"))
	if err != nil {
		return "", fmt.Errorf("reading TEMPLATE_SOURCE: %w", err)
	}

	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("TEMPLATE_SOURCE is empty")
	}

	return value, nil
}

// ReadTemplateVersion reads and trims the TEMPLATE_VERSION file from the given repo root.
// Returns the semver tag (e.g. "v0.1.0").
func ReadTemplateVersion(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "TEMPLATE_VERSION"))
	if err != nil {
		return "", fmt.Errorf("reading TEMPLATE_VERSION: %w", err)
	}

	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", fmt.Errorf("TEMPLATE_VERSION is empty")
	}

	return value, nil
}
