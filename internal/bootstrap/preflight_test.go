package bootstrap

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// fakeLookPath returns a LookPathFunc that reports the given set of tools as found.
func fakeLookPath(found map[string]bool) LookPathFunc {
	return func(file string) (string, error) {
		if found[file] {
			return "/usr/bin/" + file, nil
		}
		return "", errors.New("not found: " + file)
	}
}

// fakeRunCommand returns a RunCommandFunc that returns the given output/error
// for commands matching the prefix args[0].
func fakeRunCommand(responses map[string]struct {
	out string
	err error
}) RunCommandFunc {
	return func(name string, args ...string) (string, error) {
		key := name
		if len(args) > 0 {
			key = name + " " + args[0]
		}
		if r, ok := responses[key]; ok {
			return r.out, r.err
		}
		return "", nil
	}
}

// fakeConfirm returns a ConfirmFunc that always answers with the given value.
func fakeConfirm(answer bool) ConfirmFunc {
	return func(_ string) (bool, error) {
		return answer, nil
	}
}

func TestRunPreflight_AllToolsPresent_ReturnsNil(t *testing.T) {
	found := map[string]bool{
		"git":   true,
		"gh":    true,
		"goose": true,
		"claude": true,
	}
	run := fakeRunCommand(map[string]struct {
		out string
		err error
	}{
		"gh auth": {out: "Logged in", err: nil},
	})

	var buf bytes.Buffer
	err := RunPreflight(&buf, fakeLookPath(found), run, fakeConfirm(false))

	if err != nil {
		t.Fatalf("RunPreflight() expected nil error when all tools present, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "git found") {
		t.Errorf("expected 'git found' in output, got: %s", out)
	}
	if !strings.Contains(out, "goose found") {
		t.Errorf("expected 'goose found' in output, got: %s", out)
	}
}

func TestRunPreflight_GitMissing_ReturnsError(t *testing.T) {
	found := map[string]bool{
		// git intentionally absent
		"gh":    true,
		"goose": true,
	}
	run := fakeRunCommand(map[string]struct {
		out string
		err error
	}{
		"gh auth": {out: "Logged in", err: nil},
	})

	var buf bytes.Buffer
	err := RunPreflight(&buf, fakeLookPath(found), run, fakeConfirm(false))

	if err == nil {
		t.Fatal("RunPreflight() expected error when git is missing, got nil")
	}
	if !strings.Contains(err.Error(), "git") {
		t.Errorf("error should mention 'git', got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "git-scm.com") {
		t.Errorf("expected git install URL in output, got: %s", out)
	}
}

func TestRunPreflight_GhMissing_ReturnsError(t *testing.T) {
	found := map[string]bool{
		"git":   true,
		// gh intentionally absent
	}
	run := fakeRunCommand(nil)

	var buf bytes.Buffer
	err := RunPreflight(&buf, fakeLookPath(found), run, fakeConfirm(false))

	if err == nil {
		t.Fatal("RunPreflight() expected error when gh is missing, got nil")
	}
	out := buf.String()
	if !strings.Contains(out, "cli.github.com") {
		t.Errorf("expected gh install URL in output, got: %s", out)
	}
}

func TestRunPreflight_GhAuthFails_ReturnsError(t *testing.T) {
	found := map[string]bool{
		"git": true,
		"gh":  true,
	}
	run := fakeRunCommand(map[string]struct {
		out string
		err error
	}{
		"gh auth": {out: "not logged in", err: errors.New("not authenticated")},
	})

	var buf bytes.Buffer
	err := RunPreflight(&buf, fakeLookPath(found), run, fakeConfirm(false))

	if err == nil {
		t.Fatal("RunPreflight() expected error when gh auth fails, got nil")
	}
	if !strings.Contains(err.Error(), "gh auth") {
		t.Errorf("error should mention 'gh auth', got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "gh auth login") {
		t.Errorf("expected 'gh auth login' hint in output, got: %s", out)
	}
}

func TestRunPreflight_GooseMissing_UserAcceptsInstall_InstallSucceeds(t *testing.T) {
	// goose not found initially, installed via bash, then found on re-verify.
	lookCallCount := 0
	lookPath := func(file string) (string, error) {
		if file == "git" || file == "gh" || file == "claude" {
			return "/usr/bin/" + file, nil
		}
		if file == "goose" {
			lookCallCount++
			if lookCallCount >= 2 {
				// Second call (post-install) succeeds.
				return "/usr/local/bin/goose", nil
			}
			return "", errors.New("not found")
		}
		return "", errors.New("not found: " + file)
	}

	run := fakeRunCommand(map[string]struct {
		out string
		err error
	}{
		"gh auth": {out: "Logged in", err: nil},
		// bash install succeeds
		"bash": {out: "", err: nil},
	})

	var buf bytes.Buffer
	err := RunPreflight(&buf, lookPath, run, fakeConfirm(true))

	if err != nil {
		t.Fatalf("RunPreflight() expected nil after successful install, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "goose") {
		t.Errorf("expected goose mention in output, got: %s", out)
	}
}

func TestRunPreflight_GooseMissing_UserDeclines_ReturnsError(t *testing.T) {
	found := map[string]bool{
		"git": true,
		"gh":  true,
		// goose absent
	}
	run := fakeRunCommand(map[string]struct {
		out string
		err error
	}{
		"gh auth": {out: "Logged in", err: nil},
	})

	var buf bytes.Buffer
	err := RunPreflight(&buf, fakeLookPath(found), run, fakeConfirm(false))

	if err == nil {
		t.Fatal("RunPreflight() expected error when goose missing and user declines, got nil")
	}
	if !strings.Contains(err.Error(), "goose") {
		t.Errorf("error should mention 'goose', got: %v", err)
	}
}

func TestRunPreflight_ClaudeMissing_UserDeclines_ContinuesWithWarning(t *testing.T) {
	found := map[string]bool{
		"git":   true,
		"gh":    true,
		"goose": true,
		// claude absent — recommended only
	}
	run := fakeRunCommand(map[string]struct {
		out string
		err error
	}{
		"gh auth": {out: "Logged in", err: nil},
	})

	var buf bytes.Buffer
	err := RunPreflight(&buf, fakeLookPath(found), run, fakeConfirm(false))

	if err != nil {
		t.Fatalf("RunPreflight() expected nil when only recommended tool is missing, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "claude not found (recommended)") {
		t.Errorf("expected recommended warning for claude, got: %s", out)
	}
	if !strings.Contains(out, "Skipping claude") {
		t.Errorf("expected skip message for claude, got: %s", out)
	}
}

func TestRunPreflight_OutputContainsSectionHeading(t *testing.T) {
	found := map[string]bool{
		"git":   true,
		"gh":    true,
		"goose": true,
		"claude": true,
	}
	run := fakeRunCommand(map[string]struct {
		out string
		err error
	}{
		"gh auth": {out: "ok", err: nil},
	})

	var buf bytes.Buffer
	_ = RunPreflight(&buf, fakeLookPath(found), run, fakeConfirm(false))

	out := buf.String()
	if !strings.Contains(out, "Preflight checks") {
		t.Errorf("expected 'Preflight checks' heading in output, got: %s", out)
	}
}

func TestDefaultLookPath_ReturnsPathOrError(t *testing.T) {
	// "sh" should be present on any unix system.
	path, err := DefaultLookPath("sh")
	if err != nil {
		t.Errorf("DefaultLookPath('sh') expected no error, got: %v", err)
	}
	if path == "" {
		t.Error("DefaultLookPath('sh') expected non-empty path")
	}

	// A made-up binary should not be found.
	_, err = DefaultLookPath("this-binary-does-not-exist-xyz123")
	if err == nil {
		t.Error("DefaultLookPath('this-binary-does-not-exist-xyz123') expected error, got nil")
	}
}
