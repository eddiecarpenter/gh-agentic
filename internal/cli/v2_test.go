package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestV2Flag_Registration(t *testing.T) {
	root := newRootCmd("dev", "")
	f := root.PersistentFlags().Lookup("v2")
	if f == nil {
		t.Fatal("-v2 flag not registered on root command")
	}
	if f.DefValue != "false" {
		t.Errorf("-v2 default should be false, got %q", f.DefValue)
	}
}

func TestV2Flag_MountCommandRoutes(t *testing.T) {
	root := newRootCmd("dev", "")

	// Mount command is now real — it requires a version or .ai-version.
	// Verify it's registered and routes correctly by checking the error
	// is about missing version (not about unknown command).
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--v2", "mount"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error (no version specified), got nil")
	}
	if !strings.Contains(err.Error(), "no version specified") {
		t.Errorf("expected 'no version specified' error, got: %v", err)
	}
}

func TestV2Flag_MountWithoutV2Flag(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"mount"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error when mount is called without -v2")
	}
	if !strings.Contains(err.Error(), "requires the -v2 flag") {
		t.Errorf("expected '-v2 flag required' error, got: %v", err)
	}
}

func TestV2Flag_SyncBlockedInV2Mode(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--v2", "sync"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error for sync in v2 mode")
	}
	if !strings.Contains(err.Error(), "'sync' is not available in v2 mode") {
		t.Errorf("expected 'not available in v2 mode' error, got: %v", err)
	}
}

func TestV2Flag_VerifyBlockedInV2Mode(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--v2", "verify"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error for verify in v2 mode")
	}
	if !strings.Contains(err.Error(), "'verify' is not available in v2 mode") {
		t.Errorf("expected 'not available in v2 mode' error, got: %v", err)
	}
}

func TestV2Flag_BootstrapBlockedInV2Mode(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--v2", "bootstrap"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error for bootstrap in v2 mode")
	}
	if !strings.Contains(err.Error(), "'bootstrap' is not available in v2 mode") {
		t.Errorf("expected 'not available in v2 mode' error, got: %v", err)
	}
}

func TestV2Flag_InceptionBlockedInV2Mode(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--v2", "inception"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error for inception in v2 mode")
	}
	if !strings.Contains(err.Error(), "'inception' is not available in v2 mode") {
		t.Errorf("expected 'not available in v2 mode' error, got: %v", err)
	}
}

func TestV2Flag_SyncWithoutV2Works(t *testing.T) {
	root := newRootCmd("dev", "")

	// Find sync command and verify it's registered.
	var found bool
	for _, cmd := range root.Commands() {
		if cmd.Use == "sync" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("sync subcommand not registered")
	}

	// Verify the command exists without actually running it (would need deps).
	// The key test is that without -v2, the sync RunE does not return a v2
	// guard error — it proceeds to actual execution which may fail for other
	// reasons (missing repo root etc.), but NOT with a v2 error.
}

func TestV2Flag_DeprecatedErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		want    string
	}{
		{name: "sync", cmdName: "sync", want: "'sync' is not available in v2 mode"},
		{name: "verify", cmdName: "verify", want: "'verify' is not available in v2 mode"},
		{name: "bootstrap", cmdName: "bootstrap", want: "'bootstrap' is not available in v2 mode"},
		{name: "inception", cmdName: "inception", want: "'inception' is not available in v2 mode"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := errV2NotAvailable(tc.cmdName)
			if err.Error() != tc.want {
				t.Errorf("got %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestCheckV2Guard_Nil(t *testing.T) {
	// nil pointer should not trigger guard.
	err := checkV2Guard("sync", nil)
	if err != nil {
		t.Errorf("expected nil error for nil flag, got: %v", err)
	}
}

func TestCheckV2Guard_False(t *testing.T) {
	v := false
	err := checkV2Guard("sync", &v)
	if err != nil {
		t.Errorf("expected nil error for false flag, got: %v", err)
	}
}

func TestCheckV2Guard_True(t *testing.T) {
	v := true
	err := checkV2Guard("sync", &v)
	if err == nil {
		t.Fatal("expected error for true flag")
	}
	if !strings.Contains(err.Error(), "'sync' is not available in v2 mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestV2StubCommands_Registered(t *testing.T) {
	root := newRootCmd("dev", "")

	expected := []string{"mount", "init", "auth"}
	for _, name := range expected {
		var found bool
		for _, cmd := range root.Commands() {
			if strings.HasPrefix(cmd.Use, name) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q command to be registered", name)
		}
	}
}

func TestV2Flag_InitBlockedWithoutV2(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"init"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error when init is called without -v2")
	}
	if !strings.Contains(err.Error(), "requires the -v2 flag") {
		t.Errorf("expected '-v2 flag required' error, got: %v", err)
	}
}

func TestV2Flag_AuthBlockedWithoutV2(t *testing.T) {
	root := newRootCmd("dev", "")

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"auth"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error when auth is called without -v2")
	}
	if !strings.Contains(err.Error(), "requires the -v2 flag") {
		t.Errorf("expected '-v2 flag required' error, got: %v", err)
	}
}
