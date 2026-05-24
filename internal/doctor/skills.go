package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// --- schema constants ---

// requiredFrontmatterFields lists every field every skill must declare.
// The order matches skills/skill-categories.md and keeps missing-field errors
// reported in a stable order.
var requiredFrontmatterFields = []string{
	"name",
	"description",
	"category",
	"triggers",
	"loads",
	"emits-exit-block",
	"exit-hands-to",
}

// allowedCategories is the authoritative set per skills/skill-categories.md.
var allowedCategories = map[string]bool{
	"Session":     true,
	"Recovery":    true,
	"Bootstrap":   true,
	"Operation":   true,
	"Information": true,
	"Reference":   true,
}

// categoryOrderedList keeps the allowed category set ordered for deterministic
// error messages.
var categoryOrderedList = []string{
	"Session", "Recovery", "Bootstrap", "Operation", "Information", "Reference",
}

// skillNameRx constrains name values per the Anthropic-aligned rule.
var skillNameRx = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// reservedSkillNames must not be used as skill names per Anthropic's spec.
var reservedSkillNames = map[string]bool{
	"anthropic": true,
	"claude":    true,
}

// skillsDirCandidates lists the relative paths (from repo root) where skills
// can live. The first candidate that exists wins.
var skillsDirCandidates = []string{
	"skills",         // local skills (repo root skills/)
	".agents/skills", // framework skills synced via gh agentic repair
}

// --- frontmatter parsing ---

// frontmatterResult carries the parsed frontmatter and any validation errors.
type frontmatterResult struct {
	Path       string
	Violations []string // human-readable messages, one per violation
}

// findSkillsDir returns the absolute path to the skills directory that is
// present under root, and an empty string if none exist.
func findSkillsDir(root string) string {
	for _, rel := range skillsDirCandidates {
		p := filepath.Join(root, rel)
		if dirExists(p) {
			return p
		}
	}
	return ""
}

// enumerateSkillFiles returns the sorted list of skill files under skillsDir.
// Skills follow the canonical `<name>/SKILL.md` subdirectory layout; any
// top-level `*.md` files are also included for compatibility with the flat layout.
func enumerateSkillFiles(skillsDir string) ([]string, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			// Look for SKILL.md inside the subdirectory (canonical layout).
			candidate := filepath.Join(skillsDir, e.Name(), "SKILL.md")
			if _, statErr := os.Stat(candidate); statErr == nil {
				out = append(out, candidate)
			}
			continue
		}
		// Top-level .md files (flat layout, kept for compatibility).
		if strings.HasSuffix(e.Name(), ".md") {
			out = append(out, filepath.Join(skillsDir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// parseFrontmatter reads a skill file and returns the parsed YAML block plus
// any parse/format errors. The returned map is nil if parsing failed.
func parseFrontmatter(path string) (map[string]interface{}, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("read error: %v", err)}
	}
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return nil, []string{"frontmatter: missing opening '---' fence on line 1"}
	}
	rest := s[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return nil, []string{"frontmatter: missing closing '---' fence"}
	}
	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(rest[:idx]), &fm); err != nil {
		return nil, []string{fmt.Sprintf("frontmatter: YAML parse error: %v", err)}
	}
	if fm == nil {
		return nil, []string{"frontmatter: empty mapping"}
	}
	return fm, nil
}

// validateFrontmatter checks one parsed frontmatter map against the schema.
// Returns a list of human-readable violation messages. An empty list means
// the frontmatter is valid.
func validateFrontmatter(fm map[string]interface{}) []string {
	var violations []string

	// Missing required fields.
	for _, f := range requiredFrontmatterFields {
		if _, ok := fm[f]; !ok {
			violations = append(violations, frontmatterMissingMsg(f))
		}
	}

	// Unknown fields.
	for k := range fm {
		if !isAllowedField(k) {
			violations = append(violations, fmt.Sprintf(
				"frontmatter: unknown field %q\n  expected: only [%s]\n  see: skills/skill-categories.md for the schema",
				k, strings.Join(requiredFrontmatterFields, ", "),
			))
		}
	}

	// Per-field validation.
	if name, ok := fm["name"].(string); ok {
		if !skillNameRx.MatchString(name) {
			violations = append(violations,
				fmt.Sprintf("frontmatter: name %q does not match [a-z0-9-]{1,64}\n  expected: lowercase letters, numbers, hyphens only; 1–64 chars\n  see: skills/skill-categories.md for the name rules",
					name))
		}
		if reservedSkillNames[name] {
			violations = append(violations,
				fmt.Sprintf("frontmatter: name %q is reserved by Anthropic\n  expected: any name other than 'anthropic' or 'claude'\n  see: skills/skill-categories.md for the name rules",
					name))
		}
	} else if _, present := fm["name"]; present {
		violations = append(violations, "frontmatter: name must be a string")
	}

	if desc, ok := fm["description"].(string); ok {
		if desc == "" {
			violations = append(violations, "frontmatter: description is empty\n  expected: non-empty, 1–1024 chars\n  see: skills/skill-categories.md for the description rules")
		} else if len(desc) > 1024 {
			violations = append(violations,
				fmt.Sprintf("frontmatter: description length %d exceeds 1024\n  expected: 1–1024 chars\n  see: skills/skill-categories.md for the description rules",
					len(desc)))
		}
	} else if _, present := fm["description"]; present {
		violations = append(violations, "frontmatter: description must be a string")
	}

	if cat, ok := fm["category"].(string); ok {
		if !allowedCategories[cat] {
			violations = append(violations,
				fmt.Sprintf("frontmatter: category %q is not allowed\n  expected: one of [%s]\n  see: skills/skill-categories.md for category definitions",
					cat, strings.Join(categoryOrderedList, ", ")))
		}
	} else if _, present := fm["category"]; present {
		violations = append(violations, fmt.Sprintf(
			"frontmatter: category must be a string\n  expected: one of [%s]\n  see: skills/skill-categories.md for category definitions",
			strings.Join(categoryOrderedList, ", "),
		))
	}

	// emits-exit-block must be a bool; exit-hands-to must be consistent with it.
	var eb bool
	var ebOK bool
	if v, ok := fm["emits-exit-block"]; ok {
		eb, ebOK = v.(bool)
		if !ebOK {
			violations = append(violations, "frontmatter: emits-exit-block must be a boolean (true or false)")
		}
	}
	cat, _ := fm["category"].(string)
	if ebOK && cat != "" && allowedCategories[cat] {
		shouldEmit := cat == "Session" || cat == "Recovery"
		if eb != shouldEmit {
			violations = append(violations,
				fmt.Sprintf("frontmatter: emits-exit-block=%v is inconsistent with category=%s\n  expected: %v\n  see: skills/skill-categories.md for the consistency rules",
					eb, cat, shouldEmit))
		}
	}
	if v, ok := fm["exit-hands-to"]; ok {
		if ebOK {
			if eb {
				if v == nil {
					violations = append(violations, "frontmatter: exit-hands-to must be a non-empty string when emits-exit-block is true\n  see: skills/skill-categories.md for the consistency rules")
				} else if s, isStr := v.(string); !isStr || s == "" {
					violations = append(violations, "frontmatter: exit-hands-to must be a non-empty string when emits-exit-block is true\n  see: skills/skill-categories.md for the consistency rules")
				}
			} else {
				if v != nil {
					violations = append(violations, "frontmatter: exit-hands-to must be null when emits-exit-block is false\n  see: skills/skill-categories.md for the consistency rules")
				}
			}
		}
	}

	// triggers must be a string or a list of strings.
	if v, ok := fm["triggers"]; ok {
		if !isStringOrStringList(v) {
			violations = append(violations, "frontmatter: triggers must be a string or a list of strings")
		}
	}

	// loads must be a list (possibly empty) of strings.
	if v, ok := fm["loads"]; ok {
		if !isNilOrStringList(v) {
			violations = append(violations, "frontmatter: loads must be a list of strings (use [] for none)")
		}
	}

	return violations
}

// frontmatterMissingMsg produces a uniform "missing field" violation line that
// also points the user at the schema location.
func frontmatterMissingMsg(field string) string {
	return fmt.Sprintf("frontmatter: %s field is missing\n  expected: every skill frontmatter must declare %s\n  see: skills/skill-categories.md for the schema",
		field, field)
}

func isAllowedField(k string) bool {
	for _, f := range requiredFrontmatterFields {
		if f == k {
			return true
		}
	}
	return false
}

func isStringOrStringList(v interface{}) bool {
	switch t := v.(type) {
	case string:
		return true
	case []interface{}:
		for _, el := range t {
			if _, ok := el.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isNilOrStringList(v interface{}) bool {
	if v == nil {
		return true
	}
	list, ok := v.([]interface{})
	if !ok {
		return false
	}
	for _, el := range list {
		if _, ok := el.(string); !ok {
			return false
		}
	}
	return true
}

// --- check functions ---

// checkSkillFrontmatter scans the active skills directory and emits one
// CheckResult per skill (Pass on valid, Fail on any violation).
//
// Skills are looked up under deps.Root via findSkillsDir. If no skills
// directory exists at all, the check emits a single Warning — downstream repos
// without skills are allowed.
//
// When the resolved skills directory is inside .agents/ (framework files synced
// by gh agentic repair), validation is skipped entirely. Framework files are
// read-only and trusted; validating them in domain repos adds noise without value.
func checkSkillFrontmatter(deps CheckDeps) Group {
	g := Group{Name: "Skills"}

	skillsDir := findSkillsDir(deps.Root)
	if skillsDir == "" {
		g.Results = append(g.Results, CheckResult{
			Name: "skills-frontmatter", Status: Warning,
			Message: "skills/ not found — no frontmatter to validate",
		})
		return g
	}

	// Framework skills (.agents/skills) are managed by the framework and trusted
	// as-is — skip per-file validation so domain repos don't see spurious warnings.
	if rel, err := filepath.Rel(deps.Root, skillsDir); err == nil &&
		strings.HasPrefix(rel, ".agents") {
		g.Results = append(g.Results, CheckResult{
			Name:    "skills-frontmatter",
			Status:  Pass,
			Message: fmt.Sprintf("framework skills (%s) — validation skipped (read-only framework files)", rel),
		})
		return g
	}

	files, err := enumerateSkillFiles(skillsDir)
	if err != nil {
		g.Results = append(g.Results, CheckResult{
			Name: "skills-frontmatter", Status: Fail,
			Message:     fmt.Sprintf("skills: cannot enumerate %s: %v", skillsDir, err),
			Remediation: "Verify the skills directory is readable",
		})
		return g
	}
	if len(files) == 0 {
		g.Results = append(g.Results, CheckResult{
			Name: "skills-frontmatter", Status: Warning,
			Message: fmt.Sprintf("%s is empty — no skills to validate", relDisplay(deps.Root, skillsDir)),
		})
		return g
	}

	totalViolations := 0
	for _, path := range files {
		display := relDisplay(deps.Root, path)
		fm, parseErrs := parseFrontmatter(path)
		var violations []string
		if len(parseErrs) > 0 {
			violations = parseErrs
		} else {
			violations = validateFrontmatter(fm)
		}
		if len(violations) == 0 {
			g.Results = append(g.Results, CheckResult{
				Name: display, Status: Pass,
				Message: fmt.Sprintf("%s frontmatter valid", display),
			})
			continue
		}
		totalViolations += len(violations)
		for _, v := range violations {
			g.Results = append(g.Results, CheckResult{
				Name: display, Status: Fail,
				Message:     fmt.Sprintf("%s\n  %s", display, v),
				Remediation: "Run 'gh agentic check' to see all skill frontmatter issues; fix per skills/skill-categories.md",
			})
		}
	}

	_ = totalViolations // counted via Fail results in the Report
	return g
}

// relDisplay returns a path relative to root for user-facing messages,
// falling back to the absolute path if relative resolution fails.
func relDisplay(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}
