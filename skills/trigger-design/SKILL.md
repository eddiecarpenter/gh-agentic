---
name: trigger-design
description: Triggers the design phase for a Feature by applying the appropriate trigger label (in-design for headless or interactive-design for foreground, based on the needs-interactive-design classification on the Feature) and transitioning the project status to In Design. Workflow automation picks up in-design and runs the design skill headlessly; interactive-design Features are run in foreground by the human. Use when a calling skill or human needs to start the design phase for a single backlog Feature — both at the end of requirement-scoping and standalone for re-triggering held Features after blockers clear. Use even when the caller doesn't say "trigger design" — phrases like "start design for #42", "begin the design phase on this Feature", "send this Feature to design" should trigger this primitive.
triggers: automated
user-invocable: true
loads:
  - skills/definitions/error-handling.md
  - skills/apply-label/SKILL.md
  - skills/set-issue-status/SKILL.md
emits-exit-block: false
---

# Trigger Design

## Goal

Start the design phase for a single Feature by transitioning it from
`backlog` to either `in-design` (headless) or `interactive-design`
(foreground), based on whether the Feature carries the
`needs-interactive-design` classification label. Project status moves
to `In Design` in both cases — only the trigger label differs.

The decision rule is encapsulated here so callers (requirement-scoping
at end-of-walk, or the human re-triggering a held Feature) don't have
to repeat it. Wraps `apply-label` and `set-issue-status`.

## Output Artefacts

- Updated label state on the named Feature: `backlog` removed; either
  `in-design` or `interactive-design` added (mutually exclusive).
- Updated project Status field: set to `In Design`.
- A return value to the caller of shape:
  ```
  { repo: <string>, issue: <int>, trigger_label: <string>,
    status: "In Design", triggered: <bool> }
  ```
  `trigger_label` is one of `"in-design"` or `"interactive-design"`,
  whichever was applied. `triggered: true` on success; on failure
  this primitive raises rather than returning `triggered: false`.

No file artefacts. No state outside the named Feature changes.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy applied
  to `INVALID_TRIGGER_STATE`, `ALREADY_TRIGGERED`, `LABEL_OPERATION_FAILED`,
  `STATUS_OPERATION_FAILED`, `GH_API_FAILED`.

## Dependencies

- `skills/apply-label/SKILL.md` — used in step 4 to swap the trigger
  label atomically (`backlog` → `in-design` or `interactive-design`).
- `skills/set-issue-status/SKILL.md` — used in step 5 to set the
  project Status field to `In Design`.

## Steps

1. **Receive the inputs from the caller.**
   - `issue` (int, required) — the Feature issue number.
   - `repo` (string, optional, `owner/name` format) — the repo the
     Feature lives in. The normal case is that the Feature lives in
     the active repo and the caller omits this; resolve it via:

     ```bash
     gh repo view --json nameWithOwner -q .nameWithOwner
     ```

     Power-user / cross-repo callers may pass `repo` explicitly to
     target a Feature in another repo (e.g., the human re-triggering
     a held Feature from a different working directory). When given,
     use it verbatim and do NOT call `gh repo view`.

2. **Query the Feature's current labels.** Determines which trigger
   label to apply and validates the Feature is in a triggerable state:

   ```bash
   gh issue view "$issue" --repo "$repo" --json labels \
     --jq '[.labels[].name]'
   ```

   **Detect:**
   - Exit code non-zero with stderr containing "Could not resolve to
     an Issue" → raise `INVALID_TRIGGER_STATE` with severity `ERROR`
     (the issue does not exist, or the caller passed the wrong repo).
   - Exit code non-zero otherwise → raise `GH_API_FAILED` with
     severity `ERROR`; include the stderr.

3. **Validate the Feature's current state.**

   - The label set MUST contain `feature`. If not, raise
     `INVALID_TRIGGER_STATE` (`ERROR`) — the issue is not a Feature
     (likely a Requirement or some other kind), and triggering it
     for design is meaningless.
   - The label set MUST contain `backlog`. If not, raise
     `INVALID_TRIGGER_STATE` (`ERROR`) — the Feature is not at the
     backlog stage and cannot be transitioned to in-design from
     wherever it currently is. The caller is expected to have
     verified state before invoking.
   - The label set MUST NOT contain `in-design` or
     `interactive-design`. If either is present, raise
     `ALREADY_TRIGGERED` (`WARN`) — the Feature has already been
     triggered. Distinguish from `INVALID_TRIGGER_STATE` so the
     caller can choose to treat already-triggered as a no-op rather
     than an error.

4. **Determine the trigger label and apply the swap.** The decision
   rule:

   - If `needs-interactive-design` is in the label set → trigger
     label is `interactive-design`. Workflow automation does NOT
     pick this up; the human (or a calling skill) must invoke the
     design skill in foreground.
   - Otherwise → trigger label is `in-design`. Workflow automation
     picks this up and runs the design skill headlessly.

   Apply the label change via `apply-label`:

   ```
   apply-label(repo=<repo>, issue=<issue>,
               add=[<trigger-label>], remove=["backlog"])
   ```

   The `apply-label` primitive does the read/edit/re-read atomically
   and verifies the result. On failure, propagate as
   `LABEL_OPERATION_FAILED` (`ERROR`) — preserve the underlying
   error code from `apply-label` in the error detail so the caller
   can diagnose.

5. **Set the project Status field to `In Design`.**

   ```
   set-issue-status(repo=<repo>, issue=<issue>, status="In Design")
   ```

   The `set-issue-status` primitive resolves the project ID, finds
   or creates the project item, and sets the field. On failure,
   propagate as `STATUS_OPERATION_FAILED` (`ERROR`) — preserve the
   underlying error code from `set-issue-status` in the error
   detail.

   **Important:** if step 4 succeeded but step 5 fails, the Feature
   is in an inconsistent state — label says triggered, project
   status still says Backlog. Surface this clearly so the caller
   can decide whether to manually fix or let `gh agentic repair`
   reconcile.

6. **Return the result to the caller.**

   ```
   { repo: "<repo>", issue: <issue>,
     trigger_label: "<in-design or interactive-design>",
     status: "In Design",
     triggered: true }
   ```

## Error Handling

- `INVALID_TRIGGER_STATE` from steps 2–3 (issue not found, not a
  Feature, or not at backlog) → severity `ERROR`; propagate. Caller
  bug — they invoked the primitive on an issue that isn't ready
  for trigger.
- `ALREADY_TRIGGERED` from step 3 (Feature is already at `in-design`
  or `interactive-design`) → severity `WARN`. Caller may treat this
  as a no-op (the desired end state is already reached) or as an
  error (the caller expected to be the trigger source).
- `LABEL_OPERATION_FAILED` from step 4 (`apply-label` raised) →
  severity `ERROR`; propagate. The Feature's label state is
  unchanged. Recommend `gh agentic repair` and retry.
- `STATUS_OPERATION_FAILED` from step 5 (`set-issue-status` raised)
  → severity `ERROR`; propagate. **The Feature is in an inconsistent
  state**: label says triggered, project status still at Backlog.
  The caller / human must reconcile manually or via
  `gh agentic repair`.
- `GH_API_FAILED` from step 2 (the initial label query failed) →
  severity `ERROR`; propagate. The caller decides whether to retry
  — this primitive does not implement retry because the right
  policy depends on the caller's context (rate limit vs auth vs
  network).
- All other errors: propagate (default).
