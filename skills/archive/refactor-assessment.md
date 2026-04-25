---
name: refactor-assessment
description: Canonicalises the search-first, reuse-default, motivate-if-not procedure every code-touching skill performs before writing new code — defines the three permitted outcomes (reuse as-is, reuse via refactor, do not reuse with motivation), the opt-out variant, the single-line recording format, and the loader phrase consumer skills invoke. Use when any skill or agent is about to introduce a new function, type, module, schema, or similar symbol and must first confirm whether existing code already covers the need.
category: Reference
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# Refactor Assessment — Reuse & Refactor Discipline

## Purpose

This file is the **single source of truth** for the Reuse & Refactor Discipline.
It defines:

1. When a code-touching skill must perform the check.
2. The search procedure — how to look before writing.
3. The three permitted outcomes, and the opt-out variant.
4. The canonical single-line recording format.
5. The canonical loader phrase consumer skills invoke.

Consumer skills (`feature-design`, `dev-session`, `issue-session`, and the
universal `RULEBOOK` rule covering ad-hoc and foreground-recovery changes)
**load this file by name** and **reference it** — they must never restate the
procedure. One canonical procedure, no per-skill variation.

## When to Invoke

The check is required any time the agent is about to introduce a new **symbol**
into the codebase — a function, method, type, struct, interface, constant with
business meaning, module, package, schema, template, or any other named unit of
behaviour or structure.

The check applies:

- In `feature-design` — before emitting task sub-issues. If existing code
  partially covers the scope, a refactor task is emitted ahead of the feature
  tasks that depend on it.
- In `dev-session` — per task, before writing any new symbol for that task.
  "I didn't look" is not a permitted outcome; the session **halts** if the
  check has not been performed.
- In `issue-session` (bug route) — before implementing a bug fix that
  introduces a helper, type, or function. Pure local changes with no new
  symbols still record the outcome (see `n/a` below).
- In **ad-hoc human-prompted changes** and **foreground-recovery** — the
  universal rule in `RULEBOOK.md` covers these; this skill is loaded the same
  way.

The check is **not** required when the work introduces no new named symbol
(e.g. a comment fix, a constant tweak, a typo correction, a documentation-only
change). In those cases the outcome is recorded as `Reuse: n/a — <reason>` if
a consumer skill demands an outcome slot, or omitted where the skill makes the
slot optional.

## The Search Procedure

Perform these steps, in order, before writing any new symbol:

1. **Name the capability.** State in one sentence what you are about to add —
   the behaviour, the data shape, or the structural role. Give it a working
   name. Being explicit about the capability is what makes the search
   tractable.
2. **Search the repo for existing implementations.** Use every affordance
   available:
   - Grep by behaviour keywords (verbs and nouns describing the capability).
   - Grep by likely symbol names and likely file names.
   - Read the nearest package's `CATALOGUE.md`, `README.md`, or index file if
     one exists.
   - Inspect related symbols whose signatures or types are similar — a
     capability may already exist under an unrelated name.
   - For framework skills: scan `CATALOGUE.md` at the repo root for a
     matching skill before authoring a new one.
3. **Inspect candidates.** For each candidate, read the signature, the doc
   comment, and at least one call site. Decide whether it covers the need.
   - Does its behaviour match?
   - Is its scope a superset, a subset, or orthogonal?
   - What would it cost to extend it versus to introduce a parallel symbol?
4. **Decide.** Choose one of the three outcomes below. Record the outcome in
   the canonical format.

## The Three Permitted Outcomes

Exactly three outcomes are permitted, plus the opt-out variant. No other
classification is accepted.

### 1. Reuse as-is

An existing symbol already covers the need. Call it; do not fork it, do not
copy it, do not create a parallel. The reference goes into the recording
annotation.

### 2. Reuse via refactor

An existing symbol nearly covers the need. Extend or generalise it — add a
parameter, widen a type, lift it to a more general package — rather than
introducing a parallel symbol. In `feature-design` this means emitting a
refactor task ahead of the feature tasks that depend on it; in `dev-session`
and `issue-session` this means performing the refactor in the same commit (or
an earlier commit on the same branch) and threading the call through.

### 3. Do not reuse

An existing candidate is genuinely unsuitable. The agent records the
motivation — why reuse was rejected. Acceptable reasons include (non-exhaustive):

- **Scope mismatch** — the existing symbol's scope is orthogonal and coupling
  them would produce a worse abstraction.
- **Cost** — extending the existing symbol would touch N unrelated call sites
  and the risk outweighs the benefit within the current task scope.
- **Wrong abstraction** — the existing symbol is already known to be a
  suboptimal shape and adopting it would propagate the shape further.
- **Contract boundary** — the existing symbol is a contract shared with
  external consumers (see RULEBOOK — Contract Rules) and extending it requires
  approval that has not been obtained.

"I looked and none fit" without a reason is **not** a valid recording — the
outcome must state *why* the existing code is unsuitable.

### Opt-out variant

A human may explicitly waive the check for a specific ad-hoc change. The
waiver is recorded in the same annotation slot as the three outcomes, with the
human-supplied reason. An opt-out is never inferred — the human must say so.

## The Canonical Recording Format

A single line, placed in the commit trailer (for code-touching commits) or in
a task comment / design-artefact note (where no commit is produced). The line
never goes in the commit subject — subjects stay clean.

**Shape:**

```
Reuse: <as-is|via-refactor|none|opt-out|n/a> — <one-line reference or reason>
```

Where:

- `as-is` — **Reuse as-is**. Reference the symbol reused.
- `via-refactor` — **Reuse via refactor**. Reference the symbol extended (and,
  in `feature-design`, the refactor task issue number).
- `none` — **Do not reuse**. One-line motivation.
- `opt-out` — human waived the check. One-line human-supplied reason.
- `n/a` — no new symbols introduced by this change (e.g. pure local fix,
  doc-only edit). One-line description.

**Line rules:**

- One `Reuse:` line per distinct decision. A commit or task that introduces
  multiple unrelated symbols may carry multiple `Reuse:` lines.
- The reason / reference is a single line — no wrapping, no multi-line blocks.
- No trailing period required.
- The em-dash (`—`) between the outcome token and the reason is canonical;
  tools parse for it.

**Commit placement:** the `Reuse:` line(s) go in the commit body (trailer
position, after any summary paragraph and a blank line). `git log` surfaces
them cleanly without polluting the subject.

**Task-comment placement:** the same line goes on its own paragraph in the
design artefact or GitHub task comment.

## The Canonical Loader Phrase

Consumer skills invoke this reference skill with **this exact phrase**, verbatim:

> Invoke skills/refactor-assessment.md before writing any new code.

Consistent phrasing matters — it makes the discipline trivially audit-able
with a grep. Any deviation in a consumer skill is a defect; fix the consumer,
never paraphrase.

## Worked Examples

### Example 1 — Reuse as-is

Scenario: `dev-session` task adds a new command that needs to resolve the
repo owner/name. A canonical `internal/project.Resolver` already exists and
is used by six other commands.

Search: grep `Resolver`, read `internal/project/resolver.go`, inspect one
existing call site. The capability matches exactly.

Outcome: **Reuse as-is**. Call `project.Resolver.Resolve()` — do not add a
parallel helper.

Recording (commit trailer):

```
feat: add `gh agentic status` command — task 3 of 7 (#489)

Wires the new command into the Cobra tree and renders a table from the
existing status computation.

Reuse: as-is — internal/project.Resolver (already used by info, check, repair)
```

### Example 2 — Reuse via refactor

Scenario: `feature-design` is decomposing a feature that adds a
project-context reader for the new `gh agentic kanban` command. Four ad-hoc
readers already exist in the code base, each slightly different.

Search: grep `project`, find the four parallel readers, read each. They share
80% of their logic; they differ only in the output shape they return.

Outcome: **Reuse via refactor**. Emit an ordered refactor task ahead of the
feature tasks that depend on it: consolidate the four readers into a single
`internal/project.Context` with an output adapter.

Recording (design artefact / feature-design exit block):

```
Refactor Assessment: 1 refactor task emitted — consolidate four ad-hoc
project-context readers into internal/project.Context

Reuse: via-refactor — consolidate internal/status, internal/info, internal/check,
internal/kanban readers (refactor task #601 ahead of feature tasks #602–#605)
```

### Example 3 — Do not reuse (with motivation)

Scenario: `dev-session` task adds a Kafka event handler for a new topic. An
existing `internal/events/wholesale_handler.go` handles a different contract
with overlapping field names.

Search: grep `handleRecord`, read the wholesale handler, inspect its switch
statement. The shape overlaps but the wholesale handler is tightly coupled to
the wholesale contract's `*EventType` — extending it would bleed the new
topic's semantics into a contract-bound file.

Outcome: **Do not reuse**. The existing handler is contract-bound; extending
it would couple two unrelated contracts and violate the RULEBOOK contract
rules.

Recording (commit trailer):

```
feat: add retail event handler for retail.v1 topic — task 2 of 5 (#611)

Introduces internal/events/retail_handler.go with its own switch — kept
separate from the wholesale handler to preserve contract isolation.

Reuse: none — wholesale_handler is contract-bound to WholesaleContractEventType;
extending it would couple two unrelated contracts (see RULEBOOK — Contract Rules)
```

### Example 4 — Opt-out

Scenario: a human asks the agent to add a quick diagnostic print inside an
experimental script. The human says: "skip the reuse check — this is a
throwaway."

Outcome: **Opt-out**. The agent records the waiver and the human-supplied
reason.

Recording (commit trailer):

```
chore: add experimental diagnostic print to scripts/debug-queue.sh

Temporary diagnostic; will be removed once the race is reproduced.

Reuse: opt-out — throwaway diagnostic in experimental script (human-waived)
```

## Consumers of This File

- `RULEBOOK.md` — universal Reuse & Refactor Discipline rule.
- `skills/feature-design.md` — Refactor Assessment step before task emission.
- `skills/dev-session.md` — per-task Reuse & Refactor Check before writing any
  new symbol.
- `skills/issue-session.md` — Reuse & Refactor Check before a bug-fix commit.
- Ad-hoc human-prompted changes and `skills/foreground-recovery.md` — covered
  by the universal rule in `RULEBOOK.md`.

## Rules

- **This file is the only place the procedure is defined.** Consumer skills
  reference it; they must never restate the procedure. Duplication is drift.
- **The three outcomes are fixed.** Adding a fourth outcome is a framework
  change and must land through the normal pipeline — not ad-hoc.
- **The recording format is fixed.** Tools and humans grep for `Reuse: ` at
  the start of a line; changing the shape breaks audit.
- **The loader phrase is fixed.** Consumer skills invoke it verbatim.
- **"I didn't look" is not a valid outcome.** If the check cannot be
  performed, the consumer skill halts and reports the block.
- **Opt-out requires an explicit human statement in the current session.** A
  prior session's opt-out does not carry forward.
