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

func TestCheck_ValidCredentials(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return fakeFutureCredentials(), nil
		},
	}

	result, err := Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid result")
	}
	if result.ExpiresIn <= 0 {
		t.Error("expected positive expires-in duration")
	}

	output := buf.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected check mark for valid creds, got:\n%s", output)
	}
}

func TestCheck_ExpiredCredentials(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return fakeExpiredCredentials(), nil
		},
	}

	result, err := Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for expired credentials")
	}

	output := buf.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected fail mark for expired creds, got:\n%s", output)
	}
	if !strings.Contains(output, "auth refresh") {
		t.Errorf("expected remediation command, got:\n%s", output)
	}
}

func TestCheck_MissingCredentials(t *testing.T) {
	var buf bytes.Buffer

	deps := Deps{
		Run: fakeRun(""),
		ReadCredentials: func(run RunCommandFunc) ([]byte, error) {
			return nil, fmt.Errorf("not found")
		},
	}

	result, err := Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid result for missing credentials")
	}

	output := buf.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected fail mark, got:\n%s", output)
	}
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

func TestUploadCredentials_OrgScope(t *testing.T) {
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

	// Should use --org flag.
	found := false
	for _, arg := range capturedArgs {
		if arg == "--org" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --org flag in args: %v", capturedArgs)
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
