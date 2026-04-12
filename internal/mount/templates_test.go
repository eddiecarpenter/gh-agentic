package mount

import (
	"strings"
	"testing"
)

func TestWorkflowTemplate_ReferencesReusableWorkflow(t *testing.T) {
	content := workflowTemplate("v2.0.0")

	if !strings.Contains(content, "agentic-pipeline-reusable.yml@v2.0.0") {
		t.Errorf("workflow should reference reusable workflow with version, got:\n%s", content)
	}
}

func TestWorkflowTemplate_HasRequiredFields(t *testing.T) {
	content := workflowTemplate("v2.0.0")

	required := []string{
		"name: Agentic Pipeline",
		"issues:",
		"pull_request:",
		"pull_request_review:",
		"workflow_dispatch:",
		"secrets: inherit",
		"permissions:",
	}

	for _, field := range required {
		if !strings.Contains(content, field) {
			t.Errorf("workflow should contain %q", field)
		}
	}
}

func TestReleaseWorkflowTemplate_ReferencesReusableWorkflow(t *testing.T) {
	content := releaseWorkflowTemplate("v2.0.0")

	if !strings.Contains(content, "release-reusable.yml@v2.0.0") {
		t.Errorf("release workflow should reference reusable workflow with version, got:\n%s", content)
	}
}

func TestReleaseWorkflowTemplate_HasRequiredFields(t *testing.T) {
	content := releaseWorkflowTemplate("v2.0.0")

	required := []string{
		"name: Release",
		"push:",
		"tags:",
		"secrets: inherit",
	}

	for _, field := range required {
		if !strings.Contains(content, field) {
			t.Errorf("release workflow should contain %q", field)
		}
	}
}

func TestWorkflowTemplate_VersionSubstitution(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{version: "v2.0.0", want: "@v2.0.0"},
		{version: "v2.1.3", want: "@v2.1.3"},
		{version: "v3.0.0-beta.1", want: "@v3.0.0-beta.1"},
	}

	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			content := workflowTemplate(tc.version)
			if !strings.Contains(content, tc.want) {
				t.Errorf("workflow should contain %q for version %s", tc.want, tc.version)
			}
		})
	}
}

func TestCLAUDEMDTemplate_Content(t *testing.T) {
	if !strings.Contains(claudeMDTemplate, "@AGENTS.md") {
		t.Error("CLAUDE.md template should reference @AGENTS.md")
	}
}

func TestAGENTSMDTemplate_Content(t *testing.T) {
	if !strings.Contains(agentsMDTemplate, "@.ai/RULEBOOK.md") {
		t.Error("AGENTS.md template should reference @.ai/RULEBOOK.md")
	}
	if !strings.Contains(agentsMDTemplate, "@LOCALRULES.md") {
		t.Error("AGENTS.md template should reference @LOCALRULES.md")
	}
	if !strings.Contains(agentsMDTemplate, "gh agentic -v2 mount") {
		t.Error("AGENTS.md template should contain bootstrap rule")
	}
}

func TestAGENTSMDTemplate_BootstrapRule(t *testing.T) {
	// Verify the bootstrap rule distinguishes interactive vs CI context.
	if !strings.Contains(agentsMDTemplate, "Interactive context") {
		t.Error("AGENTS.md should mention interactive context")
	}
	if !strings.Contains(agentsMDTemplate, "CI context") {
		t.Error("AGENTS.md should mention CI context")
	}
}
