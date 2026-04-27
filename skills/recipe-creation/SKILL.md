---
name: recipe-creation
description: Creates or updates a Goose recipe under a thin-shell discipline that prevents recipes from duplicating skill content. Validates that recipe instructions are a one-line pointer at the canonical SKILL.md and refuses to write recipe YAML containing numbered steps, inline gh / git commands, decision logic, or any other playbook content. Audits the existing recipes/ tree for the same. Use when a human is creating a new recipe for an existing skill, updating a recipe that has drifted, or auditing the whole recipe tree. Use even when the caller doesn't say "recipe-creation" — phrases like "create a new recipe for the dev session", "lint the recipes for inline steps", "update the feature-design recipe to point at the skill" should trigger this skill.
triggers: human-interactive
user-invocable: true
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/step-skip-rule.md
  - skills/prompt-user/SKILL.md
emits-exit-block: true
exit-hands-to: "human: review the recipe diff, commit and push; the workflow uses the recipe on the next pipeline trigger"
---

# Recipe Creation

## Goal

Govern the creation and update of Goose recipes (`recipes/*.yaml`)
so they stay as **thin shells** that delegate playbook content to
the corresponding `skills/<name>/SKILL.md` file. Recipes that
contain inline steps drift from the skills; this skill exists
because that drift caused the previous round of "I changed the
skill but the behaviour didn't shift" pain.

The thin-shell rule:

- A recipe carries Goose-specific configuration: `version`,
  `title`, `description`, `parameters`, `extensions`, `settings`.
- A recipe's `instructions:` block does ONE thing: tell Goose to
  follow the playbook in `skills/<name>/SKILL.md`. Plus the
  minimum boilerplate to resolve the active repo and parameter
  bindings.
- Anything else — numbered steps, inline `gh` commands, label
  transitions, decision trees, verification gates, error
  handling — belongs in the SKILL.md, not the recipe.

This skill writes recipes that obey the rule and refuses to write
recipes that don't. It also audits the existing recipe tree.

## Output Artefacts

Per invocation, one of:

- **Created.** A new file at `recipes/<name>.yaml` conforming to
  the thin-shell template. The corresponding `.goose/recipes/`
  copy is updated by `gh agentic mount` on the next sync, not by
  this skill.
- **Updated.** An existing `recipes/<name>.yaml` slimmed to the
  thin-shell shape. Inline content that was in the recipe is
  surfaced for the human to migrate into the SKILL.md (this skill
  does NOT touch SKILL.md files; that is the human's call).
- **Audited.** A report listing every recipe under `recipes/` with
  its compliance status against the thin-shell rule. No file
  changes.

The skill's three valid terminal outputs:

**A. Created.** New thin-shell recipe written.

**B. Updated.** Existing recipe slimmed; surplus content surfaced
to the human as a migration prompt.

**C. Audit.** Compliance report only; no file changes. Exits with
the count of compliant vs non-compliant recipes.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy for
  `RECIPE_NOT_FOUND`, `RECIPE_HAS_INLINE_STEPS`, `SKILL_NOT_FOUND`,
  `INVALID_RECIPE_NAME`.
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement.

## Dependencies

- `skills/prompt-user/SKILL.md` — used at mode-pick (step 2),
  pre-write confirmation (step 9), and the audit-mode "want to
  drill into a non-compliant recipe?" prompt.

This skill makes no GitHub API calls and no git operations beyond
reading the current branch.

## Steps

The **step-skip rule** applies. Mode-gated carve-out: Create-only
steps are not run when in Update or Audit mode and vice versa.

**Branch-safety check.** The skill writes files; it MUST NOT run
on `main`. Refuse at entry with a clear remediation message,
mirroring the rule from `solution-architecture` and
`foreground-recovery`.

---

### Section A — Setup and mode pick

1. **Announce the session.** Print the banner verbatim before any
   tool call:

   ```
   ==========================================================
   === Recipe Creation — Started                              ===
   ==========================================================
   This skill governs Goose recipes (recipes/*.yaml) under the
   thin-shell rule: recipes delegate to skills, never duplicate
   their content.
   ==========================================================
   ```

   Run the branch check:

   ```bash
   git branch --show-current
   ```

   - `main` / `master` → exit cleanly with the remediation:
     "Switch to a branch first; this skill writes files and won't
     run on main."
   - Otherwise → continue.

2. **Pick mode.**

   ```
   prompt-user(
     question: "What do you want to do?",
     header: "Recipe operation",
     options: [
       {label: "Create new recipe",
        description: "New recipe for an existing skill."},
       {label: "Update existing recipe",
        description: "Slim or correct a recipe; surface inline steps for migration."},
       {label: "Audit recipes",
        description: "Lint every recipe under recipes/; report compliance."},
       {label: "Cancel",
        description: "Exit without changes."}
     ]
   )
   ```

   Branch on the answer. Cancel → exit cleanly.

---

### Section B — Lint rules (shared by all modes)

**These rules define what counts as "thin shell". Used by Create
to template, by Update to slim, by Audit to report.**

A recipe is **compliant** when:

1. Top-level keys are limited to the canonical set:
   `version`, `title`, `description`, `parameters`, `extensions`,
   `settings`, `instructions`.
2. `instructions:` block content fits the canonical template
   (step 6 below). Specifically the `instructions:` block:
   - References the SKILL.md by path exactly once.
   - Contains AT MOST these sections, in this order:
     - One-line role statement.
     - Parameter resolution (≤ 5 lines).
     - Active-repo resolution boilerplate (≤ 3 lines).
     - "Follow the playbook in skills/<name>/SKILL.md" pointer.
     - "Do NOT improvise steps" reminder.

A recipe is **non-compliant** when its `instructions:` block
contains ANY of the following anti-patterns:

- **Numbered step markers**: lines matching `^\s*\d+\.\s`,
  `^Step \d+`, `^## Step`, `^### Step`.
- **Inline `gh` commands** beyond the active-repo resolver:
  any `gh issue create`, `gh issue edit`, `gh issue close`,
  `gh issue list`, `gh label`, `gh pr create`, `gh pr view`,
  `gh api`, `gh release`.
- **Inline git operations** beyond status checks: `git commit`,
  `git checkout`, `git push`, `git merge`, `git branch -D`,
  `git fetch origin <branch>`.
- **Decision keywords** in agent-instruction context: lines
  containing `If <condition> then`, `Branch on`, `raise <ERROR>`,
  `verify the result`, `loop until passing`.
- **Sub-headings beyond context-only**: `## Steps`, `## Verification`,
  `## Error Handling`, `## Acceptance Criteria`,
  `## Output Artefacts`, `## State Model`, `## Definitions`.
- **Per-artefact prompts** or `prompt-user(` invocations.
- **Skill-internal terminology**: anti-fabrication clause,
  step-skip rule, per-revision diff, impact-delta — these are
  skill-level disciplines and must live in the SKILL.md.

The presence of ANY anti-pattern flags the recipe as
non-compliant.

---

### Section C — Create mode (only when "Create new" was picked)

3. **Get the recipe name. (create only)** Ask the human for the
   skill name to wrap. The recipe name MUST match the skill name
   (one recipe per skill; one skill per recipe is the convention).

   ```
   prompt-user(
     question: "Which skill is this recipe for?",
     header: "Skill name",
     options: [...one option per existing skill in skills/...] +
       [{label: "Other (free-text)", ...}]
   )
   ```

   On free-text, validate: the name MUST match
   `skills/<name>/SKILL.md` (case-sensitive, kebab-case). If not
   → raise `SKILL_NOT_FOUND` (`ERROR`); recipes can only wrap
   existing skills.

   Hold as `<name>`.

4. **Check for existing recipe. (create only)** If
   `recipes/<name>.yaml` already exists → re-route to Update mode
   (step 7). Don't overwrite via Create.

5. **Read the skill's frontmatter. (create only)** Open
   `skills/<name>/SKILL.md` and extract:
   - `name` (sanity-check matches)
   - `description` (used as recipe `description`)
   - `triggers` (used to set `requirement: required` vs `optional`
     on parameters)
   - any frontmatter fields useful for the recipe shape

   Hold as `<frontmatter>`.

6. **Generate the recipe YAML. (create only)** Apply the canonical
   template:

   ```yaml
   version: "1.0.0"
   title: "<Human-readable session name from <frontmatter>.description>"
   description: "<Single-sentence rephrase of <frontmatter>.description>. Triggered by <how the workflow fires this>."

   parameters:
     # Most recipes need at least one parameter (e.g. issue_number).
     # Read the SKILL.md's "Inputs" or first step to figure out what
     # the agent needs at session start. Each parameter is one entry:
     - key: <name>
       input_type: <string | number>
       requirement: required
       description: "<one-liner on what this parameter is>"

   extensions:
     - type: builtin
       name: developer
       display_name: Developer
       timeout: 600
       bundled: true

   settings:
     max_turns: 100

   instructions: |
     You are running the <skill-display-name> session.
     Follow the playbook in `skills/<name>/SKILL.md` — read it in
     full at session start and execute it. Do NOT improvise steps;
     the skill is the authoritative playbook.

     Parameter binding: <name the parameters from above and how
     the skill consumes them>.

     Resolve the active repo once via:
       gh repo view --json nameWithOwner -q .nameWithOwner

     If `docs/ARCHITECTURE.md` exists, the skill will read it; the
     recipe does not.
   ```

   Walk the lint rules (Section B) over the generated YAML to
   confirm compliance before write. Any anti-pattern flagged →
   the agent has miswritten the template; surface and stop.

7. **Pre-write confirmation. (create only)** Render the generated
   YAML in a fenced markdown block, then `prompt-user`:

   ```
   prompt-user(
     question: "Write this recipe?",
     header: "recipes/<name>.yaml",
     options: [
       {label: "Yes, write it",
        description: "Write the file and exit; you can commit + push manually."},
       {label: "Revise",
        description: "Tell me what to change."},
       {label: "Cancel",
        description: "Exit without writing."}
     ]
   )
   ```

   - Yes → step 8.
   - Revise → free-text; cap 5 revisions; loop.
   - Cancel → exit cleanly.

8. **Write the file. (create only)** Use the agent's `Write` tool
   to write `recipes/<name>.yaml`. Surface the next-action
   pointer:

   ```
   Recipe written to recipes/<name>.yaml.
   Next:
     - Run `gh agentic mount <version>` to sync to .goose/recipes/.
     - Verify the workflow YAML triggers this recipe correctly.
     - Commit and push when ready.
   ```

   Continue to Section F.

---

### Section D — Update mode (only when "Update existing" was picked)

9. **Get the recipe name. (update only)** List `recipes/*.yaml`
   files via `prompt-user`; the human picks. On selection, hold
   as `<name>` and read `recipes/<name>.yaml`.

   On missing file → raise `RECIPE_NOT_FOUND` (`ERROR`); exit.

10. **Lint the existing recipe. (update only)** Apply the rules
    from Section B against the file's `instructions:` block.
    Build a list `<violations>` of every anti-pattern detected
    with line numbers and verbatim quotes.

11. **Render the lint report. (update only)** Display:

    ```
    Recipe: recipes/<name>.yaml

    Compliant: <yes | no>

    Violations:
      - Line <N>: <category — quoted excerpt>
      - ...

    Total violations: <count>
    ```

    If `<violations>` is empty → "Already compliant. Nothing to
    do." Exit cleanly.

12. **Migrate-or-keep prompt. (update only)** For each violation,
    ask the human (in batch or one-at-a-time, agent's choice)
    whether to:

    - **Move to skill** — the agent surfaces the inline content
      for the human to copy into the corresponding SKILL.md.
      The recipe is slimmed by removing the violation.
    - **Discard** — the inline content is dropped from the recipe
      with no migration (acceptable when the content was already
      in the SKILL.md, just duplicated in the recipe).
    - **Keep** — the human disputes the violation and wants to
      keep it. Allowed but noted in the exit block; the recipe
      remains non-compliant.

    Two-pass approach:
    - First pass: agent classifies each violation as "obvious
      duplicate of skill" (suggest Discard) or "skill-level
      content not yet in skill" (suggest Move).
    - Second pass: human reviews and accepts/overrides per
      violation.

13. **Generate the slimmed recipe. (update only)** Apply the
    accepted Move/Discard decisions; render the new YAML.

14. **Surface the migration content. (update only)** For all
    "Move to skill" decisions, output the content the human needs
    to add to `skills/<name>/SKILL.md`:

    ```
    Migration to skills/<name>/SKILL.md:

    The following content was in the recipe and the human chose
    "Move to skill". Copy it into the SKILL.md (typically into
    the Steps section); this skill will NOT modify SKILL.md
    automatically.

    --- begin migrate ---
    <content per violation>
    --- end migrate ---
    ```

15. **Pre-write confirmation. (update only)** Same shape as step
    7: render slimmed YAML, ask Confirm / Revise / Cancel. On
    Confirm, write the slimmed file. Continue to Section F.

---

### Section E — Audit mode (only when "Audit" was picked)

16. **Walk the recipe tree. (audit only)** List every file under
    `recipes/*.yaml`. For each, apply the lint (Section B), record
    `{ name, compliant: bool, violation_count, violations[] }`.

17. **Render the report. (audit only)** Output:

    ```
    Recipe Audit — <date>

    Compliant (<count>/<total>):
      ✓ <name>
      ✓ <name>

    Non-compliant (<count>/<total>):
      ✗ <name> (<violation_count> violations)
        - Top categories: <numbered-steps, gh-commands, ...>
      ...

    Recommended next:
      - Run /recipe-creation against each non-compliant recipe in
        Update mode to slim it.
    ```

18. **Drill-in prompt. (audit only)** If any non-compliant recipes
    were found:

    ```
    prompt-user(
      question: "Drill into a non-compliant recipe now?",
      header: "Audit drill-in",
      options: [
        ...one option per non-compliant recipe (capped at 4 — if
           more, use a free-text fallback)...,
        {label: "No — exit", description: "I'll come back to these."}
      ]
    )
    ```

    On selection, route to step 9 (Update mode) with that recipe.
    On No-exit → continue to Section F.

---

### Section F — Closeout

19. **Emit the exit block.** Match the actual outcome:

    **Output A — Created:**
    ```
    === Recipe Creation — Created ===

    Produced: recipes/<name>.yaml

    Next:
      - gh agentic mount <version> to sync to .goose/recipes/
      - Commit + push when ready
      - Verify the workflow YAML invokes this recipe
    ```

    **Output B — Updated:**
    ```
    === Recipe Creation — Updated ===

    Produced: recipes/<name>.yaml (slimmed)

    Violations resolved: <count>
      - Moved to skill: <count>
      - Discarded as duplicate: <count>
      - Kept (recipe remains non-compliant on these): <count>

    Migration content for the human to apply to
    skills/<name>/SKILL.md was surfaced above.

    Next: commit + push when ready
    ```

    **Output C — Audited:**
    ```
    === Recipe Creation — Audit ===

    Inspected: <total> recipes
    Compliant: <count>
    Non-compliant: <count>

    See report above for per-recipe details.
    ```

20. **Terminate the session.** Per `emits-exit-block: true`,
    invoke the host runtime's session-close API if available;
    otherwise halt.

## Verification

Run the framework checks against this skill:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/recipe-creation/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/recipe-creation/SKILL.md
```

Pass criteria: both commands exit 0.

### Mechanical checks

Run by `verify-skill-mechanical.py`:

- `all_sections_present`, `frontmatter_required_fields`,
  `frontmatter_name_valid`, `description_within_length_limit`,
  `description_assertive`, `description_third_person`,
  `references_resolve`.

### Ground-truth checks

Run by `check-description-triggers.py`:

- `description_triggers_appropriately` — phrasings classified per
  the `GROUND_TRUTH` entry for `recipe-creation`.

### Self-application

A future enhancement: a Go-side or Python-side lint that re-applies
the Section B rules to every `recipes/*.yaml` and fails CI on
non-compliance. Out of scope for this skill itself; tracked as a
candidate Requirement.

## Error Handling

- `INVALID_RECIPE_NAME` from step 3 (chosen name is not
  kebab-case or contains path separators) → severity `ERROR`;
  exit. Recipes follow the same naming as skills.
- `SKILL_NOT_FOUND` from step 3 (`skills/<name>/SKILL.md` does
  not exist) → severity `ERROR`; exit. Recipes wrap existing
  skills only — there is no "create a recipe for a skill that
  doesn't exist yet" path.
- `RECIPE_NOT_FOUND` from step 9 (Update mode, file missing) →
  severity `ERROR`; exit. The human can re-invoke in Create mode
  if appropriate.
- `RECIPE_HAS_INLINE_STEPS` is the categorisation used in the
  lint report; not a propagated error code. The skill never
  refuses to write a recipe the human explicitly chose to keep
  non-compliant — but it does surface the kept violations clearly
  in the exit block.
- All other errors: propagate (default).
