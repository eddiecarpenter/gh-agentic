package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FederationRepo represents a single target repository listed in FEDERATION.md.
type FederationRepo struct {
	// Name is the full "owner/repo" identifier of the target repository.
	Name string `yaml:"name"`
	// Purpose is a human-readable description of this repo's role in the federation.
	Purpose string `yaml:"purpose"`
}

// Federation holds the parsed contents of a FEDERATION.md manifest file.
type Federation struct {
	// Repos is the list of target repositories declared in the manifest.
	Repos []FederationRepo `yaml:"repos"`
}

// federationFileName is the canonical name of the federation manifest file.
const federationFileName = "FEDERATION.md"

// IsFederationRepo returns true when a FEDERATION.md file exists at root.
// It uses os.Stat only — no YAML decode — so it is fast and has no side effects.
// A false return means the repo is single topology; no further federation config
// is required.
func IsFederationRepo(root string) bool {
	_, err := os.Stat(filepath.Join(root, federationFileName))
	return err == nil
}

// ReadFederation reads and validates the FEDERATION.md manifest at root.
// It returns the first validation error found:
//   - file present but empty → "FEDERATION.md: file is empty"
//   - YAML parse error → "FEDERATION.md: YAML parse error: <detail>"
//   - repos list absent or empty → "FEDERATION.md: repos list is empty"
//   - entry N missing name → "FEDERATION.md: entry N: name is required"
//   - entry N missing/blank purpose → "FEDERATION.md: entry N: purpose is required"
//   - entry N name not in owner/repo format → "FEDERATION.md: entry N: name must be in owner/repo format"
//   - duplicate name → "FEDERATION.md: duplicate repo <name>"
func ReadFederation(root string) (*Federation, error) {
	path := filepath.Join(root, federationFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("FEDERATION.md: %w", err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("FEDERATION.md: file is empty")
	}

	var fed Federation
	if err := yaml.Unmarshal(data, &fed); err != nil {
		return nil, fmt.Errorf("FEDERATION.md: YAML parse error: %s", err.Error())
	}

	if len(fed.Repos) == 0 {
		return nil, fmt.Errorf("FEDERATION.md: repos list is empty")
	}

	seen := make(map[string]bool, len(fed.Repos))
	for i, repo := range fed.Repos {
		entry := i + 1
		if repo.Name == "" {
			return nil, fmt.Errorf("FEDERATION.md: entry %d: name is required", entry)
		}
		if strings.TrimSpace(repo.Purpose) == "" {
			return nil, fmt.Errorf("FEDERATION.md: entry %d: purpose is required", entry)
		}
		if !isValidOwnerRepo(repo.Name) {
			return nil, fmt.Errorf("FEDERATION.md: entry %d: name must be in owner/repo format", entry)
		}
		lower := strings.ToLower(repo.Name)
		if seen[lower] {
			return nil, fmt.Errorf("FEDERATION.md: duplicate repo %q", repo.Name)
		}
		seen[lower] = true
	}

	return &fed, nil
}

// isValidOwnerRepo returns true when name is in "owner/repo" format with
// non-empty owner and repo parts. A simple split-count check per the task spec.
func isValidOwnerRepo(name string) bool {
	parts := strings.SplitN(name, "/", 3)
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}
