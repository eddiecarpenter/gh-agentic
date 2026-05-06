package mount

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowTemplate_ReferencesPipelineWorkflow(t *testing.T) {
	content := workflowTemplate("v2.0.0")

	if !strings.Contains(content, "agentic-pipeline.yml@v2.0.0") {
		t.Errorf("workflow should reference pipeline workflow with version, got:\n%s", content)
	}
	// Guard against regression to the pre-v2.3.0 filename.
	if strings.Contains(content, "agentic-pipeline-reusable.yml") {
		t.Errorf("workflow must not reference the legacy -reusable filename (collapsed in v2.3.0), got:\n%s", content)
	}
}

func TestWorkflowTemplate_ForwardsDispatchInputs(t *testing.T) {
	content := workflowTemplate("v2.0.0")

	// workflow_dispatch inputs must be forwarded to the reusable via
	// `with:` — without that, the pr-review-session branch of the callee
	// sees empty strings. See feature #619 addendum.
	required := []string{
		"with:",
		"pr_number: ${{ inputs.pr_number || '' }}",
		"branch_name: ${{ inputs.branch_name || '' }}",
	}
	for _, f := range required {
		if !strings.Contains(content, f) {
			t.Errorf("workflow must forward dispatch inputs via with: — missing %q in:\n%s", f, content)
		}
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

func TestReleaseWorkflowTemplate_ReferencesPipelineWorkflow(t *testing.T) {
	content := releaseWorkflowTemplate("v2.0.0")

	if !strings.Contains(content, "release.yml@v2.0.0") {
		t.Errorf("release workflow should reference the consolidated release.yml with version, got:\n%s", content)
	}
	// Guard against the dangling legacy reference we shipped for several
	// releases without a corresponding file ever existing.
	if strings.Contains(content, "release-reusable.yml") {
		t.Errorf("release workflow must not reference release-reusable.yml — that file has never existed in gh-agentic; got:\n%s", content)
	}
}

func TestReleaseWorkflowTemplate_HasRequiredFields(t *testing.T) {
	content := releaseWorkflowTemplate("v2.0.0")

	required := []string{
		"name: Release",
		// Consolidated release workflow is triggered by `release: published`,
		// not by tag push. Release creation is each project's own concern.
		"release:",
		"types: [published]",
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
	if !strings.Contains(agentsMDTemplate, "@.agents/RULEBOOK.md") {
		t.Error("AGENTS.md template should reference @.agents/RULEBOOK.md")
	}
	if !strings.Contains(agentsMDTemplate, "@LOCALRULES.md") {
		t.Error("AGENTS.md template should reference @LOCALRULES.md")
	}
	if !strings.Contains(agentsMDTemplate, "gh agentic mount") {
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

func TestAGENTSMDTemplate_BootstrapRuleCommand(t *testing.T) {
	// Bootstrap rule references the argument-less mount command — the
	// framework version is resolved from AGENTIC_FRAMEWORK_VERSION (or the
	// latest release) by project.Resolve, so callers do not supply a version
	// at the command line.
	if !strings.Contains(agentsMDTemplate, "gh agentic mount") {
		t.Error("AGENTS.md bootstrap rule should reference 'gh agentic mount'")
	}
	if strings.Contains(agentsMDTemplate, "$(cat .ai-version)") {
		t.Error("AGENTS.md bootstrap rule must not embed $(cat .ai-version) — the file is gone (#585)")
	}
}

func TestAGENTSMDTemplate_BootstrapRuleAIAbsence(t *testing.T) {
	// Bootstrap rule activates when .agents/ is absent.
	if !strings.Contains(agentsMDTemplate, ".agents/ directory does not exist") {
		t.Error("AGENTS.md bootstrap rule should mention '.agents/ directory does not exist'")
	}
}

func TestAGENTSMDTemplate_BootstrapRuleStops(t *testing.T) {
	// Bootstrap rule should instruct Claude to stop.
	if !strings.Contains(agentsMDTemplate, "stop immediately") {
		t.Error("AGENTS.md bootstrap rule should instruct to 'stop immediately'")
	}
}

func TestAGENTSMDTemplate_BootstrapRuleNoProceeding(t *testing.T) {
	// Bootstrap rule should prevent proceeding without mount.
	if !strings.Contains(agentsMDTemplate, "Do not proceed") {
		t.Error("AGENTS.md bootstrap rule should say 'Do not proceed'")
	}
}

func TestAGENTSMDTemplateFile_MatchesEmbeddedTemplate(t *testing.T) {
	// Verify the AGENTS.md.tmpl file exists and has consistent content.
	data, err := os.ReadFile("templates/AGENTS.md.tmpl")
	if err != nil {
		t.Skipf("template file not found (may be running from different dir): %v", err)
	}
	content := string(data)

	// Both should have the same bootstrap rule elements.
	if !strings.Contains(content, "gh agentic mount") {
		t.Error("template file should reference mount command")
	}
	if strings.Contains(content, "$(cat .ai-version)") {
		t.Error("template file must not embed $(cat .ai-version) — the file is gone (#585)")
	}
	if !strings.Contains(content, "Interactive context") {
		t.Error("template file should distinguish interactive context")
	}
	if !strings.Contains(content, "CI context") {
		t.Error("template file should distinguish CI context")
	}
}
