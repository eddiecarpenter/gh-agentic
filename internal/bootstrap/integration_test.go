package bootstrap

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// integrationLookPath returns a LookPathFunc that succeeds for tools in the found
// set and fails for all others.
func integrationLookPath(found map[string]bool) LookPathFunc {
	return func(file string) (string, error) {
		if found[file] {
			return "/usr/bin/" + file, nil
		}
		return "", fmt.Errorf("%s not found", file)
	}
}

// integrationConfirm returns a ConfirmFunc that always returns the given value.
func integrationConfirm(yes bool) ConfirmFunc {
	return func(_ string) (bool, error) {
		return yes, nil
	}
}

func TestIntegrationPreflight_AllToolsPresent_Passes(t *testing.T) {
	lookPath := integrationLookPath(map[string]bool{
		"git":    true,
		"gh":     true,
		"goose":  true,
		"claude": true,
	})
	run := func(name string, args ...string) (string, error) {
		// gh auth status succeeds.
		if name == "gh" && len(args) > 0 && args[0] == "auth" {
			return "Logged in", nil
		}
		return "", nil
	}

	var buf bytes.Buffer
	err := RunPreflight(&buf, lookPath, run, integrationConfirm(false))

	if err != nil {
		t.Fatalf("expected nil error, got: %v\nOutput:\n%s", err, buf.String())
	}

	output := buf.String()
	for _, tool := range []string{"git", "gh", "goose", "claude"} {
		if !strings.Contains(output, tool+" found") {
			t.Errorf("expected '%s found' in output, got:\n%s", tool, output)
		}
	}
}

func TestIntegrationPreflight_RequiredMissing_Git_HardStop(t *testing.T) {
	lookPath := integrationLookPath(map[string]bool{
		"gh":     true,
		"goose":  true,
		"claude": true,
		// git is missing
	})
	run := func(name string, args ...string) (string, error) {
		return "", nil
	}

	var buf bytes.Buffer
	err := RunPreflight(&buf, lookPath, run, integrationConfirm(false))

	if err == nil {
		t.Fatal("expected error for missing git, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "git") {
		t.Errorf("expected 'git' mentioned in output, got:\n%s", output)
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("expected error to mention git, got: %v", err)
	}
}

func TestIntegrationPreflight_RequiredMissing_Gh_HardStop(t *testing.T) {
	lookPath := integrationLookPath(map[string]bool{
		"git":    true,
		"goose":  true,
		"claude": true,
		// gh is missing
	})
	run := func(name string, args ...string) (string, error) {
		return "", nil
	}

	var buf bytes.Buffer
	err := RunPreflight(&buf, lookPath, run, integrationConfirm(false))

	if err == nil {
		t.Fatal("expected error for missing gh, got nil")
	}

	output := buf.String()
	if !strings.Contains(output, "gh") {
		t.Errorf("expected 'gh' mentioned in output, got:\n%s", output)
	}
	if !strings.Contains(err.Error(), "gh") {
		t.Errorf("expected error to mention gh, got: %v", err)
	}
}

func TestIntegrationPreflight_OptionalMissing_Goose_OfferInstall(t *testing.T) {
	// goose is required but has an installPrompt, so it gets an install offer.
	installAttempted := false
	gooseInstalled := false

	lookPath := func(file string) (string, error) {
		switch file {
		case "git", "gh", "claude":
			return "/usr/bin/" + file, nil
		case "goose":
			if gooseInstalled {
				return "/usr/bin/goose", nil
			}
			return "", fmt.Errorf("goose not found")
		default:
			return "", fmt.Errorf("%s not found", file)
		}
	}

	run := func(name string, args ...string) (string, error) {
		// gh auth status succeeds.
		if name == "gh" && len(args) > 0 && args[0] == "auth" {
			return "Logged in", nil
		}
		// Install command for goose.
		if name == "bash" && len(args) > 0 && args[0] == "-c" {
			installAttempted = true
			gooseInstalled = true
			return "", nil
		}
		return "", nil
	}

	// User accepts the install prompt.
	confirm := func(prompt string) (bool, error) {
		return true, nil
	}

	var buf bytes.Buffer
	err := RunPreflight(&buf, lookPath, run, confirm)

	if err != nil {
		t.Fatalf("expected nil error after goose install, got: %v\nOutput:\n%s", err, buf.String())
	}

	if !installAttempted {
		t.Error("expected install to be attempted for goose")
	}

	output := buf.String()
	if !strings.Contains(output, "goose") {
		t.Errorf("expected 'goose' mentioned in output, got:\n%s", output)
	}
}
