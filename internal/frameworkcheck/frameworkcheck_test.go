package frameworkcheck

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoRoot walks up from this test file until it finds go.mod and returns
// the containing directory. Tests rely on this to open the framework files
// (skills/, RULEBOOK.md) regardless of where `go test` is invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(self)
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repo root from %s", self)
	return ""
}

// readFile fails the test rather than returning an error so assertions stay
// flat.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// parseFrontmatter splits a skill file into its YAML frontmatter and body
// using the same fence rules the validator enforces.
func parseFrontmatter(t *testing.T, body string) (map[string]any, string) {
	t.Helper()
	if !strings.HasPrefix(body, "---\n") {
		t.Fatalf("file does not begin with '---' fence")
	}
	rest := body[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		t.Fatalf("file has no closing '---' fence")
	}
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(rest[:idx]), &fm); err != nil {
		t.Fatalf("YAML parse error: %v", err)
	}
	return fm, rest[idx+5:]
}

// forbiddenHarnessTokens lists tool and harness names that must never appear
// in any framework skill body added by feature #483. The list is deliberately
// narrow — it covers the specific names called out as forbidden in the
// feature acceptance criteria rather than every possible product name.
var forbiddenHarnessTokens = []string{
	"AskUserQuestion",
	"prompt_user",
	"Claude Code",
	"claude-code",
}

// containsAnyCaseInsensitive returns the first needle found in haystack using
// case-insensitive matching, or an empty string if none is present.
func containsAnyCaseInsensitive(haystack string, needles []string) string {
	lower := strings.ToLower(haystack)
	for _, n := range needles {
		if strings.Contains(lower, strings.ToLower(n)) {
			return n
		}
	}
	return ""
}

// TestAskUser_FrontmatterConformance verifies skills/ask-user.md declares the
// Reference-category frontmatter required by AC-13.
func TestAskUser_FrontmatterConformance(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "ask-user.md")
	body := readFile(t, path)
	fm, _ := parseFrontmatter(t, body)

	if got := fm["name"]; got != "ask-user" {
		t.Errorf("name: want %q, got %q", "ask-user", got)
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
	if !ok || len(loads) != 0 {
		t.Errorf("loads: want [], got %v", fm["loads"])
	}
}

// TestAskUser_HarnessNeutralBody verifies the ask-user body names no specific
// tool or harness per AC-13.
func TestAskUser_HarnessNeutralBody(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "ask-user.md")
	body := readFile(t, path)
	_, skillBody := parseFrontmatter(t, body)
	if hit := containsAnyCaseInsensitive(skillBody, forbiddenHarnessTokens); hit != "" {
		t.Errorf("forbidden tool/harness token %q found in skills/ask-user.md body", hit)
	}
}

// TestAskUser_CanonicalShapesPresent verifies the four canonical prompt
// shapes listed in task #484 appear in the body.
func TestAskUser_CanonicalShapesPresent(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "ask-user.md")
	body := readFile(t, path)
	wants := []string{
		"Confirm / revise",
		"Multi-choice selection",
		"Yes / not-now / later",
		"Name collision",
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("ask-user.md body missing canonical shape heading %q", w)
		}
	}
}

// TestAskUser_OptionConstraintsDocumented verifies the option-constraint
// rules required by task #484 (2–4 directed options, ~40-char labels, Other
// escape, fallback phrasing).
func TestAskUser_OptionConstraintsDocumented(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "ask-user.md")
	body := readFile(t, path)

	constraints := []string{
		"2 and 4",          // option count
		"40 characters",    // label length
		"Other",            // escape option mentioned
		"typed free-text",  // fallback phrasing
	}
	for _, c := range constraints {
		if !strings.Contains(body, c) {
			t.Errorf("ask-user.md body missing expected constraint phrase %q", c)
		}
	}
}

// TestSkillCreation_FrontmatterConformance verifies skills/skill-creation.md
// declares the Operation-category frontmatter required by task #486.
func TestSkillCreation_FrontmatterConformance(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "skill-creation.md")
	body := readFile(t, path)
	fm, _ := parseFrontmatter(t, body)

	if got := fm["name"]; got != "skill-creation" {
		t.Errorf("name: want %q, got %q", "skill-creation", got)
	}
	if got := fm["category"]; got != "Operation" {
		t.Errorf("category: want %q, got %q", "Operation", got)
	}
	if got, ok := fm["emits-exit-block"].(bool); !ok || got != false {
		t.Errorf("emits-exit-block: want false (Operation category), got %v", fm["emits-exit-block"])
	}
	if fm["exit-hands-to"] != nil {
		t.Errorf("exit-hands-to: want null (emits-exit-block false), got %v", fm["exit-hands-to"])
	}
	loads, ok := fm["loads"].([]any)
	if !ok {
		t.Fatalf("loads: want list, got %T", fm["loads"])
	}
	loadSet := map[string]bool{}
	for _, v := range loads {
		if s, ok := v.(string); ok {
			loadSet[s] = true
		}
	}
	for _, needed := range []string{"ask-user", "skill-categories"} {
		if !loadSet[needed] {
			t.Errorf("loads missing required entry %q", needed)
		}
	}
}

// TestSkillCreation_BodyCoversRequiredSections verifies the body covers the
// reactive + proactive procedures, classification rubric, placement logic,
// and name-collision handling required by task #486 and AC-1/3/4/5/6/7/8/
// 9/11.
func TestSkillCreation_BodyCoversRequiredSections(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "skill-creation.md")
	body := readFile(t, path)

	requiredHeadings := []string{
		"## Reactive Procedure",
		"## Proactive Procedure",
		"## Classification Rubric",
		"## Placement Logic",
		"## Name Collision Handling",
		"## Feedback Block",
	}
	for _, h := range requiredHeadings {
		if !strings.Contains(body, h) {
			t.Errorf("skill-creation.md missing required heading %q", h)
		}
	}

	// Proactive threshold must be explicit.
	if !strings.Contains(body, "three or more substantively-similar") {
		t.Errorf("skill-creation.md proactive threshold phrasing missing")
	}
	if !strings.Contains(body, "not counting retries") {
		t.Errorf("skill-creation.md retry-exclusion phrasing missing")
	}

	// Placement logic must reference the origin-remote detection.
	if !strings.Contains(body, "git remote get-url origin") {
		t.Errorf("skill-creation.md missing `git remote get-url origin` placement detection")
	}
	if !strings.Contains(body, "eddiecarpenter/gh-agentic") {
		t.Errorf("skill-creation.md missing framework-repo identity literal")
	}

	// Name-collision options.
	for _, opt := range []string{"Rename", "Overwrite", "Cancel"} {
		if !strings.Contains(body, opt) {
			t.Errorf("skill-creation.md name-collision option %q missing", opt)
		}
	}
}

// TestSkillCreation_NoDirectBuildCatalogueInvocation enforces AC-12:
// skill-creation must rely on session-init's mtime-based self-heal and must
// never invoke build-catalogue directly.
func TestSkillCreation_NoDirectBuildCatalogueInvocation(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "skill-creation.md")
	body := readFile(t, path)
	// The skill is permitted to *reference* build-catalogue.md to state the
	// non-invocation rule, but must not direct the agent to invoke it. Look
	// for imperative phrasing that would breach this.
	badPhrases := []regexp.Regexp{
		*regexp.MustCompile(`(?i)invoke\s+skills/build-catalogue`),
		*regexp.MustCompile(`(?i)call\s+skills/build-catalogue`),
		*regexp.MustCompile(`(?i)run\s+skills/build-catalogue`),
	}
	for _, r := range badPhrases {
		if r.MatchString(body) {
			t.Errorf("skill-creation.md contains forbidden direct-invocation phrase matching %s", r.String())
		}
	}
	// Positive assertion: the body must explicitly state reliance on
	// session-init self-heal (AC-12).
	if !strings.Contains(body, "session-init") {
		t.Errorf("skill-creation.md must reference session-init for catalogue self-heal")
	}
}

// TestSkillCreation_HarnessNeutralBody ensures AC-wide harness-neutrality
// extends to the skill-creation body too.
func TestSkillCreation_HarnessNeutralBody(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "skill-creation.md")
	body := readFile(t, path)
	_, skillBody := parseFrontmatter(t, body)
	if hit := containsAnyCaseInsensitive(skillBody, forbiddenHarnessTokens); hit != "" {
		t.Errorf("forbidden tool/harness token %q found in skill-creation.md body", hit)
	}
}

// TestSkillCategories_AllSkeletonsPresent verifies each of the six category
// skeletons appears in skill-categories.md (task #485).
func TestSkillCategories_AllSkeletonsPresent(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "skill-categories.md")
	body := readFile(t, path)

	if !strings.Contains(body, "## Category Skeletons") {
		t.Fatalf("skill-categories.md missing '## Category Skeletons' section")
	}

	for _, cat := range []string{"Session", "Recovery", "Bootstrap", "Operation", "Information", "Reference"} {
		heading := "### " + cat + " skeleton"
		if !strings.Contains(body, heading) {
			t.Errorf("skill-categories.md missing skeleton heading %q", heading)
		}
	}

	// Exit-block emission must be present in Session and Recovery skeletons;
	// must not appear in Bootstrap / Operation / Information / Reference
	// skeletons (the latter are permitted to say "do not emit an exit
	// block", which is a distinct phrase we exclude from the positive check).
	want := map[string]bool{
		"Session":     true,
		"Recovery":    true,
		"Bootstrap":   false,
		"Operation":   false,
		"Information": false,
		"Reference":   false,
	}
	for cat, shouldHaveExit := range want {
		section := sliceSkeleton(body, cat)
		if section == "" {
			t.Errorf("could not isolate %s skeleton", cat)
			continue
		}
		// Positive phrase = "Emit the canonical exit block". This is the
		// imperative the skeleton uses to direct the author to produce the
		// block. "Do not emit an exit block" in non-session skeletons is
		// explicitly not matched by this phrase.
		hasEmit := strings.Contains(section, "Emit the canonical exit block")
		if hasEmit != shouldHaveExit {
			t.Errorf("%s skeleton canonical exit-block emission: want %v, got %v", cat, shouldHaveExit, hasEmit)
		}
	}
}

// sliceSkeleton returns the text between the given category's "### <cat>
// skeleton" heading and the next "### " heading (exclusive). Returns an
// empty string if the heading is not present.
func sliceSkeleton(body, category string) string {
	start := strings.Index(body, "### "+category+" skeleton")
	if start < 0 {
		return ""
	}
	rest := body[start:]
	next := strings.Index(rest[1:], "\n### ")
	if next < 0 {
		return rest
	}
	return rest[:next+1]
}

// TestFeatureScoping_DelegatesToAskUser verifies the updated
// feature-scoping.md references skills/ask-user.md at the confirmation and
// selection moments required by task #487 and AC-14.
func TestFeatureScoping_DelegatesToAskUser(t *testing.T) {
	path := filepath.Join(repoRoot(t), "skills", "feature-scoping.md")
	body := readFile(t, path)
	fm, _ := parseFrontmatter(t, body)

	// Loads must now include ask-user.
	loads, ok := fm["loads"].([]any)
	if !ok {
		t.Fatalf("feature-scoping loads: want list, got %T", fm["loads"])
	}
	found := false
	for _, v := range loads {
		if s, ok := v.(string); ok && s == "ask-user" {
			found = true
		}
	}
	if !found {
		t.Errorf("feature-scoping.md loads must include 'ask-user'")
	}

	// The body must invoke skills/ask-user.md at least enough times to cover
	// the 11 documented invocation points in task #487 (seven artefacts,
	// deployment strategy plus downstream flag-name/reason, parking lot,
	// explicit trigger, impact-delta). Allow slack for phrasing — we
	// require at least 10 explicit invocations.
	count := strings.Count(body, "skills/ask-user.md")
	if count < 10 {
		t.Errorf("feature-scoping.md invokes skills/ask-user.md %d times; want at least 10", count)
	}

	// Rules section must acknowledge delegation — prevents future drift
	// where someone re-adds inline option rules.
	if !strings.Contains(body, "Interaction shape is delegated") {
		t.Errorf("feature-scoping.md Rules section must record that interaction shape is delegated to ask-user.md")
	}
}

// TestRulebook_ProactiveRulePresent verifies the single-pointer addition to
// RULEBOOK.md required by task #488 / AC-10.
func TestRulebook_ProactiveRulePresent(t *testing.T) {
	path := filepath.Join(repoRoot(t), "RULEBOOK.md")
	body := readFile(t, path)

	if !strings.Contains(body, "skills/skill-creation.md") {
		t.Fatalf("RULEBOOK.md missing pointer to skills/skill-creation.md")
	}

	// The bullet must reference "proactive-suggestion mode" or equivalent —
	// the rule is defined elsewhere, so we look for the pointer phrasing.
	if !strings.Contains(body, "proactive-suggestion mode") {
		t.Errorf("RULEBOOK.md missing 'proactive-suggestion mode' phrasing")
	}

	// The detail must NOT have leaked into RULEBOOK (AC-10).
	forbiddenInRulebook := []string{
		"three or more substantively-similar",
		"git remote get-url origin",
		"rename/overwrite/cancel",
		"~40 characters",
	}
	for _, phrase := range forbiddenInRulebook {
		if strings.Contains(body, phrase) {
			t.Errorf("RULEBOOK.md contains detail %q that must live in skill-creation.md only", phrase)
		}
	}
}

// TestRulebook_ProactiveRule_SizeCap verifies the proactive-rule addition
// stays under the ~2-line cap required by AC-10. We count lines in the
// specific bullet that starts with the proactive sentence.
func TestRulebook_ProactiveRule_SizeCap(t *testing.T) {
	path := filepath.Join(repoRoot(t), "RULEBOOK.md")
	body := readFile(t, path)

	// Extract the bullet. It starts with a line beginning with "- **If the
	// agent performs the same substantive action repeatedly" and ends at
	// the next blank line.
	startIdx := strings.Index(body, "- **If the agent performs the same substantive action repeatedly")
	if startIdx < 0 {
		t.Fatalf("RULEBOOK.md bullet not found — check the bullet prefix wording")
	}
	tail := body[startIdx:]
	endIdx := strings.Index(tail, "\n\n")
	if endIdx < 0 {
		endIdx = len(tail)
	}
	bullet := tail[:endIdx]

	// "~2 lines of body text" — allow up to 4 wrapped markdown lines so the
	// rule can wrap naturally at 80 cols but cannot silently grow into a
	// paragraph.
	lineCount := strings.Count(bullet, "\n") + 1
	if lineCount > 4 {
		t.Errorf("proactive-rule bullet is %d lines; AC-10 caps it at ~2 lines of body (allow up to 4 wrapped lines)", lineCount)
	}
}
