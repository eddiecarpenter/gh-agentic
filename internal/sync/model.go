// Package sync implements the business logic for gh agentic sync.
// It syncs the .ai/ directory from the upstream agentic-development template.
package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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

// AIConfig represents the contents of .ai/config.yml.
type AIConfig struct {
	Template string `yaml:"template"`
	Version  string `yaml:"version"`
}

// ReadAIConfig reads and parses .ai/config.yml from the given repo root.
func ReadAIConfig(root string) (AIConfig, error) {
	data, err := os.ReadFile(filepath.Join(root, ".ai", "config.yml"))
	if err != nil {
		return AIConfig{}, err
	}
	var cfg AIConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return AIConfig{}, fmt.Errorf("parsing .ai/config.yml: %w", err)
	}
	return cfg, nil
}

// ReadTemplateSource reads the template repo slug from .ai/config.yml.
// Falls back to TEMPLATE_SOURCE for repos that have not yet been migrated.
//
// TODO(deprecated): remove TEMPLATE_SOURCE fallback in next major version — .ai/config.yml is the source of truth.
func ReadTemplateSource(root string) (string, error) {
	if cfg, err := ReadAIConfig(root); err == nil {
		if value := strings.TrimSpace(cfg.Template); value != "" {
			return value, nil
		}
	}

	// TODO(deprecated): remove in next major version — TEMPLATE_SOURCE fallback for pre-v1.5.0 repos.
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

// ReadTemplateVersion reads the current template version from .ai/config.yml.
// Falls back to TEMPLATE_VERSION for repos that have not yet been migrated.
//
// TODO(deprecated): remove TEMPLATE_VERSION fallback in next major version — .ai/config.yml is the source of truth.
func ReadTemplateVersion(root string) (string, error) {
	if cfg, err := ReadAIConfig(root); err == nil {
		if value := strings.TrimSpace(cfg.Version); value != "" {
			return value, nil
		}
	}

	// TODO(deprecated): remove in next major version — TEMPLATE_VERSION fallback for pre-v1.5.0 repos.
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
