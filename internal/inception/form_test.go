package inception

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
)

// --- validateRepoName tests ---

func TestValidateRepoName_ValidNames_ReturnsNil(t *testing.T) {
	valid := []string{
		"charging",
		"my-service",
		"a-b-c-123",
		"ocs-testbench",
		"project1",
		"a",
	}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			if err := validateRepoName(name); err != nil {
				t.Errorf("validateRepoName(%q) expected nil, got: %v", name, err)
			}
		})
	}
}

func TestValidateRepoName_EmptyString_ReturnsError(t *testing.T) {
	err := validateRepoName("")
	if err == nil {
		t.Error("validateRepoName(\"\") expected error, got nil")
	}
}

func TestValidateRepoName_Uppercase_ReturnsError(t *testing.T) {
	err := validateRepoName("MyProject")
	if err == nil {
		t.Error("validateRepoName(\"MyProject\") expected error, got nil")
	}
}

func TestValidateRepoName_Spaces_ReturnsError(t *testing.T) {
	err := validateRepoName("my project")
	if err == nil {
		t.Error("validateRepoName(\"my project\") expected error for space, got nil")
	}
}

func TestValidateRepoName_SpecialChars_ReturnsError(t *testing.T) {
	for _, name := range []string{"my_project", "my.project", "my@project"} {
		err := validateRepoName(name)
		if err == nil {
			t.Errorf("validateRepoName(%q) expected error, got nil", name)
		}
	}
}

// --- validateStackSelection tests ---

func TestValidateStackSelection_Empty_ReturnsError(t *testing.T) {
	err := validateStackSelection([]string{})
	if err == nil {
		t.Error("validateStackSelection([]) expected error, got nil")
	}
}

func TestValidateStackSelection_OneStack_ReturnsNil(t *testing.T) {
	err := validateStackSelection([]string{"Go"})
	if err != nil {
		t.Errorf("validateStackSelection([Go]) expected nil, got: %v", err)
	}
}

// --- DeriveRepoName tests ---

func TestDeriveRepoName_Domain_AppendsSuffix(t *testing.T) {
	got := DeriveRepoName("charging", "domain")
	if got != "charging-domain" {
		t.Errorf("DeriveRepoName() = %q, want %q", got, "charging-domain")
	}
}

func TestDeriveRepoName_Tool_AppendsSuffix(t *testing.T) {
	got := DeriveRepoName("testbench", "tool")
	if got != "testbench-tool" {
		t.Errorf("DeriveRepoName() = %q, want %q", got, "testbench-tool")
	}
}

func TestDeriveRepoName_Other_NoSuffix(t *testing.T) {
	got := DeriveRepoName("my-service", "other")
	if got != "my-service" {
		t.Errorf("DeriveRepoName() = %q, want %q", got, "my-service")
	}
}

// --- RenderSummaryBox tests ---

func TestRenderSummaryBox_ContainsAllFields(t *testing.T) {
	cfg := InceptionConfig{
		RepoType:    "domain",
		RepoName:    "charging",
		Description: "OCS charging engine",
		Stacks:     []string{"Go"},
		Owner:       "acme-org",
	}

	rendered := RenderSummaryBox(cfg)

	checks := []string{
		"domain",
		"charging",
		"acme-org/charging-domain",
		"OCS charging engine",
		"Go",
		"acme-org",
	}
	for _, want := range checks {
		if !strings.Contains(rendered, want) {
			t.Errorf("RenderSummaryBox() expected %q in output, got:\n%s", want, rendered)
		}
	}
}

func TestRenderSummaryBox_OtherType_NoSuffix(t *testing.T) {
	cfg := InceptionConfig{
		RepoType: "other",
		RepoName: "my-service",
		Owner:    "alice",
	}

	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "alice/my-service") {
		t.Errorf("RenderSummaryBox() expected 'alice/my-service' in output, got:\n%s", rendered)
	}
}

func TestRenderSummaryBox_ToolType_HasSuffix(t *testing.T) {
	cfg := InceptionConfig{
		RepoType: "tool",
		RepoName: "testbench",
		Owner:    "alice",
	}

	rendered := RenderSummaryBox(cfg)
	if !strings.Contains(rendered, "alice/testbench-tool") {
		t.Errorf("RenderSummaryBox() expected 'alice/testbench-tool' in output, got:\n%s", rendered)
	}
}

// --- FormRunFunc injection tests ---

func TestFormRunFunc_TypeDefined(t *testing.T) {
	var fn FormRunFunc = func(f *huh.Form) error {
		return nil
	}
	if err := fn(huh.NewForm()); err != nil {
		t.Errorf("expected nil from fake FormRunFunc, got: %v", err)
	}
}

func TestDefaultFormRun_IsNotNil(t *testing.T) {
	if DefaultFormRun == nil {
		t.Error("DefaultFormRun should not be nil")
	}
}

func TestRunForm_WithFakeFormRunFunc_CompletesWithoutTerminal(t *testing.T) {
	// Inject a fake FormRunFunc that returns nil without touching a terminal.
	// Since the confirm form's bound `confirmed` bool stays false,
	// RunForm returns ErrAborted — which proves it ran to completion without a TTY.
	var buf strings.Builder

	callCount := 0
	fakeFormRun := FormRunFunc(func(f *huh.Form) error {
		callCount++
		return nil
	})

	ctx := EnvContext{Owner: "alice"}
	_, err := RunForm(&buf, ctx, fakeFormRun)

	// The confirm form's bound value defaults to false, so RunForm returns ErrAborted.
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("expected ErrAborted, got: %v", err)
	}

	// Three forms should have been called: type, details, confirm.
	if callCount != 3 {
		t.Errorf("expected 3 form runs, got %d", callCount)
	}
}

func TestRunForm_WithFakeFormRunFunc_SetsOwnerFromContext(t *testing.T) {
	// Verify the config's Owner field is set from the EnvContext.
	var buf strings.Builder

	fakeFormRun := FormRunFunc(func(f *huh.Form) error {
		return nil
	})

	ctx := EnvContext{Owner: "acme-org"}
	// Will return ErrAborted because confirm stays false, but we can check
	// that the owner was set correctly via the summary output.
	_, _ = RunForm(&buf, ctx, fakeFormRun)

	output := buf.String()
	if !strings.Contains(output, "acme-org") {
		t.Errorf("expected owner 'acme-org' in output, got:\n%s", output)
	}
}

func TestRunForm_FormRunFunc_ErrorPropagation(t *testing.T) {
	var buf strings.Builder
	expectedErr := errors.New("form error")

	fakeFormRun := FormRunFunc(func(f *huh.Form) error {
		return expectedErr
	})

	ctx := EnvContext{Owner: "alice"}
	_, err := RunForm(&buf, ctx, fakeFormRun)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "form error") {
		t.Errorf("expected 'form error' in error, got: %v", err)
	}
}
