package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeRun returns a RunCommandFunc that records calls and returns the given output.
func fakeRun(output string) RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		return output, nil
	}
}

// fakeRunErr returns a RunCommandFunc that returns an error.
func fakeRunErr(errMsg string) RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		return "", fmt.Errorf("%s", errMsg)
	}
}

// fakeCredentials returns credential bytes with the given expiry.
func fakeCredentials(expiresAt string) []byte {
	cred := map[string]string{"expiresAt": expiresAt}
	data, _ := json.Marshal(cred)
	return data
}

// fakeFutureCredentials returns credentials that expire in the future.
func fakeFutureCredentials() []byte {
	future := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	return fakeCredentials(future)
}

// fakeExpiredCredentials returns credentials that have already expired.
func fakeExpiredCredentials() []byte {
	past := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	return fakeCredentials(past)
}

func TestLogin_Success(t *testing.T) {
	var buf bytes.Buffer

	refreshCalled := false
	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return fakeFutureCredentials(), nil
		},
		ClaudeRefresh: func() error {
			refreshCalled = true
			return nil
		},
		RepoFullName: "owner/repo",
		Owner:        "owner",
		OwnerType:    "user",
	}

	err := Login(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !refreshCalled {
		t.Error("expected ClaudeRefresh to be called")
	}

	output := buf.String()
	if !strings.Contains(output, "Login successful") {
		t.Errorf("expected login success message, got:\n%s", output)
	}
	if !strings.Contains(output, "CLAUDE_CREDENTIALS_JSON updated") {
		t.Errorf("expected credentials updated message, got:\n%s", output)
	}
}

func TestLogin_RefreshError(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return fakeFutureCredentials(), nil
		},
		ClaudeRefresh: func() error {
			return fmt.Errorf("login failed")
		},
		RepoFullName: "owner/repo",
	}

	err := Login(&buf, deps)
	if err == nil {
		t.Fatal("expected error when refresh fails")
	}
	if !strings.Contains(err.Error(), "claude login failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRefresh_Success(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return fakeFutureCredentials(), nil
		},
		RepoFullName: "owner/repo",
		Owner:        "owner",
		OwnerType:    "user",
	}

	err := Refresh(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "CLAUDE_CREDENTIALS_JSON updated") {
		t.Errorf("expected credentials updated message, got:\n%s", output)
	}
}

func TestRefresh_NoCredentials(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return nil, fmt.Errorf("not found")
		},
		RepoFullName: "owner/repo",
	}

	err := Refresh(&buf, deps)
	if err == nil {
		t.Fatal("expected error when credentials missing")
	}
	if !strings.Contains(err.Error(), "credentials not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRefresh_UploadError(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRunErr("API error"),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return fakeFutureCredentials(), nil
		},
		RepoFullName: "owner/repo",
		Owner:        "owner",
		OwnerType:    "user",
	}

	err := Refresh(&buf, deps)
	if err == nil {
		t.Fatal("expected error on upload failure")
	}
	if !strings.Contains(err.Error(), "failed to set secret") {
		t.Errorf("unexpected error: %v", err)
	}
}

func fakeCheckRepoSecret(exists bool) CheckRepoSecretFunc {
	return func(owner, repo, name string) (bool, error) {
		return exists, nil
	}
}

func TestCheck_InSync(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return fakeFutureCredentials(), nil
		},
		CheckRepoSecret: fakeCheckRepoSecret(true),
		Owner:           "owner",
		RepoName:        "repo",
	}

	result, err := Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid local credentials")
	}
	if !result.RepoSecretSet {
		t.Error("expected repo secret to be set")
	}
	if !result.InSync {
		t.Error("expected in-sync result")
	}

	output := buf.String()
	if !strings.Contains(output, "in sync") {
		t.Errorf("expected 'in sync' in output, got:\n%s", output)
	}
}

func TestCheck_LocalValidSecretMissing(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return fakeFutureCredentials(), nil
		},
		CheckRepoSecret: fakeCheckRepoSecret(false),
		Owner:           "owner",
		RepoName:        "repo",
	}

	result, err := Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid local credentials")
	}
	if result.InSync {
		t.Error("expected out-of-sync when secret missing")
	}

	output := buf.String()
	if !strings.Contains(output, "auth refresh") {
		t.Errorf("expected 'auth refresh' remediation, got:\n%s", output)
	}
}

func TestCheck_ExpiredCredentials(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return fakeExpiredCredentials(), nil
		},
		CheckRepoSecret: fakeCheckRepoSecret(true),
		Owner:           "owner",
		RepoName:        "repo",
	}

	result, err := Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for expired credentials")
	}
	if result.InSync {
		t.Error("expected out-of-sync for expired credentials")
	}

	output := buf.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected fail mark for expired creds, got:\n%s", output)
	}
	if !strings.Contains(output, "auth login") {
		t.Errorf("expected login remediation, got:\n%s", output)
	}
}

func TestCheck_MissingCredentials(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return nil, fmt.Errorf("not found")
		},
		CheckRepoSecret: fakeCheckRepoSecret(false),
		Owner:           "owner",
		RepoName:        "repo",
	}

	result, err := Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for missing credentials")
	}
	if result.InSync {
		t.Error("expected out-of-sync when nothing is configured")
	}

	output := buf.String()
	if !strings.Contains(output, "auth login") {
		t.Errorf("expected login remediation, got:\n%s", output)
	}
}

func TestCheck_UnparseableExpiry(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return []byte(`{"token": "abc123"}`), nil // No expiry field.
		},
		CheckRepoSecret: fakeCheckRepoSecret(true),
		Owner:           "owner",
		RepoName:        "repo",
	}

	result, err := Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("credentials with unknown expiry should be treated as valid")
	}

	output := buf.String()
	if !strings.Contains(output, "expiry unknown") {
		t.Errorf("expected 'expiry unknown' message, got:\n%s", output)
	}
}

func TestParseCredentialExpiry_RFC3339(t *testing.T) {
	data := fakeCredentials("2026-05-15T10:00:00Z")
	expiry, err := parseCredentialExpiry(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expiry.Year() != 2026 || expiry.Month() != 5 || expiry.Day() != 15 {
		t.Errorf("unexpected expiry: %v", expiry)
	}
}

func TestParseCredentialExpiry_NoField(t *testing.T) {
	data := []byte(`{"token": "abc"}`)
	_, err := parseCredentialExpiry(data)
	if err == nil {
		t.Fatal("expected error when no expiry field")
	}
}

func TestParseCredentialExpiry_InvalidJSON(t *testing.T) {
	_, err := parseCredentialExpiry([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseCredentialExpiry_AlternateField(t *testing.T) {
	data := []byte(`{"expiry": "2026-06-01T00:00:00Z"}`)
	expiry, err := parseCredentialExpiry(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expiry.Month() != 6 {
		t.Errorf("expected June, got: %v", expiry.Month())
	}
}

// TestUploadCredentials_OrgOwnerGetsRepoScope (v3.0.2): organisation-owned repos
// now write the credential at --repo level, not --org. The control plane runs
// the pipeline and reads CLAUDE_CREDENTIALS_JSON at the repo level; an org secret
// did not reliably resolve for the CP's own workflow.
func TestUploadCredentials_OrgOwnerGetsRepoScope(t *testing.T) {
	var buf bytes.Buffer
	var capturedArgs []string

	deps := Deps{
		Run: func(name string, args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		},
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return []byte("creds"), nil
		},
		RepoFullName: "myorg/repo",
		Owner:        "myorg",
		OwnerType:    "Organization",
	}

	err := uploadCredentials(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, arg := range capturedArgs {
		if arg == "--org" {
			t.Errorf("org owner must NOT use --org (v3.0.2 writes repo-level): %v", capturedArgs)
		}
	}
	found := false
	for i := 0; i < len(capturedArgs)-1; i++ {
		if capturedArgs[i] == "--repo" && capturedArgs[i+1] == "myorg/repo" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected `--repo myorg/repo` in args: %v", capturedArgs)
	}
}

func TestUploadCredentials_RepoScope(t *testing.T) {
	var buf bytes.Buffer
	var capturedArgs []string

	deps := Deps{
		Run: func(name string, args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		},
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return []byte("creds"), nil
		},
		RepoFullName: "user/repo",
		Owner:        "user",
		OwnerType:    "user",
	}

	err := uploadCredentials(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use --repo flag.
	found := false
	for _, arg := range capturedArgs {
		if arg == "--repo" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --repo flag in args: %v", capturedArgs)
	}
}

// TestUploadCredentials_ScopeForRoute_OrgOwner (v3.0.2): an organisation owner
// now gets `--repo <repo>` with the correct target — never `--org`.
func TestUploadCredentials_ScopeForRoute_OrgOwner(t *testing.T) {
	var buf bytes.Buffer
	var capturedArgs []string

	deps := Deps{
		Run: func(name string, args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		},
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return []byte("creds"), nil
		},
		RepoFullName: "acme/cp",
		Owner:        "acme",
		OwnerType:    OwnerTypeOrg,
	}

	if err := uploadCredentials(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect: gh secret set CLAUDE_CREDENTIALS_JSON --body <b64> --repo acme/cp
	for _, arg := range capturedArgs {
		if arg == "--org" {
			t.Fatalf("org owner must NOT use --org (v3.0.2 writes repo-level): %v", capturedArgs)
		}
	}
	found := false
	for i := 0; i < len(capturedArgs)-1; i++ {
		if capturedArgs[i] == "--repo" {
			if capturedArgs[i+1] != "acme/cp" {
				t.Fatalf("--repo target: got %q, want %q (args: %v)", capturedArgs[i+1], "acme/cp", capturedArgs)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected --repo flag in args: %v", capturedArgs)
	}
}

// TestUploadCredentials_ScopeForRoute_UserOwner asserts the user-account
// case stays at --repo with the correct target.
func TestUploadCredentials_ScopeForRoute_UserOwner(t *testing.T) {
	var buf bytes.Buffer
	var capturedArgs []string

	deps := Deps{
		Run: func(name string, args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		},
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return []byte("creds"), nil
		},
		RepoFullName: "eddie/repo",
		Owner:        "eddie",
		OwnerType:    OwnerTypeUser,
	}

	if err := uploadCredentials(&buf, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for i := 0; i < len(capturedArgs)-1; i++ {
		if capturedArgs[i] == "--repo" {
			if capturedArgs[i+1] != "eddie/repo" {
				t.Fatalf("--repo target: got %q, want %q (args: %v)", capturedArgs[i+1], "eddie/repo", capturedArgs)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected --repo flag in args: %v", capturedArgs)
	}
}
