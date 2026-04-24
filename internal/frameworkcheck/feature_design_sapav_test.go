package frameworkcheck

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestFeatureDesign_LoadsCaptureDesignPlan verifies skills/feature-design.md
// declares capture-design-plan in its loads list — the design agent must be
// able to read the Design Plan template at publish time (task #661, AC1).
func TestFeatureDesign_LoadsCaptureDesignPlan(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "feature-design.md")
	body := readFile(t, path)
	fm, _ := parseFrontmatter(t, body)

	loads, ok := fm["loads"].([]any)
	if !ok {
		t.Fatalf("feature-design loads: want list, got %T", fm["loads"])
	}
	for _, v := range loads {
		if s, ok := v.(string); ok && s == "capture-design-plan" {
			return
		}
	}
	t.Errorf("feature-design.md loads must include 'capture-design-plan'; got %v", loads)
}

// TestFeatureDesign_PublishPrecedesTaskCreation verifies the Design Plan
// publish step exists in the body AND appears before the Task creation step
// — the SAPAV publish-before-act discipline (task #661, AC2).
func TestFeatureDesign_PublishPrecedesTaskCreation(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "feature-design.md")
	body := readFile(t, path)
	_, skillBody := parseFrontmatter(t, body)

	// The publish step must mention both the CLI command (gh issue comment)
	// and the phrase 'Design Plan' to be identifiable.
	publishIdx := strings.Index(skillBody, "gh issue comment")
	if publishIdx < 0 {
		t.Fatalf("feature-design.md missing 'gh issue comment' invocation for Design Plan publish")
	}
	if !strings.Contains(skillBody, "Design Plan") {
		t.Fatalf("feature-design.md missing 'Design Plan' phrase")
	}

	// The task-creation step uses the anchor phrase from the existing skill:
	// "Creates Task sub-issues".
	taskCreationIdx := strings.Index(skillBody, "Creates Task sub-issues")
	if taskCreationIdx < 0 {
		t.Fatalf("feature-design.md missing 'Creates Task sub-issues' anchor")
	}

	if publishIdx >= taskCreationIdx {
		t.Errorf("publish step (idx %d) must precede task-creation step (idx %d)", publishIdx, taskCreationIdx)
	}
}

// TestFeatureDesign_AmendFollowsTaskCreation verifies an append-only amend
// step exists after Task creation — the moment the #N references become
// available (task #661, AC3).
func TestFeatureDesign_AmendFollowsTaskCreation(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "feature-design.md")
	body := readFile(t, path)
	_, skillBody := parseFrontmatter(t, body)

	taskCreationIdx := strings.Index(skillBody, "Creates Task sub-issues")
	if taskCreationIdx < 0 {
		t.Fatalf("feature-design.md missing 'Creates Task sub-issues' anchor")
	}

	// The amend step uses the canonical append-only anchor:
	// 'Tasks (created)' subsection — matches the append-only discipline
	// defined in skills/capture-design-plan.md.
	amendIdx := strings.Index(skillBody, "Tasks (created)")
	if amendIdx < 0 {
		t.Fatalf("feature-design.md missing 'Tasks (created)' amend-step anchor")
	}

	if amendIdx <= taskCreationIdx {
		t.Errorf("amend step (idx %d) must follow task-creation step (idx %d)", amendIdx, taskCreationIdx)
	}

	// Amend must be explicitly append-only — verify the discipline is named.
	if !strings.Contains(skillBody, "append-only") {
		t.Errorf("feature-design.md amend step must state the 'append-only' discipline")
	}
}

// TestFeatureDesign_HaltOnPublishFailure verifies the halt-on-failure wording
// is explicit and names the three blocked follow-on actions (task #661,
// AC4 — covers feature AC2).
func TestFeatureDesign_HaltOnPublishFailure(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "feature-design.md")
	body := readFile(t, path)
	_, skillBody := parseFrontmatter(t, body)

	// The halt wording must use at least one of the canonical halt phrases.
	haltPhrases := []string{"halt", "REFUSED", "does not proceed to Task creation"}
	haltFound := false
	for _, p := range haltPhrases {
		if strings.Contains(skillBody, p) {
			haltFound = true
			break
		}
	}
	if !haltFound {
		t.Errorf("feature-design.md missing halt-on-failure wording (any of %v)", haltPhrases)
	}

	// The body must name each of the three things that must not happen on
	// publish failure: task creation, branch creation, in-development label.
	blockedActions := map[string][]string{
		"task creation":        {"task creation", "Task creation", "creating tasks", "create tasks"},
		"branch creation":      {"branch creation", "create the feature branch", "creating the feature branch", "create the branch"},
		"in-development label": {"in-development label", "`in-development`", "in-development"},
	}
	for action, variants := range blockedActions {
		found := false
		for _, v := range variants {
			if strings.Contains(skillBody, v) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("feature-design.md halt wording must name blocked action %q (tried %v)", action, variants)
		}
	}
}

// TestFeatureDesign_ExitBlockIncludesDesignPlanURL verifies the exit-block
// example in the skill body includes the Design Plan comment URL line
// (task #661, AC5).
func TestFeatureDesign_ExitBlockIncludesDesignPlanURL(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "feature-design.md")
	body := readFile(t, path)
	_, skillBody := parseFrontmatter(t, body)

	// The exit block starts with the canonical === header; the Design Plan
	// line must appear within it. We require the literal phrase
	// 'Design Plan comment:' (with colon) to be present — the URL value
	// itself is templated.
	exitHeaderIdx := strings.Index(skillBody, "=== Feature Design Session — Completed ===")
	if exitHeaderIdx < 0 {
		t.Fatalf("feature-design.md exit-block header missing")
	}
	tail := skillBody[exitHeaderIdx:]
	// Limit the search window to the first exit block closing fence so a
	// later reference cannot mask a missing line.
	if end := strings.Index(tail, "\n```\n"); end >= 0 {
		tail = tail[:end]
	}
	if !strings.Contains(tail, "Design Plan comment:") {
		t.Errorf("feature-design.md exit block must include 'Design Plan comment:' line")
	}
}
