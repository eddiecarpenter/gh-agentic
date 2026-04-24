---
name: capture-design-plan
description: Defines the canonical Markdown template for the Design Plan comment feature-design publishes on a Feature issue before any Task sub-issues are created — decomposition rationale, planned tasks, alternatives considered, refactor assessment, and optional codebase findings and risks. Use when feature-design is about to publish its pre-task decomposition rationale as a durable comment on the Feature issue, or when verifying that a published Design Plan comment conforms to the required shape.
category: Reference
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# Capture Design Plan — Feature Issue Comment Template

## Purpose

Defines the canonical structure for the **Design Plan** — the structured comment
`feature-design` publishes on a Feature issue *before* any Task sub-issues are
created. The comment is the durable, machine-readable artefact that binds the
planned tasks to the reasoning behind them. It is the Plan step of SAPAV for
the design phase: the moment a human or a PR-review session can later ask "did
the implementation match the design's intent?" because the intent has been
written down.

Task emission is blocked until this comment has been published successfully.
Publication failure halts the design session — see `skills/feature-design.md`
for the halt discipline.

## When to Use

- `feature-design` loads this skill at the moment it is ready to publish the
  Design Plan comment — after analysis (step 3), after Refactor Assessment
  (step 4), and *before* Task sub-issue creation.
- Use it again when the plan must be amended after tasks are created, to
  populate the `#N` references in the `## Tasks` section (append-only — see
  Rules).
- Use it when reviewing a published Design Plan comment to verify it conforms
  to the shape below.

## Template

The agent copies this template verbatim, fills in the sections, and posts the
result as a comment on the Feature issue (via `gh issue comment --body-file`).

```markdown
## Design Plan

## Decomposition

One-paragraph rationale binding the planned tasks together. State the single
organising idea behind the decomposition — what shape the work takes and why
these tasks, in this order, cover the feature.

## Tasks

Ordered list of planned tasks in implementation sequence. Before task
creation, each line uses a `[planned]` placeholder because issue numbers do
not yet exist. After task creation, a `### Tasks (created)` subsection is
appended with the real `#N` references (append-only amendment — see Rules).

- [planned] <task-1 title>
- [planned] <task-2 title>
- [planned] <task-3 title>

## Alternatives Considered

At least one line of content. Name the decomposition(s) that were considered
and rejected, and the reason each was rejected. If a feature has a single
obvious decomposition, the section body is the literal string:

_None — single obvious decomposition._

Empty or omitted is invalid — the section must always have content. The
literal fallback forces honest accounting: "I did consider alternatives and
none was viable" is distinct from "I did not think about it".

## Refactor Assessment

Summary of the outcomes recorded during the Refactor Assessment step of
`feature-design.md` (step 4), per the canonical recording format from
`skills/refactor-assessment.md`. One line per new symbol the feature will
introduce, using the canonical `Reuse: <outcome> — <reason>` annotation:

- `Reuse: as-is — <symbol reference>` — the feature will call an existing
  symbol; no refactor task needed.
- `Reuse: via-refactor — <symbol and refactor task reference>` — an existing
  symbol nearly covers the need; a refactor task is emitted as the first
  task in the order.
- `Reuse: none — <motivation>` — no adjacent code covers the need; the
  feature introduces the symbol fresh.
- `Reuse: n/a — <description>` — the task introduces no new symbols.

## Codebase Findings

*(optional — include when specific files, symbols, or modules were inspected
during analysis and the references help future PR review.)*

List the relevant files and symbols that were examined, with a one-line note
of what each contributes to the decomposition.

## Risks

*(optional — include when the design is explicitly making a bet on a
particular assumption, interface stability, or trade-off.)*

List known risks the design is accepting, one per line. Each risk should
state the assumption and what would invalidate it.
```

### Amendment after Task creation

After Task sub-issues have been created and their `#N` values are known, the
comment is amended **append-only**. Pick one of the two approaches and use it
consistently for this plan:

1. **Edit the original comment** to append a `### Tasks (created)` subsection
   at the end of the `## Tasks` section (never rewrite the original
   `[planned]` list):

   ```markdown
   ### Tasks (created)

   - #<N1> — <task-1 title>
   - #<N2> — <task-2 title>
   - #<N3> — <task-3 title>
   ```

2. **Post a follow-up comment** linking to the original Design Plan and
   containing the same `### Tasks (created)` subsection.

Both approaches are append-only: the original `[planned]` list remains
unchanged so the audit trail between plan and execution is preserved.

## Rules

- **Soft target 300–500 words; hard cap 1000 words.** These bounds apply to
  the published comment body (template-filled, not the raw template). Aim
  for the soft target; treat 500–1000 as a yellow zone that needs
  justification.
- **Beyond the hard cap 1000 words:** the agent logs a warning (surfaced for
  PR review to flag) and still publishes. Do not truncate — a warning plus
  a long plan is better than a silently truncated plan.
- **`## Alternatives Considered` must never be empty.** If there truly is a
  single obvious decomposition, use the literal string
  `_None — single obvious decomposition._` — this is the only permitted
  empty-alternatives phrasing. Empty or omitted section is invalid and
  blocks task emission.
- **Amendment is append-only.** After task creation, either edit the
  original comment to add a trailing `### Tasks (created)` subsection with
  the real `#N` references, or post a follow-up comment linked to the
  original. **Never rewrite existing sections** — the audit trail between
  the original plan and the created tasks must be preserved.
- **Cross-repo references are not needed** — Tasks always live in the same
  repo as the Feature, so the `#N` short form is always correct inside the
  Design Plan comment.
- **Publication failure halts the design session** — see
  `skills/feature-design.md` for the halt discipline. This skill defines
  only the shape of the artefact; enforcement of the publish-before-act and
  halt-on-failure rules lives in `feature-design.md`.
