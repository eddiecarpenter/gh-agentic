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

)

// RunCommandFunc is a function type for running shell commands.
// Defined in shell.go.

// ReadCredentialsFunc reads Claude credentials from the local store.
type ReadCredentialsFunc func(run RunCommandFunc) ([]byte, error)

// ClaudeRefreshFunc triggers a Claude Code login/refresh.
type ClaudeRefreshFunc func() error

// CheckRepoSecretFunc checks whether a named secret exists in a GitHub repo.
// Returns true if the secret is set, false if not found.
type CheckRepoSecretFunc func(owner, repo, secretName string) (bool, error)

// Deps holds injectable dependencies for auth operations.
type Deps struct {
	Run              RunCommandFunc
	ReadCredentials  ReadCredentialsFunc
	ClaudeRefresh    ClaudeRefreshFunc
	CheckRepoSecret  CheckRepoSecretFunc
	RepoFullName     string
	Owner            string
	RepoName         string
	OwnerType        string
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
	Valid         bool
	ExpiresIn     time.Duration
	ExpiresAt     time.Time
	Message       string
	RepoSecretSet bool
	InSync        bool
}

const secretName = "CLAUDE_CREDENTIALS_JSON"

// Check verifies local Claude credentials and the repo secret, then compares them.
func Check(w io.Writer, deps Deps) (CheckResult, error) {
	// --- Local credentials ---
	localValid := false
	var localExpiry time.Time
	var localExpiresIn time.Duration

	data, localErr := deps.ReadCredentials(deps.Run)
	if localErr != nil {
		fmt.Fprintf(w, "  ✗  Local credentials — not found\n")
		fmt.Fprintf(w, "       → Run 'gh agentic auth login'\n")
	} else {
		expiry, parseErr := parseCredentialExpiry(data)
		if parseErr != nil {
			localValid = true
			fmt.Fprintf(w, "  ✓  Local credentials — present (expiry unknown)\n")
		} else {
			remaining := time.Until(expiry)
			if remaining <= 0 {
				fmt.Fprintf(w, "  ✗  Local credentials — expired\n")
				fmt.Fprintf(w, "       → Run 'gh agentic auth login'\n")
			} else {
				localValid = true
				localExpiry = expiry
				localExpiresIn = remaining
				days := int(remaining.Hours() / 24)
				fmt.Fprintf(w, "  ✓  Local credentials — valid (expires in %d days)\n", days)
			}
		}
	}

	// --- Repo secret ---
	secretSet := false
	if deps.CheckRepoSecret != nil {
		set, err := deps.CheckRepoSecret(deps.Owner, deps.RepoName, secretName)
		if err == nil {
			secretSet = set
		}
	}
	if secretSet {
		fmt.Fprintf(w, "  ✓  %s — set in repo\n", secretName)
	} else {
		fmt.Fprintf(w, "  ✗  %s — not set in repo\n", secretName)
	}

	// --- Comparison ---
	inSync := localValid && secretSet
	fmt.Fprintln(w, "")
	switch {
	case localValid && secretSet:
		fmt.Fprintf(w, "  ✓  Credentials in sync\n")
	case localValid && !secretSet:
		fmt.Fprintf(w, "  ✗  Local credentials not uploaded — run 'gh agentic auth refresh'\n")
	case !localValid && secretSet:
		fmt.Fprintf(w, "  ✗  Local credentials missing or expired — run 'gh agentic auth login'\n")
	default:
		fmt.Fprintf(w, "  ✗  No credentials configured — run 'gh agentic auth login'\n")
	}

	return CheckResult{
		Valid:         localValid,
		ExpiresIn:     localExpiresIn,
		ExpiresAt:     localExpiry,
		RepoSecretSet: secretSet,
		InSync:        inSync,
		Message:       fmt.Sprintf("local=%v repo=%v", localValid, secretSet),
	}, nil
}

// uploadCredentials reads local credentials and uploads them to GitHub.
func uploadCredentials(w io.Writer, deps Deps) error {
	data, err := deps.ReadCredentials(deps.Run)
	if err != nil {
		return fmt.Errorf("credentials not found: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	var ghArgs []string
	if deps.OwnerType == OwnerTypeOrg {
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
