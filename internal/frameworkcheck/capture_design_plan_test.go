package frameworkcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCaptureDesignPlan_FileExists verifies the Design Plan template skill
// file is present at its canonical path (task #660, AC1).
func TestCaptureDesignPlan_FileExists(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "capture-design-plan.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("skills/capture-design-plan.md missing: %v", err)
	}
}

// TestCaptureDesignPlan_FrontmatterConformance verifies the frontmatter shape
// required by Reference-category skills per skills/skill-categories.md (task
// #660, AC1).
func TestCaptureDesignPlan_FrontmatterConformance(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "capture-design-plan.md")
	body := readFile(t, path)
	fm, _ := parseFrontmatter(t, body)

	if got := fm["name"]; got != "capture-design-plan" {
		t.Errorf("name: want %q, got %q", "capture-design-plan", got)
	}
	if got := fm["category"]; got != "Reference" {
		t.Errorf("category: want %q, got %q", "Reference", got)
	}
	if got, ok := fm["emits-exit-block"].(bool); !ok || got != false {
		t.Errorf("emits-exit-block: want false, got %v", fm["emits-exit-block"])
	}
	if fm["exit-hands-to"] != nil {
		t.Errorf("exit-hands-to: want null, got %v", fm["exit-hands-to"])
	}
	loads, ok := fm["loads"].([]any)
	if !ok {
		t.Fatalf("loads: want list, got %T", fm["loads"])
	}
	if len(loads) != 0 {
		t.Errorf("loads: want [], got %v", loads)
	}
}

// TestCaptureDesignPlan_RequiredSectionsPresent verifies the template body
// declares the five required Markdown headings plus the literal
// single-obvious-decomposition fallback phrase (task #660, AC2/AC3/AC4).
func TestCaptureDesignPlan_RequiredSectionsPresent(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "capture-design-plan.md")
	body := readFile(t, path)

	requiredHeadings := []string{
		"## Decomposition",
		"## Tasks",
		"## Alternatives Considered",
		"## Refactor Assessment",
		"## Codebase Findings",
		"## Risks",
	}
	for _, h := range requiredHeadings {
		if !strings.Contains(body, h) {
			t.Errorf("capture-design-plan.md missing required heading %q", h)
		}
	}

	// AC4 literal fallback phrase — must appear verbatim.
	const literalFallback = "_None — single obvious decomposition._"
	if !strings.Contains(body, literalFallback) {
		t.Errorf("capture-design-plan.md missing literal fallback phrase %q", literalFallback)
	}
}

// TestCaptureDesignPlan_WordCountBoundsDocumented verifies the Rules section
// documents the soft 300–500 and hard 1000 word bounds required by AC5
// (task #660, AC4 of the task / feature AC5).
func TestCaptureDesignPlan_WordCountBoundsDocumented(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "capture-design-plan.md")
	body := readFile(t, path)

	// The bounds are documented as numeric tokens. Require both the soft-target
	// range anchor (300, 500) and the hard cap (1000) to appear.
	for _, n := range []string{"300", "500", "1000"} {
		if !strings.Contains(body, n) {
			t.Errorf("capture-design-plan.md missing word-count boundary %q", n)
		}
	}

	// The body must also name the soft/hard discipline so it is not a
	// coincidence of numerics — look for both "soft" and "hard".
	lower := strings.ToLower(body)
	for _, term := range []string{"soft", "hard"} {
		if !strings.Contains(lower, term) {
			t.Errorf("capture-design-plan.md missing word-count discipline term %q", term)
		}
	}
}

// TestCaptureDesignPlan_AppendOnlyAmendmentDocumented verifies the Rules
// section explains the append-only amendment discipline (task #660, Rules).
func TestCaptureDesignPlan_AppendOnlyAmendmentDocumented(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "capture-design-plan.md")
	body := readFile(t, path)

	for _, phrase := range []string{"append-only", "Tasks (created)"} {
		if !strings.Contains(body, phrase) {
			t.Errorf("capture-design-plan.md missing amendment-discipline phrase %q", phrase)
		}
	}
}

// TestCaptureDesignPlan_ListedInCatalogue verifies CATALOGUE.md mentions the
// new skill under the Reference category (task #660, AC5).
func TestCaptureDesignPlan_ListedInCatalogue(t *testing.T) {
	path := filepath.Join(repoRoot(t), "CATALOGUE.md")
	body := readFile(t, path)

	refIdx := strings.Index(body, "## Reference skills")
	if refIdx < 0 {
		t.Fatalf("CATALOGUE.md missing '## Reference skills' section")
	}
	// Isolate the Reference section so we verify the skill is listed *under*
	// Reference, not some other category.
	referenceSection := body[refIdx:]
	if nextHeading := strings.Index(referenceSection[1:], "\n## "); nextHeading >= 0 {
		referenceSection = referenceSection[:nextHeading+1]
	}
	if !strings.Contains(referenceSection, "capture-design-plan") {
		t.Errorf("CATALOGUE.md does not list capture-design-plan under Reference skills")
	}
}
