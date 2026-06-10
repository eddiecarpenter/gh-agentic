package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsFederationRepo_FilePresent_ReturnsTrue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte("repos:\n  - name: owner/repo\n    purpose: test\n"), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}
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
	content := `repos:
  - name: owner/repo-one
    purpose: "First domain area"
  - name: owner/repo-two
    purpose: "Second domain area"
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("ReadFederation: unexpected error: %v", err)
	}
	if len(fed.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(fed.Repos))
	}
	if fed.Repos[0].Name != "owner/repo-one" {
		t.Errorf("repo[0].Name: expected %q, got %q", "owner/repo-one", fed.Repos[0].Name)
	}
	if fed.Repos[0].Purpose != "First domain area" {
		t.Errorf("repo[0].Purpose: expected %q, got %q", "First domain area", fed.Repos[0].Purpose)
	}
	if fed.Repos[1].Name != "owner/repo-two" {
		t.Errorf("repo[1].Name: expected %q, got %q", "owner/repo-two", fed.Repos[1].Name)
	}
}

func TestReadFederation_ValidManifestSingleRepo_ParsedCorrectly(t *testing.T) {
	dir := t.TempDir()
	content := `repos:
  - name: myorg/my-service
    purpose: "The only domain repo"
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	fed, err := ReadFederation(dir)
	if err != nil {
		t.Fatalf("ReadFederation: unexpected error: %v", err)
	}
	if len(fed.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(fed.Repos))
	}
	if fed.Repos[0].Name != "myorg/my-service" {
		t.Errorf("expected name %q, got %q", "myorg/my-service", fed.Repos[0].Name)
	}
}

func TestReadFederation_EmptyFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte("   \n  "), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for empty file, got nil")
	}
	if !strings.Contains(err.Error(), "file is empty") {
		t.Errorf("error message should contain 'file is empty', got: %q", err.Error())
	}
}

func TestReadFederation_MalformedYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	// Indentation error — valid YAML tokens but unexpected structure that
	// produces a YAML parse error.
	content := `repos:
  - name: [unclosed bracket
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for malformed YAML, got nil")
	}
	if !strings.Contains(err.Error(), "YAML parse error") {
		t.Errorf("error message should contain 'YAML parse error', got: %q", err.Error())
	}
}

func TestReadFederation_EmptyReposList_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	content := `repos: []
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for empty repos list, got nil")
	}
	if !strings.Contains(err.Error(), "repos list is empty") {
		t.Errorf("error message should contain 'repos list is empty', got: %q", err.Error())
	}
}

func TestReadFederation_MissingReposKey_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	content := `something: else
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error when repos key is absent, got nil")
	}
	if !strings.Contains(err.Error(), "repos list is empty") {
		t.Errorf("error message should contain 'repos list is empty', got: %q", err.Error())
	}
}

func TestReadFederation_MissingName_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	content := `repos:
  - purpose: "Some purpose"
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for missing name, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error message should contain 'name is required', got: %q", err.Error())
	}
}

func TestReadFederation_MissingPurpose_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	content := `repos:
  - name: owner/repo-name
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for missing purpose, got nil")
	}
	if !strings.Contains(err.Error(), "purpose is required") {
		t.Errorf("error message should contain 'purpose is required', got: %q", err.Error())
	}
}

func TestReadFederation_BlankPurpose_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	content := `repos:
  - name: owner/repo-name
    purpose: "   "
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for blank purpose, got nil")
	}
	if !strings.Contains(err.Error(), "purpose is required") {
		t.Errorf("error message should contain 'purpose is required', got: %q", err.Error())
	}
}

func TestReadFederation_BadNameFormat_ReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		badName string
	}{
		{name: "no slash", badName: "justareponame"},
		{name: "double slash", badName: "owner/repo/extra"},
		{name: "empty owner", badName: "/repoonly"},
		{name: "empty repo", badName: "owneronly/"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			content := "repos:\n  - name: " + tc.badName + "\n    purpose: \"A purpose\"\n"
			if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
				t.Fatalf("writing FEDERATION.md: %v", err)
			}

			_, err := ReadFederation(dir)
			if err == nil {
				t.Fatalf("expected error for name %q, got nil", tc.badName)
			}
			if !strings.Contains(err.Error(), "owner/repo format") {
				t.Errorf("error should mention 'owner/repo format', got: %q", err.Error())
			}
		})
	}
}

func TestReadFederation_DuplicateName_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	content := `repos:
  - name: owner/repo-name
    purpose: "First"
  - name: owner/repo-name
    purpose: "Second"
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for duplicate repo name, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate repo") {
		t.Errorf("error message should contain 'duplicate repo', got: %q", err.Error())
	}
}

func TestReadFederation_DuplicateNameCaseInsensitive_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	content := `repos:
  - name: Owner/Repo-Name
    purpose: "First"
  - name: owner/repo-name
    purpose: "Second"
`
	if err := os.WriteFile(filepath.Join(dir, federationFileName), []byte(content), 0644); err != nil {
		t.Fatalf("writing FEDERATION.md: %v", err)
	}

	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error for case-insensitive duplicate repo name, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate repo") {
		t.Errorf("error message should contain 'duplicate repo', got: %q", err.Error())
	}
}

func TestReadFederation_FileNotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	// No file written — ReadFederation should report the OS error.
	_, err := ReadFederation(dir)
	if err == nil {
		t.Fatal("expected an error when FEDERATION.md is absent, got nil")
	}
}
