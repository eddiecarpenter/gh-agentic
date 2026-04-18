---
name: skill-categories
description: Authoritatively defines the six-category skill taxonomy (Session, Recovery, Bootstrap, Operation, Information, Reference) and the YAML frontmatter schema every skill must conform to. Use when authoring a new skill, classifying an existing skill, validating frontmatter, or reasoning about which exit protocol applies to a skill.
category: Reference
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# Skill Categories and Frontmatter Schema

## Purpose

This file is the **single source of truth** for the skill taxonomy and the YAML
frontmatter schema. No other file in the framework may redefine or contradict
these categories or fields. If a change is needed, change it here first and let
everything else follow.

## Design Principles

Two principles drove this design and must be preserved when extending it.

1. **Token-cost first.** `RULEBOOK.md` is always loaded (every session, forever)
   and skills are on-demand. Detail about categories, exit protocol, and
   frontmatter lives in this file — not in RULEBOOK. RULEBOOK only names the
   taxonomy and points here.
2. **Optimise for the agent.** Rules and skills are structured, unambiguous, and
   machine-parseable (YAML frontmatter, enumerated values, explicit schemas).
   Human readability is a floor, not a ceiling: when clarity for the agent and
   decoration for the human are in tension, the agent wins.

## The Six Categories

Every skill belongs to exactly one category. The category determines lifecycle,
exit protocol, and classification criteria.

| Category | Purpose | Lifecycle | Exit Protocol | Classification Criteria |
|---|---|---|---|---|
| **Session** | Drive a complete pipeline phase end-to-end (requirements, scoping, design, development, review, issue/bug). | Entered at session start, runs to completion or blocked state, terminates the session on exit. | **Emits exit block.** After the exit block the session terminates — the agent must not continue. If a session-close API is available, invoke it. | The skill drives an entire pipeline phase, produces a defined artefact, and corresponds to a row in the Session Types table in `RULEBOOK.md`. |
| **Recovery** | Provide the human-controlled escape hatch when the automated pipeline is blocked. | Entered when the pipeline cannot self-heal, runs interactively, terminates the session on exit. | **Emits exit block.** A recovery session is a session — when it emits its exit block, the session ends. | The skill is invoked by a human (or by automation handing off to a human) to resolve a blocked pipeline state and does not itself produce a pipeline artefact other than a fix or an unblocking action. |
| **Bootstrap** | Prepare the environment at session start or after a framework sync so the rest of the session can run correctly. | Entered before any other skill in a session, runs to completion, returns control silently. | **No exit block.** Bootstrap skills do not terminate sessions — they return control to whatever invoked them. | The skill is invoked at the start of every session (or after a template sync) to load context, validate health, mount the framework, or otherwise prepare the runtime. |
| **Operation** | Perform a self-contained procedure that produces a specific outcome (generate an artefact, run a diagnostic, publish a release, rebuild an index). | Invoked on demand, runs to completion, returns control silently. | **No exit block.** Operations are not sessions — they return their result to the caller. | The skill has a defined input → output procedure, is not a pipeline phase, and does not terminate the caller's session. |
| **Information** | Surface context to the agent or the user (notifications, status summaries, release notes). | Invoked on demand when information must be produced, runs to completion, returns control silently. | **No exit block.** Information skills produce output, not handover. | The skill's primary output is information (to the user, to the agent, or to a downstream artefact) and it does not itself run a pipeline phase or mutate pipeline state. |
| **Reference** | Define rules, templates, schemas, or patterns that other skills follow. | Read by the agent when another skill needs the rule or template; never executed as a session. | **No exit block.** Reference skills are never session-terminating. | The skill is an authoritative definition — a schema, a template, a command reference, or a rule book — and is read (not run) by other skills and by the agent. |

## Frontmatter Schema

Every skill in the framework **must** declare YAML frontmatter between `---`
markers at the top of its markdown file. The schema below is exhaustive: every
field is listed; unknown fields are not permitted.

### Field reference

| Field | Required | Type | Allowed values | Description |
|---|---|---|---|---|
| `name` | Yes | string | `^[a-z0-9-]{1,64}$` — lowercase letters, numbers, hyphens; 1–64 chars; must not contain reserved words `anthropic` or `claude`; must not contain XML tags. | Anthropic-aligned canonical field. Identifies the skill. Typically matches the filename without the `.md` extension. |
| `description` | Yes | string | Non-empty; 1–1024 chars; no XML tags; **third person**; **trigger-oriented** (states both *what* the skill does and *when* to invoke it; end with a "Use when …" clause). | Anthropic-aligned canonical field. Claude uses this to decide when to invoke the skill. |
| `category` | Yes | enum | Exactly one of `Session`, `Recovery`, `Bootstrap`, `Operation`, `Information`, `Reference`. Case-sensitive. | Framework-specific. Determines lifecycle and exit protocol per the trait table above. |
| `triggers` | Yes | string or list of strings | Common values: `human-interactive`, `automation: <label>` (e.g. `automation: in-design`, `automation: in-development`), `automation: pr-review-submitted`, `automation: issue-assigned`, `on-demand`, `session-start`, `post-sync`. A skill may declare multiple triggers as a list. | Framework-specific. Describes what invokes the skill. Drives catalogue classification and trigger-conflict reasoning. |
| `loads` | Yes | list of strings | Zero or more skill `name` values (the names of other skills this skill may invoke). Use an empty list `[]` when the skill loads nothing else. | Framework-specific. Declares the set of skills this skill may hand control to or read during execution. Supports lazy loading: `session-init` reads the catalogue, the catalogue lists each skill's `loads`, and referenced skills are read only when the listed skill actually invokes them. |
| `emits-exit-block` | Yes | boolean | `true` or `false`. Must be `true` for `category: Session` and `category: Recovery`; must be `false` for all other categories. | Framework-specific. Declares whether the skill emits the canonical exit block on termination (see `skills/session-exit.md`). Also controls the session-termination rule (see RULEBOOK). |
| `exit-hands-to` | Yes | string or null | A short free-text label identifying the party control hands to (e.g. `automation: in-development`, `github-actions: pr-open`, `human`, `automation: feature-design`) or `null` when the skill does not hand over. Must be `null` when `emits-exit-block: false`. | Framework-specific. Machine-readable handover target. Feeds the `Next:` line of the exit block and the catalogue. |

### Rules

- **No unknown fields.** `gh agentic check` treats unknown keys as validation errors.
- **No renaming of Anthropic fields.** `name` and `description` keep Anthropic's
  exact names and semantics. Framework-specific extensions live alongside under
  their own names.
- **Consistency between `category` and `emits-exit-block`.**
  - `category: Session` or `category: Recovery` ⇒ `emits-exit-block: true`
  - Any other category ⇒ `emits-exit-block: false`
- **Consistency between `emits-exit-block` and `exit-hands-to`.**
  - `emits-exit-block: false` ⇒ `exit-hands-to: null`
  - `emits-exit-block: true` ⇒ `exit-hands-to` must be a non-empty string

### Worked example

A fully conformant frontmatter block for a session-ending skill:

```yaml
---
name: feature-scoping
description: Decomposes a Requirement issue into one or more Feature issues, defines acceptance criteria, and hands selected features to Feature Design via the in-design label. Use when the human opens the Feature Scoping (Stage 2) recipe to scope a requirement into features.
category: Session
triggers: human-interactive
loads:
  - session-init
  - capture-feature
  - set-issue-status
  - session-exit
emits-exit-block: true
exit-hands-to: "automation: feature-design | human (for held features)"
---
```

A fully conformant frontmatter block for a non-terminating skill:

```yaml
---
name: build-catalogue
description: Regenerates CATALOGUE.md from every skill's YAML frontmatter in a deterministic, diff-friendly order. Use when CATALOGUE.md is missing or stale (any skill file has a newer mtime than the catalogue), or when a skill has been added, removed, or had its frontmatter edited.
category: Operation
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---
```

## Consumers of this File

- `gh agentic check` — validates each skill's frontmatter against this schema.
- `skills/build-catalogue.md` — reads frontmatter, groups by `category`, emits `CATALOGUE.md`.
- `skills/session-init.md` — loads `CATALOGUE.md` (which is derived from this schema) instead of reading every skill body.
- Skill authors (human and AI) — read this file before writing or classifying a skill.

## Category Skeletons

Each skeleton below is the minimal conformant body structure a new skill of
that category should follow. Replace every `<…>` placeholder and every
`<!-- comment -->` with real content; the resulting file — combined with the
skeleton's frontmatter — is a valid skill.

Skeletons are authoritative templates used by `skills/skill-creation.md` to
generate new skill bodies, and by human authors writing skills by hand. They
do not introduce any new category trait, frontmatter field, or rule — they
only render the existing schema and taxonomy into a per-category starting
point.

Conventions used across all skeletons:

- **Frontmatter** block is shown in full. Copy it verbatim and fill in
  `<name>`, `<description>`, and any category-variable fields.
- **Body headings** are the required sections for the category. Sections
  marked *(optional)* may be omitted when there is nothing meaningful to put
  under them; never emit an empty section.
- **Exit block** placeholder appears only in Session and Recovery skeletons
  (the two categories with `emits-exit-block: true`).
- **Feedback block** placeholder appears in Operation skeletons — a short
  completion report that is *not* an exit block.

### Session skeleton

```markdown
---
name: <name>
description: <third-person, trigger-oriented description ending in "Use when …">
category: Session
triggers: <human-interactive | automation: <label>>
loads:
  - session-init
  - <other-skills-this-session-invokes>
  - session-exit
emits-exit-block: true
exit-hands-to: "<automation: <label> | human | next skill>"
---

# <Session Display Name> — <Phase Number or Name>

## Purpose

<!-- One paragraph: what pipeline phase this session drives and what artefact
     it produces. -->

## When to Run

<!-- The trigger: human-initiated recipe, automation label, etc. Reference
     the row in the RULEBOOK session table. -->

## How to Start

<!-- Concrete invocation path (recipe name, label applied, etc.). -->

## What the Agent Does

1. Print the session-start banner.
2. <Step — typically load context, verify prerequisites, gather inputs.>
3. <Step — produce the artefact(s), invoking ask-user inline at every
    confirmation, classification, or choice moment.>
4. <Step — wire labels, sub-issues, branches, or other state.>
5. Emit the canonical exit block (see `skills/session-exit.md`):

   ```
   === <Session Display Name> — Completed ===

   Produced:
     - <artefact 1>
     - <artefact 2>

   Blocked: <none | reason>

   Next: <automation trigger | human action | nothing>
   ```

6. Terminate the session per RULEBOOK — Session Termination.

## Outputs

<!-- Bulleted list of the artefacts produced on success. -->

## Rules

<!-- Session-specific rules. Do not restate framework-wide rules from
     RULEBOOK. -->
```

### Recovery skeleton

```markdown
---
name: <name>
description: <third-person, trigger-oriented description ending in "Use when …">
category: Recovery
triggers: human-interactive
loads:
  - session-init
  - <other-skills-this-session-invokes>
  - session-exit
emits-exit-block: true
exit-hands-to: "human | automation: <label>"
---

# <Recovery Display Name>

## Purpose

<!-- One paragraph: which blocked pipeline state this session recovers, and
     what the successful outcome looks like. -->

## When to Run

<!-- The human-triggered conditions under which this session is opened. -->

## How to Start

<!-- Concrete invocation path. -->

## What the Agent Does

1. Print the session-start banner.
2. <Step — gather the exact error output or blocking condition.>
3. <Step — diagnose; present candidate fixes to the human via ask-user.>
4. <Step — apply the minimal fix; verify it resolved the block.>
5. Emit the canonical exit block (see `skills/session-exit.md`) matching the
    actual outcome.
6. Terminate the session per RULEBOOK — Session Termination.

## Outputs

<!-- The fix applied, the artefact updated, or the explicit "nothing — human
     decided not to proceed" outcome. -->

## Rules

<!-- Recovery-specific rules (e.g. "never edit a signed commit without human
     approval"). -->
```

### Bootstrap skeleton

```markdown
---
name: <name>
description: <third-person, trigger-oriented description ending in "Use when …">
category: Bootstrap
triggers: <session-start | post-sync | on-demand>
loads:
  - <skills-this-bootstrap-invokes>
emits-exit-block: false
exit-hands-to: null
---

# <Bootstrap Display Name>

## Purpose

<!-- One paragraph: what environment state this skill prepares so the rest of
     the session runs correctly. -->

## When to Invoke

<!-- The precise conditions under which this skill runs. Bootstrap skills are
     invoked by the runtime or by another skill, never interactively. -->

## What the Agent Does

1. <Step — read/validate the artefact or condition this skill guards.>
2. <Step — perform the preparation (mount, validate, repair, load).>
3. Return control silently to the caller. Do not emit an exit block.

## Outputs

<!-- The environmental side-effects produced: files written, state loaded,
     checks run. -->

## Rules

<!-- Bootstrap-specific rules: what must be idempotent, what must hard-fail,
     what must never mutate. -->
```

### Operation skeleton

```markdown
---
name: <name>
description: <third-person, trigger-oriented description ending in "Use when …">
category: Operation
triggers: <on-demand | human-interactive>
loads:
  - <skills-this-operation-invokes>
emits-exit-block: false
exit-hands-to: null
---

# <Operation Display Name>

## Purpose

<!-- One paragraph: the input → output contract. What the operation takes in
     and what artefact or side effect it produces. -->

## When to Invoke

<!-- The trigger: human request, inline invocation from another skill,
     automation tick. -->

## Procedure

1. <Step — validate inputs; fail loudly on missing prerequisites.>
2. <Step — perform the work, invoking ask-user inline for any required
    human decision.>
3. <Step — write the artefact or apply the side effect.>
4. Emit the feedback block described below.
5. Return control silently. Do not emit a session exit block.

## Feedback Block

The Operation reports completion with a short plain-text feedback block —
**not** the Session/Recovery exit block. Shape:

```
=== <Operation Display Name> — Completed ===

Produced:
  - <artefact or side effect>

Blocked: <none | short reason>

Next: <informational note | nothing>
```

## Rules

<!-- Operation-specific rules: idempotency, ordering, what must never be
     silently skipped. -->
```

### Information skeleton

```markdown
---
name: <name>
description: <third-person, trigger-oriented description ending in "Use when …">
category: Information
triggers: <on-demand | automation: <label>>
loads: []
emits-exit-block: false
exit-hands-to: null
---

# <Information Display Name>

## Purpose

<!-- One paragraph: what context is surfaced and to whom (user, agent,
     downstream artefact). -->

## When to Invoke

<!-- The trigger: notification moment, periodic status print, downstream
     consumer asking for context. -->

## What the Agent Produces

<!-- The exact output shape — a formatted block, a notification payload, a
     summary file. Show a worked example. -->

## Procedure

1. <Step — gather the inputs the output is derived from.>
2. <Step — format the output per the shape above.>
3. <Step — emit it to the correct destination (stdout, notifier, file).>
4. Return control silently. Do not emit an exit block.

## Rules

<!-- Information-specific rules: sensitive-data redaction, truncation,
     deterministic formatting, etc. -->
```

### Reference skeleton

```markdown
---
name: <name>
description: <third-person, trigger-oriented description ending in "Use when …">
category: Reference
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# <Reference Display Name>

## Purpose

<!-- One paragraph: which rule, schema, pattern, or template this file
     canonicalises — and why a single source of truth matters for it. -->

## When to Invoke

<!-- The inline-invocation contract: which consumer skills read this file,
     under what condition. Reference skills are never standalone sessions. -->

## <Rule / Template / Schema Content>

<!-- The body is the rule or template itself, not a procedure. Examples of
     appropriate content:
       - A canonical shape (e.g. the exit block template).
       - A label-to-status mapping table.
       - A set of canonical prompt shapes.
       - A schema definition.
     Include worked examples when the shape is non-trivial. -->

## Rules

<!-- Rules about how consumers must use this reference: inline-only, no
     drift, no duplication. -->
```

## Rules

- This file is the only place the category taxonomy is defined. Any apparent
  contradiction elsewhere is a defect and must be fixed by updating the other
  file to match — never by editing this file to match the drift.
- Adding a category, renaming a category, or changing a field's required/optional
  status is a framework-level change. It must be agreed in a scoping session and
  landed through the normal pipeline, not ad-hoc.
- The field names in the Frontmatter Schema are normative: tools, validators,
  and generators must use these exact keys.
