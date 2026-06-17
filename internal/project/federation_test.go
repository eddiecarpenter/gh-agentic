package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validManifest is a domain-grouped FEDERATION.md fixture with two domains,
// one of which spans two repos.
const validManifest = `domains:
  - name: charging
    purpose: "Rating, balance, charging events"
    repos:
      - name: owner/charging-rating
        purpose: "Rating engine"
      - name: owner/charging-balance
        purpose: "Balance management"
  - name: billing
    purpose: "Invoice generation"
    repos:
      - name: owner/billing-domain
        purpose: "Bill runs and statements"
`

func writeManifest(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}
}

func TestIsFederationRepo_FilePresent_ReturnsTrue(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, validManifest)
	if !IsFederationRepo(dir) {
		t.Error("IsFederationRepo: expected true when FEDERATION.md exists, got false")
	}
}

func TestIsFederationRepo_FileAbsent_ReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	if IsFederationRepo(dir) {
		t.Error("IsFederationRepo: expected false when FEDERATION.md is absent, got true")
	}
}

func TestReadFederation_ValidManifest_ParsedCorrectly(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, validManifest)

	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("ReadFederation: unexpected error: %v", err)
	}
	if len(fed.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(fed.Domains))
	}
	if fed.Domains[0].Name != "charging" {
		t.Errorf("domain[0].Name: expected %q, got %q", "charging", fed.Domains[0].Name)
	}
	if len(fed.Domains[0].Repos) != 2 {
		t.Fatalf("expected 2 repos in domain charging, got %d", len(fed.Domains[0].Repos))
	}
	if fed.Domains[0].Repos[0].Name != "owner/charging-rating" {
		t.Errorf("repo[0].Name: expected %q, got %q", "owner/charging-rating", fed.Domains[0].Repos[0].Name)
	}
	if fed.Domains[1].Name != "billing" {
		t.Errorf("domain[1].Name: expected %q, got %q", "billing", fed.Domains[1].Name)
	}
}

func TestReadFederation_AllRepos_Flattens(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, validManifest)

	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("ReadFederation: unexpected error: %v", err)
	}
	all := fed.AllRepos()
	if len(all) != 3 {
		t.Fatalf("AllRepos: expected 3 repos, got %d", len(all))
	}
	want := []string{"owner/charging-rating", "owner/charging-balance", "owner/billing-domain"}
	for i, w := range want {
		if all[i].Name != w {
			t.Errorf("AllRepos[%d]: expected %q, got %q (order must be preserved)", i, w, all[i].Name)
		}
	}
}

// AC-3: a domain with exactly one repo is a valid single-repo domain.
func TestReadFederation_SingleRepoDomain_ParsedCorrectly(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `domains:
  - name: platform
    purpose: "The only domain"
    repos:
      - name: myorg/my-service
        purpose: "The only domain repo"
`)

	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("ReadFederation: unexpected error: %v", err)
	}
	if len(fed.Domains) != 1 || len(fed.Domains[0].Repos) != 1 {
		t.Fatalf("expected 1 domain with 1 repo, got %d domain(s)", len(fed.Domains))
	}
	if fed.AllRepos()[0].Name != "myorg/my-service" {
		t.Errorf("expected name %q, got %q", "myorg/my-service", fed.AllRepos()[0].Name)
	}
}

// Hard-cut: the legacy flat `repos:` schema is rejected with migration guidance.
func TestReadFederation_FlatSchema_Rejected(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `repos:
  - name: owner/repo-one
    purpose: "First"
`)

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for the legacy flat schema, got nil")
	}
	if !strings.Contains(err.Error(), "flat `repos:` schema is no longer supported") {
		t.Errorf("error should guide migration to domains, got: %q", err.Error())
	}
}

func TestReadFederation_EmptyFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "   \n  ")

	_, err := ReadFederation(dir)
	if err == nil || !strings.Contains(err.Error(), "file is empty") {
		t.Fatalf("expected 'file is empty' error, got: %v", err)
	}
}

func TestReadFederation_MalformedYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "domains:\n  - name: [unclosed bracket\n")

	_, err := ReadFederation(dir)
	if err == nil || !strings.Contains(err.Error(), "YAML parse error") {
		t.Fatalf("expected 'YAML parse error', got: %v", err)
	}
}

func TestReadFederation_EmptyDomainsList_Valid(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "domains: []\n")

	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("empty domains should be valid (CP with no domains yet), got: %v", err)
	}
	if len(fed.Domains) != 0 {
		t.Errorf("expected 0 domains, got %d", len(fed.Domains))
	}
}

func TestWriteFederation_AllowsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFederation(dir, &Federation{}); err != nil {
		t.Fatalf("expected WriteFederation to accept an empty manifest, got: %v", err)
	}
	fed, err := ReadFederation(dir)
	if err != nil || len(fed.Domains) != 0 {
		t.Fatalf("empty manifest round-trip failed: err=%v domains=%d", err, len(fed.Domains))
	}
}

func TestReadFederation_MissingDomainsKey_Valid(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "something: else\n")

	if _, err := ReadFederation(dir); err != nil {
		t.Fatalf("a manifest with no domains key should be valid (empty), got: %v", err)
	}
}

func TestReadFederation_DomainMissingName_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `domains:
  - purpose: "no name"
    repos:
      - name: owner/repo
        purpose: "p"
`)

	_, err := ReadFederation(dir)
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected domain 'name is required', got: %v", err)
	}
}

func TestReadFederation_DomainBadSlug_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `domains:
  - name: "Not A Slug"
    purpose: "p"
    repos:
      - name: owner/repo
        purpose: "p"
`)

	_, err := ReadFederation(dir)
	if err == nil || !strings.Contains(err.Error(), "lowercase slug") {
		t.Fatalf("expected slug error, got: %v", err)
	}
}

func TestReadFederation_DomainMissingPurpose_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `domains:
  - name: charging
    purpose: "   "
    repos:
      - name: owner/repo
        purpose: "p"
`)

	_, err := ReadFederation(dir)
	if err == nil || !strings.Contains(err.Error(), "purpose is required") {
		t.Fatalf("expected domain 'purpose is required', got: %v", err)
	}
}

func TestReadFederation_DuplicateDomain_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `domains:
  - name: charging
    purpose: "p"
    repos:
      - name: owner/repo-a
        purpose: "p"
  - name: charging
    purpose: "p"
    repos:
      - name: owner/repo-b
        purpose: "p"
`)

	_, err := ReadFederation(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate domain") {
		t.Fatalf("expected 'duplicate domain', got: %v", err)
	}
}

func TestReadFederation_DomainEmptyRepos_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `domains:
  - name: charging
    purpose: "p"
    repos: []
`)

	_, err := ReadFederation(dir)
	if err == nil || !strings.Contains(err.Error(), "repos list is empty") {
		t.Fatalf("expected 'repos list is empty', got: %v", err)
	}
}

// AC-2: a repo with no name yields an error naming the offending domain.
func TestReadFederation_RepoMissingName_ErrorNamesDomain(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `domains:
  - name: charging
    purpose: "p"
    repos:
      - purpose: "no name"
`)

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for missing repo name, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected 'name is required', got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "charging") {
		t.Errorf("expected the error to name the offending domain 'charging', got: %q", err.Error())
	}
}

func TestReadFederation_RepoMissingPurpose_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `domains:
  - name: charging
    purpose: "p"
    repos:
      - name: owner/repo
        purpose: "   "
`)

	_, err := ReadFederation(dir)
	if err == nil || !strings.Contains(err.Error(), "purpose is required") {
		t.Fatalf("expected repo 'purpose is required', got: %v", err)
	}
}

func TestReadFederation_RepoBadNameFormat_ReturnsError(t *testing.T) {
	for _, badName := range []string{"justareponame", "owner/repo/extra", "/repoonly", "owneronly/"} {
		t.Run(badName, func(t *testing.T) {
			dir := t.TempDir()
			writeManifest(t, dir, "domains:\n  - name: charging\n    purpose: \"p\"\n    repos:\n      - name: "+badName+"\n        purpose: \"p\"\n")

			_, err := ReadFederation(dir)
			if err == nil || !strings.Contains(err.Error(), "owner/repo format") {
				t.Fatalf("expected 'owner/repo format' error for %q, got: %v", badName, err)
			}
		})
	}
}

func TestReadFederation_DuplicateRepoAcrossDomains_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, `domains:
  - name: charging
    purpose: "p"
    repos:
      - name: Owner/Shared-Repo
        purpose: "p"
  - name: billing
    purpose: "p"
    repos:
      - name: owner/shared-repo
        purpose: "p"
`)

	_, err := ReadFederation(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate repo") {
		t.Fatalf("expected case-insensitive 'duplicate repo' across domains, got: %v", err)
	}
}

func TestReadFederation_FileNotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadFederation(dir); err == nil {
		t.Fatal("expected an error when FEDERATION.md is absent, got nil")
	}
}

func TestWriteFederation_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, validManifest)
	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("ReadFederation: %v", err)
	}

	created, err := fed.AddRepo("billing", "Invoice generation", "owner/billing-statements", "Statements")
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	if created {
		t.Error("expected createdDomain=false for an existing domain")
	}
	if err := WriteFederation(dir, fed); err != nil {
		t.Fatalf("WriteFederation: %v", err)
	}

	reread, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("re-ReadFederation: %v", err)
	}
	if len(reread.AllRepos()) != 4 {
		t.Fatalf("expected 4 repos after round-trip, got %d", len(reread.AllRepos()))
	}
	var found bool
	for _, r := range reread.AllRepos() {
		if r.Name == "owner/billing-statements" {
			found = true
		}
	}
	if !found {
		t.Error("the added repo did not survive the write/read round-trip")
	}
}

func TestWriteFederation_RejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	fed := &Federation{Domains: []FederationDomain{{Name: "charging", Purpose: "p", Repos: nil}}}
	if err := WriteFederation(dir, fed); err == nil {
		t.Fatal("expected WriteFederation to reject an invalid manifest")
	}
	if IsFederationRepo(dir) {
		t.Error("an invalid manifest must not be written to disk")
	}
}

func TestFederation_AddRepo_LazyCreatesDomain(t *testing.T) {
	fed := &Federation{}
	created, err := fed.AddRepo("billing", "Invoicing", "acme/billing", "Bill runs")
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	if !created {
		t.Error("expected createdDomain=true for a new domain")
	}
	if !fed.HasDomain("Billing") {
		t.Error("HasDomain should match case-insensitively")
	}
	if len(fed.Domains) != 1 || len(fed.Domains[0].Repos) != 1 {
		t.Fatalf("expected 1 domain with 1 repo, got %d domains", len(fed.Domains))
	}
}

func TestFederation_AddRepo_RejectsDuplicate(t *testing.T) {
	fed := &Federation{}
	if _, err := fed.AddRepo("charging", "C", "acme/svc", "p"); err != nil {
		t.Fatalf("first AddRepo: %v", err)
	}
	if _, err := fed.AddRepo("billing", "B", "Acme/SVC", "p"); err == nil {
		t.Fatal("expected a duplicate-repo error (case-insensitive) across domains")
	}
}

// --- Backward compatibility: legacy FEDERATION.md (v3.0.1 rename) ---

func TestIsFederationRepo_LegacyMdOnly_ReturnsTrue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, legacyFederationFileName), []byte(validManifest), 0644); err != nil {
		t.Fatalf("writing legacy manifest: %v", err)
	}
	if !IsFederationRepo(dir) {
		t.Error("IsFederationRepo: expected true when only the legacy FEDERATION.md exists")
	}
}

func TestReadFederation_LegacyMdOnly_Parsed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, legacyFederationFileName), []byte(validManifest), 0644); err != nil {
		t.Fatalf("writing legacy manifest: %v", err)
	}
	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("ReadFederation (legacy .md): %v", err)
	}
	if len(fed.Domains) != 2 {
		t.Errorf("expected 2 domains from legacy manifest, got %d", len(fed.Domains))
	}
}

func TestReadFederation_PrefersYamlOverLegacyMd(t *testing.T) {
	dir := t.TempDir()
	// Legacy .md has one domain; canonical .yaml has the two-domain validManifest.
	legacy := "domains:\n  - name: legacy\n    purpose: old\n    repos:\n      - name: o/r\n        purpose: p\n"
	if err := os.WriteFile(filepath.Join(dir, legacyFederationFileName), []byte(legacy), 0644); err != nil {
		t.Fatalf("writing legacy manifest: %v", err)
	}
	writeManifest(t, dir, validManifest) // writes FEDERATION.yaml
	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("ReadFederation: %v", err)
	}
	if len(fed.Domains) != 2 || fed.Domains[0].Name == "legacy" {
		t.Errorf("expected canonical FEDERATION.yaml to win, got domains %+v", fed.Domains)
	}
}

func TestWriteFederation_WritesYamlAndMigratesLegacyMd(t *testing.T) {
	dir := t.TempDir()
	// Start from a legacy-only manifest.
	if err := os.WriteFile(filepath.Join(dir, legacyFederationFileName), []byte(validManifest), 0644); err != nil {
		t.Fatalf("writing legacy manifest: %v", err)
	}
	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("ReadFederation: %v", err)
	}
	if err := WriteFederation(dir, fed); err != nil {
		t.Fatalf("WriteFederation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, federationFileName)); err != nil {
		t.Errorf("expected FEDERATION.yaml to be written, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, legacyFederationFileName)); !os.IsNotExist(err) {
		t.Errorf("expected legacy FEDERATION.md to be removed after write, stat err: %v", err)
	}
}
