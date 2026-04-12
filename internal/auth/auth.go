// Package auth implements v2 credential management for Claude Code.
// It provides login, refresh, and check operations for the
// CLAUDE_CREDENTIALS_JSON secret.
package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
)

// RunCommandFunc is a function type for running shell commands.
type RunCommandFunc = bootstrap.RunCommandFunc

// ReadCredentialsFunc reads Claude credentials from the local store.
type ReadCredentialsFunc func(run RunCommandFunc) ([]byte, error)

// ClaudeRefreshFunc triggers a Claude Code login/refresh.
type ClaudeRefreshFunc func() error

// Deps holds injectable dependencies for auth operations.
type Deps struct {
	Run             RunCommandFunc
	ReadCredentials ReadCredentialsFunc
	ClaudeRefresh   ClaudeRefreshFunc
	RepoFullName    string
	Owner           string
	OwnerType       string
}

// Login forces a Claude Code login, reads the refreshed credentials, and
// uploads them to the repo as CLAUDE_CREDENTIALS_JSON.
func Login(w io.Writer, deps Deps) error {
	fmt.Fprintln(w, "  Logging in to Claude Code...")

	// Step 1: Force login/refresh.
	if deps.ClaudeRefresh != nil {
		if err := deps.ClaudeRefresh(); err != nil {
			return fmt.Errorf("claude login failed: %w", err)
		}
	}
	fmt.Fprintln(w, "  ✓ Login successful")

	// Step 2: Read and upload credentials.
	if err := uploadCredentials(w, deps); err != nil {
		return err
	}

	return nil
}

// Refresh reads the current local credentials and uploads them to the
// repo secret without triggering a login.
func Refresh(w io.Writer, deps Deps) error {
	if err := uploadCredentials(w, deps); err != nil {
		return err
	}
	return nil
}

// CheckResult represents the result of a credential check.
type CheckResult struct {
	Valid     bool
	ExpiresIn time.Duration
	ExpiresAt time.Time
	Message   string
}

// Check verifies that the CLAUDE_CREDENTIALS_JSON is present and not expired.
// Returns a CheckResult with validity status and expiry information.
func Check(w io.Writer, deps Deps) (CheckResult, error) {
	data, err := deps.ReadCredentials(deps.Run)
	if err != nil {
		result := CheckResult{
			Valid:   false,
			Message: "credentials not found — run 'gh agentic -v2 auth login'",
		}
		fmt.Fprintf(w, "  ✗ CLAUDE_CREDENTIALS_JSON — not found\n")
		fmt.Fprintf(w, "    → Run 'gh agentic -v2 auth login'\n")
		return result, nil
	}

	// Try to parse expiry from credentials JSON.
	expiry, parseErr := parseCredentialExpiry(data)
	if parseErr != nil {
		// Can't determine expiry — treat as valid but warn.
		result := CheckResult{
			Valid:   true,
			Message: "credentials present (expiry unknown)",
		}
		fmt.Fprintln(w, "  ✓ CLAUDE_CREDENTIALS_JSON — present (expiry unknown)")
		return result, nil
	}

	now := time.Now()
	remaining := expiry.Sub(now)

	if remaining <= 0 {
		result := CheckResult{
			Valid:     false,
			ExpiresAt: expiry,
			Message:   "credentials expired",
		}
		fmt.Fprintln(w, "  ✗ CLAUDE_CREDENTIALS_JSON — expired")
		fmt.Fprintln(w, "    → Run 'gh agentic -v2 auth refresh'")
		return result, nil
	}

	days := int(remaining.Hours() / 24)
	result := CheckResult{
		Valid:     true,
		ExpiresIn: remaining,
		ExpiresAt: expiry,
		Message:   fmt.Sprintf("valid (expires in %d days)", days),
	}
	fmt.Fprintf(w, "  ✓ CLAUDE_CREDENTIALS_JSON — valid (expires in %d days)\n", days)
	return result, nil
}

// uploadCredentials reads local credentials and uploads them to GitHub.
func uploadCredentials(w io.Writer, deps Deps) error {
	data, err := deps.ReadCredentials(deps.Run)
	if err != nil {
		return fmt.Errorf("credentials not found: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	var ghArgs []string
	if deps.OwnerType == bootstrap.OwnerTypeOrg {
		ghArgs = []string{"secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", encoded, "--org", deps.Owner}
	} else {
		ghArgs = []string{"secret", "set", "CLAUDE_CREDENTIALS_JSON", "--body", encoded, "--repo", deps.RepoFullName}
	}

	if _, setErr := deps.Run("gh", ghArgs...); setErr != nil {
		return fmt.Errorf("failed to set secret: %w", setErr)
	}

	fmt.Fprintf(w, "  ✓ CLAUDE_CREDENTIALS_JSON updated in %s\n", deps.RepoFullName)
	return nil
}

// credentialData represents the structure of Claude credentials JSON
// for extracting expiry information.
type credentialData struct {
	// OAuth tokens often have an expires_at or expiry field.
	ExpiresAt string `json:"expiresAt"`
	Expiry    string `json:"expiry"`
}

// parseCredentialExpiry attempts to extract an expiry timestamp from
// credential JSON. Returns the expiry time or an error if not parseable.
func parseCredentialExpiry(data []byte) (time.Time, error) {
	var cred credentialData
	if err := json.Unmarshal(data, &cred); err != nil {
		return time.Time{}, fmt.Errorf("parsing credentials: %w", err)
	}

	// Try different field names.
	expiryStr := cred.ExpiresAt
	if expiryStr == "" {
		expiryStr = cred.Expiry
	}
	if expiryStr == "" {
		return time.Time{}, fmt.Errorf("no expiry field found")
	}

	// Try common time formats.
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, expiryStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognised time format: %s", expiryStr)
}
