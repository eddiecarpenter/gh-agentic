package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/eddiecarpenter/gh-agentic/internal/auth"
)

func fakeValidCredentials() []byte {
	future := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	data, _ := json.Marshal(map[string]string{"expiresAt": future})
	return data
}

func fakeExpiredCreds() []byte {
	past := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	data, _ := json.Marshal(map[string]string{"expiresAt": past})
	return data
}

func TestAuthCmd_WithoutV2Flag(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"auth"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error without -v2 flag")
	}
	if !strings.Contains(err.Error(), "requires the --v2 flag") {
		t.Errorf("expected v2 flag error, got: %v", err)
	}
}

func TestAuthCmd_SubcommandsRegistered(t *testing.T) {
	deps := authDeps{
		run: func(name string, args ...string) (string, error) { return "", nil },
		readCredentials: func(run auth.RunCommandFunc) ([]byte, error) {
			return fakeValidCredentials(), nil
		},
	}

	cmd := newAuthCmdWithDeps(deps)
	subcommands := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subcommands[c.Use] = true
	}

	expected := []string{"login", "refresh", "check"}
	for _, name := range expected {
		if !subcommands[name] {
			t.Errorf("expected %q subcommand to be registered", name)
		}
	}
}

func TestAuthCheck_ReturnsErrorOnExpired(t *testing.T) {
	// This tests the auth.Check function directly with expired credentials.
	var buf bytes.Buffer

	deps := auth.Deps{
		Run: func(name string, args ...string) (string, error) { return "", nil },
		ReadCredentials: func(run auth.RunCommandFunc) ([]byte, error) {
			return fakeExpiredCreds(), nil
		},
	}

	result, err := auth.Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid result for expired credentials")
	}
}

func TestAuthCheck_ReturnsValidForFuture(t *testing.T) {
	var buf bytes.Buffer

	deps := auth.Deps{
		Run: func(name string, args ...string) (string, error) { return "", nil },
		ReadCredentials: func(run auth.RunCommandFunc) ([]byte, error) {
			return fakeValidCredentials(), nil
		},
	}

	result, err := auth.Check(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid result for future credentials")
	}
}

func TestAuthLogin_CallsRefresh(t *testing.T) {
	var buf bytes.Buffer
	refreshCalled := false

	deps := auth.Deps{
		Run: func(name string, args ...string) (string, error) { return "", nil },
		ReadCredentials: func(run auth.RunCommandFunc) ([]byte, error) {
			return fakeValidCredentials(), nil
		},
		ClaudeRefresh: func() error {
			refreshCalled = true
			return nil
		},
		RepoFullName: "owner/repo",
		Owner:        "owner",
		OwnerType:    "user",
	}

	err := auth.Login(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !refreshCalled {
		t.Error("expected ClaudeRefresh to be called")
	}
}

func TestAuthRefresh_UploadsCredentials(t *testing.T) {
	var buf bytes.Buffer
	var secretSet bool

	deps := auth.Deps{
		Run: func(name string, args ...string) (string, error) {
			if name == "gh" {
				secretSet = true
			}
			return "", nil
		},
		ReadCredentials: func(run auth.RunCommandFunc) ([]byte, error) {
			return fakeValidCredentials(), nil
		},
		RepoFullName: "owner/repo",
		Owner:        "owner",
		OwnerType:    "user",
	}

	err := auth.Refresh(&buf, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !secretSet {
		t.Error("expected gh secret set to be called")
	}
}

func TestAuthRefresh_NoCredentials(t *testing.T) {
	var buf bytes.Buffer

	deps := auth.Deps{
		Run: func(name string, args ...string) (string, error) { return "", nil },
		ReadCredentials: func(run auth.RunCommandFunc) ([]byte, error) {
			return nil, fmt.Errorf("not found")
		},
		RepoFullName: "owner/repo",
	}

	err := auth.Refresh(&buf, deps)
	if err == nil {
		t.Fatal("expected error when credentials missing")
	}
}
