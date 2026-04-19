package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// writeSkill creates a skill file under root/skills with the given name.
func writeSkill(t *testing.T, root, filename, body string) {
	t.Helper()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, filename), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
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
	writeSkill(t, root, "valid-skill.md", validFrontmatter("valid-skill", "Reference", false))

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	for _, r := range g.Results {
		if r.Status == Fail {
			t.Errorf("expected no failures for valid frontmatter; got fail: %s", r.Message)
		}
	}
	r := findResult(g, "valid-skill.md frontmatter valid")
	if r == nil || r.Status != Pass {
		t.Errorf("expected valid-skill.md to pass; got %+v", r)
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
	writeSkill(t, root, "broken-skill.md", body)

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "category field is missing")
	if r == nil {
		t.Fatal("expected a failure mentioning 'category field is missing'")
	}
	if r.Status != Fail {
		t.Errorf("expected Fail, got status %d", r.Status)
	}
	if !strings.Contains(r.Message, "broken-skill.md") {
		t.Errorf("expected message to name the file, got: %s", r.Message)
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
	writeSkill(t, root, "weird-skill.md", body)

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
	writeSkill(t, root, "bad-yaml.md", body)

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
	writeSkill(t, root, "no-fence.md", body)

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
	writeSkill(t, root, "inconsistent.md", body)

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
	writeSkill(t, root, "extra-field.md", body)

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, `unknown field "version"`)
	if r == nil {
		t.Fatal("expected a failure mentioning the unknown field")
	}
	if r.Status != Fail {
		t.Errorf("expected Fail status")
	}
}

func TestCheckCatalogue_Missing(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "one.md", validFrontmatter("one", "Reference", false))
	// No CATALOGUE.md at root.

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "CATALOGUE.md missing")
	if r == nil {
		t.Fatal("expected a warning about the missing catalogue")
	}
	if r.Status != Warning {
		t.Errorf("expected Warning status, got %d", r.Status)
	}
}

func TestCheckCatalogue_Stale(t *testing.T) {
	root := t.TempDir()
	// Create CATALOGUE.md first (old mtime), then a skill later so the skill
	// mtime is strictly newer.
	cataloguePath := filepath.Join(root, "CATALOGUE.md")
	if err := os.WriteFile(cataloguePath, []byte("# Skill Catalogue\n"), 0o644); err != nil {
		t.Fatalf("write catalogue: %v", err)
	}
	older := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(cataloguePath, older, older); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	writeSkill(t, root, "newer-skill.md", validFrontmatter("newer-skill", "Reference", false))
	newer := time.Now()
	if err := os.Chtimes(filepath.Join(root, "skills", "newer-skill.md"), newer, newer); err != nil {
		t.Fatalf("chtimes skill: %v", err)
	}

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "CATALOGUE.md stale")
	if r == nil {
		t.Fatal("expected a warning about a stale catalogue")
	}
	if r.Status != Warning {
		t.Errorf("expected Warning status, got %d", r.Status)
	}
	if !strings.Contains(r.Message, "newer-skill.md") {
		t.Errorf("expected stale message to name the newer file; got: %s", r.Message)
	}
}

func TestCheckCatalogue_UpToDate(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "stable.md", validFrontmatter("stable", "Reference", false))
	older := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, "skills", "stable.md"), older, older); err != nil {
		t.Fatalf("chtimes skill: %v", err)
	}

	cataloguePath := filepath.Join(root, "CATALOGUE.md")
	if err := os.WriteFile(cataloguePath, []byte("# Skill Catalogue\n"), 0o644); err != nil {
		t.Fatalf("write catalogue: %v", err)
	}
	// Catalogue mtime is "now", skill is older — catalogue fresh.

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "CATALOGUE.md up to date")
	if r == nil {
		t.Fatal("expected a pass for an up-to-date catalogue")
	}
	if r.Status != Pass {
		t.Errorf("expected Pass status, got %d", r.Status)
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
	// Domain repo where skills live under .ai/skills — the check should find them there.
	root := t.TempDir()
	aiSkills := filepath.Join(root, ".ai", "skills")
	if err := os.MkdirAll(aiSkills, 0o755); err != nil {
		t.Fatalf("mkdir .ai/skills: %v", err)
	}
	body := validFrontmatter("framework-skill", "Reference", false)
	if err := os.WriteFile(filepath.Join(aiSkills, "framework-skill.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	g := checkSkillFrontmatter(CheckDeps{Root: root})

	r := findResult(g, "framework-skill.md frontmatter valid")
	if r == nil {
		t.Fatal("expected the .ai/skills/ skill to be discovered")
	}
	if r.Status != Pass {
		t.Errorf("expected Pass, got status %d", r.Status)
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
