package inception

import (
	"strings"
	"testing"
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
