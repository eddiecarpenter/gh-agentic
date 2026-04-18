---
name: session-exit
description: Defines the canonical universal exit block emitted by every Session and Recovery skill at termination, with the three fixed sections (Produced, Blocked, Next) and worked variants. Use when authoring or updating any session-ending skill, or when verifying that an exit block conforms to the framework shape.
category: Reference
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# Session Exit — Canonical Exit Block

## Purpose

Every Session and Recovery skill (see `skills/skill-categories.md`) **must** emit
the same exit block shape when it reaches its terminal step. This file is the
single source of truth for that shape. All session-ending skills reference this
file rather than redefining the block locally.

Why a single shape:

- **Unambiguous handover.** The `Next:` line tells the next actor — automation,
  human, or nothing — what happens next. No guessing.
- **Machine-parseable.** The block is plain text with fixed anchors (`===`,
  `Produced:`, `Blocked:`, `Next:`) so tooling can detect session termination
  reliably.
- **No drift.** Any deviation is a defect. Fix the skill, never the template.

## The three fixed sections

Every exit block has exactly these three sections, in this order, always present:

1. **Produced** — what this session created. The list of artefacts (issue
   numbers, branches, PRs, files, labels applied). If nothing was produced,
   record `Produced:` followed by a single bullet stating the fact (e.g.
   `- nothing — session aborted before any artefact was created`). Never omit.
2. **Blocked** — whether the session reached a blocked state, and the reason.
   Explicit value is required: `Blocked: none` when nothing is blocked. Never
   omit.
3. **Next** — the handover. Names the automation trigger, the human action, or
   `nothing`. Never omit.

## Canonical template

Copy this exactly. The `===` marker is the only decoration permitted.

```
=== <Skill Name> — Completed ===

Produced:
  - <artefact 1>
  - <artefact 2>

Blocked: <none | description of what is blocked and why>

Next: <automation trigger | human action | nothing>
```

## Rules

- **Plain text only.** Terminal-friendly. No colour codes, no Unicode decoration
  beyond the `===` markers on the first line.
- **All three sections are always present.** Missing information is explicit
  (`Blocked: none`, `Next: nothing`), never omitted.
- **`Next:` is the handover field.** It names exactly one of:
  - An automation trigger — e.g. `automation: feature-design (in-design label)`
  - A human action — e.g. `human — review PR #N and merge if approved`
  - `nothing` — when the session produced no handover
- **Branching outcomes emit a conforming variant.** A skill with multiple
  legitimate exit states (e.g. `feature-scoping` with all-triggered /
  some-held / none-triggered) emits the variant that matches the actual state,
  and each variant conforms to the same template shape.
- **The skill name in the header line.** Use the session's display name (e.g.
  `Feature Scoping Session (Phase 2)`, `Dev Session`, `Foreground Recovery`).
- **Emit the block immediately before terminating.** The skill must not perform
  additional work after emitting the block (see RULEBOOK — session-termination
  rule).

## Worked examples

### Example 1 — Clean automated handoff

A feature-scoping session where all agreed features are triggered for design.

```
=== Feature Scoping Session (Phase 2) — Completed ===

Produced:
  - Feature issue #512 created (triggered for design)
  - Feature issue #513 created (triggered for design)
  - Requirement #480 transitioned: scoping → scheduled

Blocked: none

Next: automation: feature-design (in-design label on #512, #513)
```

### Example 2 — Some-features-held variant

A feature-scoping session where one feature is held because it depends on an
unmerged upstream PR.

```
=== Feature Scoping Session (Phase 2) — Completed ===

Produced:
  - Feature issue #521 created (triggered for design)
  - Feature issue #522 created (held at backlog — cross-repo dependency)
  - Requirement #481 transitioned: scoping → scheduled

Blocked: #522 — depends on NewOpenBSS/charging-domain PR #1045 to merge first

Next: automation: feature-design (in-design label on #521); human: re-trigger #522 once the upstream PR merges
```

### Example 3 — All-features-held variant

A feature-scoping session where every agreed feature is held pending a
dependency — no automation triggered at all.

```
=== Feature Scoping Session (Phase 2) — Completed ===

Produced:
  - Feature issue #530 created (held at backlog — cross-repo dependency)
  - Feature issue #531 created (held at backlog — cross-repo dependency)
  - Requirement #482 transitioned: scoping → scheduled

Blocked: #530, #531 — both depend on NewOpenBSS/charging-domain PR #1052 to merge first

Next: nothing — re-trigger each feature once the upstream PR merges
```

### Example 4 — Dev Session clean exit

A dev session that completed all tasks and verified acceptance criteria.

```
=== Dev Session — Completed ===

Produced:
  - 7 tasks closed (#540–#546)
  - 7 commits on feature/497-example
  - Acceptance criteria coverage verified (5 of 5)
  - recovery.md archived to recovery-logs/recovery-log-497.md

Blocked: none

Next: automation: workflow pushes branch and opens PR closing #497
```

### Example 5 — Foreground Recovery exit

A foreground recovery session that fixed a failing build and pushed the fix.

```
=== Foreground Recovery — Completed ===

Produced:
  - fix: corrected nil-pointer in internal/chargeengine/debit.go
  - 1 commit on feature/498-balance-check
  - Build and tests pass locally

Blocked: none

Next: automation: dev-session re-triggers on push; if not, human re-applies in-development label on #498
```

## Rules for adopting this template

- Any skill with `category: Session` or `category: Recovery` in its frontmatter
  **must** have `emits-exit-block: true` and **must** emit one of the variants
  above at termination.
- Any skill with a category other than Session or Recovery **must** have
  `emits-exit-block: false` and **must not** emit this block. These skills
  return control silently.
- The exact header text (`=== <Skill Name> — Completed ===`) is the anchor tools
  use to detect session termination. Do not alter the spacing, the `===`
  markers, or the em-dash.
- If you need a new exit variant (for a genuinely new outcome shape), add a
  worked example here rather than improvising in the skill.
