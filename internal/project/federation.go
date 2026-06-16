package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// FederationRepo represents a single target repository within a domain.
type FederationRepo struct {
	// Name is the full "owner/repo" identifier of the target repository.
	Name string `yaml:"name"`
	// Purpose is a human-readable description of this repo's role in the domain.
	Purpose string `yaml:"purpose"`
}

// FederationDomain groups one or more repos under a named domain. A domain is
// the unit of documentation tiering: its docs live centrally on the control
// plane under docs/domains/<name>/.
type FederationDomain struct {
	// Name is a filesystem-safe slug; it maps to docs/domains/<name>/.
	Name string `yaml:"name"`
	// Purpose is a human-readable description of the domain.
	Purpose string `yaml:"purpose"`
	// Repos are the implementation repositories that make up this domain.
	// A single-repo domain is valid — its list has one entry.
	Repos []FederationRepo `yaml:"repos"`
}

// Federation holds the parsed contents of a FEDERATION.md manifest file.
type Federation struct {
	// Domains groups the federation's repos under the domains they implement.
	Domains []FederationDomain `yaml:"domains"`
}

// flatFederation is used only to detect the legacy flat `repos:` schema so
// ReadFederation can reject it with a migration-guiding error.
type flatFederation struct {
	Repos []FederationRepo `yaml:"repos"`
}

// federationFileName is the canonical name of the federation manifest file.
const federationFileName = "FEDERATION.md"

// domainNamePattern validates a domain name as a filesystem-safe slug, since
// each domain maps to a docs/domains/<name>/ folder.
var domainNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// AllRepos flattens the manifest's domains into a single ordered slice of
// repos, preserving domain and in-domain order. Consumers that need the flat
// target set use this rather than re-walking the nesting.
func (f *Federation) AllRepos() []FederationRepo {
	var repos []FederationRepo
	for _, d := range f.Domains {
		repos = append(repos, d.Repos...)
	}
	return repos
}

// IsFederationRepo returns true when a FEDERATION.md file exists at root.
// It uses os.Stat only — no YAML decode — so it is fast and has no side effects.
// A false return means the repo is single topology; no further federation config
// is required.
func IsFederationRepo(root string) bool {
	_, err := os.Stat(filepath.Join(root, federationFileName))
	return err == nil
}

// ReadFederation reads and validates the domain-grouped FEDERATION.md manifest
// at root. It returns the first validation error found:
//   - file present but empty → "FEDERATION.md: file is empty"
//   - YAML parse error → "FEDERATION.md: YAML parse error: <detail>"
//   - legacy flat `repos:` schema → "FEDERATION.md: flat `repos:` schema is no longer supported — group repos under `domains:`"
//   - domains list absent or empty → "FEDERATION.md: domains list is empty"
//   - domain missing name → "FEDERATION.md: domain N: name is required"
//   - domain name not a slug → "FEDERATION.md: domain <name>: name must be a lowercase slug ([a-z0-9-])"
//   - domain missing/blank purpose → "FEDERATION.md: domain <name>: purpose is required"
//   - duplicate domain → "FEDERATION.md: duplicate domain <name>"
//   - domain with no repos → "FEDERATION.md: domain <name>: repos list is empty"
//   - repo missing name → "FEDERATION.md: domain <name>: repo N: name is required"
//   - repo missing/blank purpose → "FEDERATION.md: domain <name>: repo <name>: purpose is required"
//   - repo name not owner/repo → "FEDERATION.md: domain <name>: repo <name>: name must be in owner/repo format"
//   - duplicate repo across the manifest → "FEDERATION.md: duplicate repo <name>"
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

	if len(fed.Domains) == 0 {
		// A flat `repos:` manifest is rejected with migration guidance; an empty
		// `domains:` list is valid — a freshly-created control plane with no
		// domains registered yet (domains are added later via `project join`).
		var flat flatFederation
		if yaml.Unmarshal(data, &flat) == nil && len(flat.Repos) > 0 {
			return nil, fmt.Errorf("FEDERATION.md: flat `repos:` schema is no longer supported — group repos under `domains:`")
		}
		return &fed, nil
	}

	if err := validateFederation(&fed); err != nil {
		return nil, err
	}
	return &fed, nil
}

// validateFederation applies the domain-grouped manifest rules. It is shared by
// ReadFederation (after parsing) and WriteFederation (before writing) so a
// malformed in-memory Federation never reaches disk.
func validateFederation(fed *Federation) error {
	// An empty domains list is valid — a control plane with no domains registered
	// yet. Per-domain/per-repo rules below only apply when domains are present.
	seenRepo := make(map[string]bool)
	seenDomain := make(map[string]bool)
	for di, d := range fed.Domains {
		domainNo := di + 1
		if d.Name == "" {
			return fmt.Errorf("FEDERATION.md: domain %d: name is required", domainNo)
		}
		if !domainNamePattern.MatchString(d.Name) {
			return fmt.Errorf("FEDERATION.md: domain %q: name must be a lowercase slug ([a-z0-9-])", d.Name)
		}
		if strings.TrimSpace(d.Purpose) == "" {
			return fmt.Errorf("FEDERATION.md: domain %q: purpose is required", d.Name)
		}
		domainKey := strings.ToLower(d.Name)
		if seenDomain[domainKey] {
			return fmt.Errorf("FEDERATION.md: duplicate domain %q", d.Name)
		}
		seenDomain[domainKey] = true

		if len(d.Repos) == 0 {
			return fmt.Errorf("FEDERATION.md: domain %q: repos list is empty", d.Name)
		}
		for ri, repo := range d.Repos {
			repoNo := ri + 1
			if repo.Name == "" {
				return fmt.Errorf("FEDERATION.md: domain %q: repo %d: name is required", d.Name, repoNo)
			}
			if strings.TrimSpace(repo.Purpose) == "" {
				return fmt.Errorf("FEDERATION.md: domain %q: repo %q: purpose is required", d.Name, repo.Name)
			}
			if !isValidOwnerRepo(repo.Name) {
				return fmt.Errorf("FEDERATION.md: domain %q: repo %q: name must be in owner/repo format", d.Name, repo.Name)
			}
			repoKey := strings.ToLower(repo.Name)
			if seenRepo[repoKey] {
				return fmt.Errorf("FEDERATION.md: duplicate repo %q", repo.Name)
			}
			seenRepo[repoKey] = true
		}
	}
	return nil
}

// HasDomain reports whether a domain with the given name (case-insensitive)
// already exists in the manifest.
func (f *Federation) HasDomain(name string) bool {
	for _, d := range f.Domains {
		if strings.EqualFold(d.Name, name) {
			return true
		}
	}
	return false
}

// AddRepo registers repoName under the named domain, lazy-creating the domain
// (with domainPurpose) when it does not yet exist. It returns whether a new
// domain was created. It errors when repoName is already registered in any
// domain — a repo belongs to exactly one domain.
func (f *Federation) AddRepo(domain, domainPurpose, repoName, repoPurpose string) (createdDomain bool, err error) {
	for _, d := range f.Domains {
		for _, r := range d.Repos {
			if strings.EqualFold(r.Name, repoName) {
				return false, fmt.Errorf("repo %q is already registered in domain %q", repoName, d.Name)
			}
		}
	}
	repo := FederationRepo{Name: repoName, Purpose: repoPurpose}
	for i := range f.Domains {
		if strings.EqualFold(f.Domains[i].Name, domain) {
			f.Domains[i].Repos = append(f.Domains[i].Repos, repo)
			return false, nil
		}
	}
	f.Domains = append(f.Domains, FederationDomain{Name: domain, Purpose: domainPurpose, Repos: []FederationRepo{repo}})
	return true, nil
}

// WriteFederation writes the domain-grouped manifest to FEDERATION.md at root,
// round-tripping the schema ReadFederation parses. The manifest is validated
// before writing so a malformed in-memory Federation never reaches disk.
func WriteFederation(root string, fed *Federation) error {
	if err := validateFederation(fed); err != nil {
		return err
	}
	data, err := yaml.Marshal(fed)
	if err != nil {
		return fmt.Errorf("FEDERATION.md: marshalling: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, federationFileName), data, 0644); err != nil {
		return fmt.Errorf("FEDERATION.md: %w", err)
	}
	return nil
}

// isValidOwnerRepo returns true when name is in "owner/repo" format with
// non-empty owner and repo parts.
func isValidOwnerRepo(name string) bool {
	parts := strings.SplitN(name, "/", 3)
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}
