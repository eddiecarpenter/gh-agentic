package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validFrontmatter returns a minimal-but-conformant skill file body with
// the supplied category. The other fields are set to valid defaults.
func validFrontmatter(name, category string, emits bool) string {
	handsTo := "null"
	ebStr := "false"
	if emits {
		handsTo = "\"automation: next\""
		ebStr = "true"
	}
	// Keep description short but trigger-oriented to stay under the 1024-char cap.
	return "---\n" +
		"name: " + name + "\n" +
		"description: Does a testable thing and hands off. Use when a test fixture calls this skill.\n" +
		"category: " + category + "\n" +
		"triggers: on-demand\n" +
		"loads: []\n" +
		"emits-exit-block: " + ebStr + "\n" +
		"exit-hands-to: " + handsTo + "\n" +
		"---\n\n# " + name + "\n\nBody.\n"
}

// writeSkill creates a skill file under root/skills/<name>/SKILL.md (the
// canonical subdirectory layout).
func writeSkill(t *testing.T, root, name, body string) {
	t.Helper()
	skillDir := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skills/%s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s/SKILL.md: %v", name, err)
	}
}

// findResult returns the first CheckResult whose Message contains needle, or
// nil if none is found.
func findResult(g Group, needle string) *CheckResult {
	for i := range g.Results {
		if strings.Contains(g.Results[i].Message, needle) {
			return &g.Results[i]
		}
	}
	return nil
}

func TestCheckSkillFrontmatter_Valid(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "valid-skill", validFrontmatter("valid-skill", "Reference", false))

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	for _, r := range g.Results {
		if r.Status == Fail {
			t.Errorf("expected no failures for valid frontmatter; got fail: %s", r.Message)
		}
	}
	r := findResult(g, "valid-skill/SKILL.md frontmatter valid")
	if r == nil || r.Status != Pass {
		t.Errorf("expected valid-skill/SKILL.md to pass; got %+v", r)
	}
}

func TestCheckSkillFrontmatter_MissingCategory(t *testing.T) {
	root := t.TempDir()
	body := "---\n" +
		"name: broken-skill\n" +
		"description: Missing category field. Use when testing the validator.\n" +
		"triggers: on-demand\n" +
		"loads: []\n" +
		"emits-exit-block: false\n" +
		"exit-hands-to: null\n" +
		"---\n\n# broken-skill\n\nBody.\n"
	writeSkill(t, root, "broken-skill", body)

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "category field is missing")
	if r == nil {
		t.Fatal("expected a failure mentioning 'category field is missing'")
	}
	if r.Status != Fail {
		t.Errorf("expected Fail, got status %d", r.Status)
	}
	if !strings.Contains(r.Message, "broken-skill") {
		t.Errorf("expected message to name the skill, got: %s", r.Message)
	}
	if !strings.Contains(r.Message, "skills/skill-categories.md") {
		t.Errorf("expected message to include 'see:' pointer to schema, got: %s", r.Message)
	}
}

func TestCheckSkillFrontmatter_InvalidCategory(t *testing.T) {
	root := t.TempDir()
	body := "---\n" +
		"name: weird-skill\n" +
		"description: Invalid category value. Use when testing the validator.\n" +
		"category: Magical\n" +
		"triggers: on-demand\n" +
		"loads: []\n" +
		"emits-exit-block: false\n" +
		"exit-hands-to: null\n" +
		"---\n\n# weird-skill\n\nBody.\n"
	writeSkill(t, root, "weird-skill", body)

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, `category "Magical" is not allowed`)
	if r == nil {
		t.Fatal("expected a failure mentioning the invalid category value")
	}
	if r.Status != Fail {
		t.Errorf("expected Fail status")
	}
	if !strings.Contains(r.Message, "Session, Recovery, Bootstrap, Operation, Information, Reference") {
		t.Errorf("expected enumerated allowed categories in message, got: %s", r.Message)
	}
}

func TestCheckSkillFrontmatter_MalformedYAML(t *testing.T) {
	root := t.TempDir()
	body := "---\nname: bad-yaml\ndescription: [not\nproperly\nclosed\n---\n# bad-yaml\n"
	writeSkill(t, root, "bad-yaml", body)

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "YAML parse error")
	if r == nil {
		t.Fatal("expected a failure mentioning 'YAML parse error'")
	}
	if r.Status != Fail {
		t.Errorf("expected Fail status")
	}
}

func TestCheckSkillFrontmatter_NoFence(t *testing.T) {
	root := t.TempDir()
	body := "# no-frontmatter-skill\n\nBody.\n"
	writeSkill(t, root, "no-fence", body)

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "missing opening '---' fence")
	if r == nil {
		t.Fatal("expected a failure mentioning the missing opening fence")
	}
	if r.Status != Fail {
		t.Errorf("expected Fail status")
	}
}

func TestCheckSkillFrontmatter_InconsistentEmitsExitBlock(t *testing.T) {
	root := t.TempDir()
	// Reference category declared but emits-exit-block is true — inconsistent.
	body := "---\n" +
		"name: inconsistent\n" +
		"description: Category and emits-exit-block disagree. Use when testing consistency rules.\n" +
		"category: Reference\n" +
		"triggers: on-demand\n" +
		"loads: []\n" +
		"emits-exit-block: true\n" +
		"exit-hands-to: \"automation: next\"\n" +
		"---\n\n# inconsistent\n"
	writeSkill(t, root, "inconsistent", body)

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "emits-exit-block=true is inconsistent with category=Reference")
	if r == nil {
		t.Fatal("expected a failure about emits-exit-block ↔ category consistency")
	}
	if r.Status != Fail {
		t.Errorf("expected Fail status")
	}
}

func TestCheckSkillFrontmatter_UnknownField(t *testing.T) {
	root := t.TempDir()
	body := "---\n" +
		"name: extra-field\n" +
		"description: Has an unknown field. Use when testing strictness.\n" +
		"category: Reference\n" +
		"triggers: on-demand\n" +
		"loads: []\n" +
		"emits-exit-block: false\n" +
		"exit-hands-to: null\n" +
		"version: 1.2.3\n" +
		"---\n\n# extra-field\n"
	writeSkill(t, root, "extra-field", body)

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, `unknown field "version"`)
	if r == nil {
		t.Fatal("expected a failure mentioning the unknown field")
	}
	if r.Status != Fail {
		t.Errorf("expected Fail status")
	}
}

func TestCheckSkillFrontmatter_NoSkillsDir(t *testing.T) {
	root := t.TempDir()
	// No skills directory at all.

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "skills/ not found")
	if r == nil {
		t.Fatal("expected a warning when no skills directory exists")
	}
	if r.Status != Warning {
		t.Errorf("expected Warning, got status %d", r.Status)
	}
}

func TestCheckSkillFrontmatter_FederatedMount(t *testing.T) {
	// Domain repo where skills live under .agents/skills — validation must be
	// skipped entirely (framework files are read-only and trusted).
	root := t.TempDir()
	aiSkillDir := filepath.Join(root, ".agents", "skills", "framework-skill")
	if err := os.MkdirAll(aiSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir .agents/skills/framework-skill: %v", err)
	}
	body := validFrontmatter("framework-skill", "Reference", false)
	if err := os.WriteFile(filepath.Join(aiSkillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	// Should emit a single pass-through result, not per-file validation results.
	r := findResult(g, "framework skills")
	if r == nil {
		t.Fatal("expected a pass-through result for .agents/skills")
	}
	if r.Status != Pass {
		t.Errorf("expected Pass for framework bypass, got status %d", r.Status)
	}
	// Must NOT attempt to validate individual skill files.
	if findResult(g, "framework-skill/SKILL.md") != nil {
		t.Error("expected no per-file validation for .agents/ skills")
	}
}

func TestValidateFrontmatter_DescriptionTooLong(t *testing.T) {
	fm := map[string]interface{}{
		"name":             "long-desc",
		"description":      strings.Repeat("x", 1025),
		"category":         "Reference",
		"triggers":         "on-demand",
		"loads":            []interface{}{},
		"emits-exit-block": false,
		"exit-hands-to":    nil,
	}
	violations := validateFrontmatter(fm)

	found := false
	for _, v := range violations {
		if strings.Contains(v, "description length 1025 exceeds 1024") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected description-length violation; got: %v", violations)
	}
}

func TestValidateFrontmatter_ReservedName(t *testing.T) {
	fm := map[string]interface{}{
		"name":             "claude",
		"description":      "uses a reserved name",
		"category":         "Reference",
		"triggers":         "on-demand",
		"loads":            []interface{}{},
		"emits-exit-block": false,
		"exit-hands-to":    nil,
	}
	violations := validateFrontmatter(fm)

	found := false
	for _, v := range violations {
		if strings.Contains(v, `"claude" is reserved`) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected reserved-name violation; got: %v", violations)
	}
}

func TestValidateFrontmatter_LoadsMustBeList(t *testing.T) {
	fm := map[string]interface{}{
		"name":             "bad-loads",
		"description":      "loads as string",
		"category":         "Reference",
		"triggers":         "on-demand",
		"loads":            "not-a-list",
		"emits-exit-block": false,
		"exit-hands-to":    nil,
	}
	violations := validateFrontmatter(fm)

	found := false
	for _, v := range violations {
		if strings.Contains(v, "loads must be a list of strings") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected loads-type violation; got: %v", violations)
	}
}
