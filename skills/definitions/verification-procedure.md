# Verification Procedure

Defines the term **change-pinning** and the procedure that follows
from it for writing verification checks — mechanical or semantic.

## The term

A verification check is **change-pinning** when its pass/fail outcome
is bound to the specific change the skill makes. Concretely: if you
revert the change, the check must fail. If the check would pass
against both the post-change state and the pre-change state, it is
not change-pinning — it validates that something exists, not that
the right something exists.

Change-pinning is what distinguishes a check that confirms work was
done from one that merely confirms work was attempted.

## The rule

**A verification check must fail if the change described by the
skill is reverted or materially modified.**

This rule is the operational form of the change-pinning property —
it is how an author confirms a check carries the property in
practice.

## Mechanical: change-pinning by content

Weak (existence only):

```bash
test -f docs/github-app-setup.md
```

Passes whether the file is empty, wrong, or correct. Reverting the
change leaves the file in place — the check still passes.

Strong (anchored on content):

```bash
grep -qF "Every autonomous phase must publish its plan" RULEBOOK.md
```

Anchored on a specific phrase introduced by the change. Reverting
removes the phrase — the check fails.

### Heuristics for strong mechanical checks

- Use `grep -qF` (fixed-string, quiet) anchored on a phrase
  introduced *by this change* — not a phrase that already existed.
- For schema changes, assert the new field/column/route is present
  by name *and* that a sample value parses correctly.
- For workflow changes, assert the new step's name appears in the
  YAML *and* that the step has the expected `uses:` or `run:` body.
- For deletion changes, assert the removed thing is absent (the
  inverse — `! grep`).
- For state transitions, query the resulting state directly (label
  applied, branch pushed, comment posted) — never just the absence
  of an error.

### Anti-patterns

- `test -f path` alone — file may exist for unrelated reasons.
- `grep -q PATTERN` where `PATTERN` already existed in the file
  before this change.
- Asserting an error-free exit from a command whose normal output
  has not been inspected.
- Asserting a count (`wc -l ...`) without anchoring on what the
  lines should contain.

## Semantic: catching intent drift

Semantic checks have an analogous discipline. The agent's assessment
must catch *intent drift*, not just surface compliance.

A semantic check that always returns "looks fine" regardless of
skill quality is failing its purpose, the same way a `test -f`
mechanical check would. To avoid this:

- Frame each semantic check as "what specifically could go wrong here
  — and would the check surface it?".
- The check's prompt should ask the agent for a structured judgement
  with reasoning, not a yes/no.
- The check's prompt should describe the *failure shape* the
  evaluator is looking for, so the agent knows what to flag rather
  than rationalise.

### Example: weak vs strong semantic check

Weak:

> "Read this skill and tell me if the steps make sense."

The agent will almost always say yes. The check has no failure
criterion.

Strong:

> "Read this skill cold. For each step, identify whether a fresh
> agent would (a) know exactly what to do, (b) need to guess, or
> (c) be likely to skip the step under execution load. Report one
> line per step with the verdict and a one-sentence reason."

The agent is now told what failure looks like and structured to
report it. The same skill that 'looks fine' under the weak prompt
shows its gaps under the strong one.

## Reverting test

The strongest gut-check for any verification check:

> If I revert exactly the change this skill makes, would this check
> still pass?

If yes — the check is not change-pinning. Strengthen it before
declaring the skill done.

## Where this rule applies

- All checks in a skill's `## Verification` section.
- All assertions in a skill's eval prompts under
  `skills/evals/<name>.json`.
- Mechanical checks implemented in
  `skills/skill-creator/scripts/verify-skill-mechanical.py`.

It does not apply to:

- Pre-flight environment checks (e.g., "is `gh` installed") — these
  are setup verification, not change verification.
- Logging/telemetry assertions — these are observability, not
  fidelity.

## Mechanical checks (universal)

`verify-skill-mechanical.py` runs the following checks on every skill.
Skills do not restate these — run the script directly:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/<name>/SKILL.md
```

- `all_sections_present` — every mandatory section heading exists
  (Goal, Output Artefacts, Definitions, Dependencies, Steps,
  Error Handling).
- `frontmatter_required_fields(name, description, triggers, loads)`.
- `frontmatter_name_valid` — kebab-case, matches the file's
  parent-directory name.
- `description_within_length_limit` — ≤ 1024 chars.
- `description_assertive` — contains "Use when" plus an assertive
  clause directing when the skill should fire.
- `description_third_person` — written about the skill, not in
  first-person voice.
- `references_resolve` — every `loads:` path resolves to an
  existing file.

### Skill-specific extensions

A skill MAY append a "Skill-specific extension" subsection after
the canonical stanza when it has additional verification beyond the
framework checks (e.g., a CI-enforced sync test, a domain-specific
fixture, a benchmark threshold). Format:

```markdown
### Skill-specific extension

<one paragraph naming the extra check, what it covers, and how to
run it. Followed by the command in a fenced code block.>

\`\`\`bash
<command>
\`\`\`

Pass criteria: <when this check passes>.
```

Examples in the live framework:

- `gh-agentic/SKILL.md` adds the `TestGhAgenticToolSkillCoversCLI`
  Go test as a CI-enforced sync check.
- `skill-creator/SKILL.md` adds skill-creator-specific mechanical
  bullets that don't apply to other skills.

The extension is *additive* — never copy the universal-check bullets
into the skill-specific extension. If a check applies to every
skill, it lives in this definition; if it applies only to one
skill, it goes in that skill's extension.
