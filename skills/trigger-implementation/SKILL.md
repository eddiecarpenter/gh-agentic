---
name: trigger-implementation
description: Triggers the implementation phase for a Feature by removing whichever design-phase label it currently carries (in-design, interactive-design, or designed) and applying in-development, then transitioning the project status to In Development. Workflow automation picks up in-development and runs the dev-session headlessly. Use when a calling skill or human needs to start the implementation phase for a Feature whose design is complete — both at the end of feature-design (interactive mode, "trigger now" choice) and standalone for un-parking a Feature that was previously stopped at designed. Use even when the caller doesn't say "trigger implementation" — phrases like "send this Feature to development", "start the dev phase on #42", "un-park this designed Feature", "ship the design to implementation" should trigger this primitive.
triggers: automated
user-invocable: true
loads:
  - skills/definitions/error-handling.md
  - skills/apply-label/SKILL.md
  - skills/set-issue-status/SKILL.md
emits-exit-block: false
---

# Trigger Implementation

## Goal

Start the implementation phase for a Feature whose design is
complete by transitioning it to `in-development` (label) and
`In Development` (project status). The Feature may currently be
at any of three design-phase labels:

- `in-design` — headless design just completed (the `feature-design`
  skill calls this primitive at end-of-flow in headless mode).
- `interactive-design` — interactive design just completed and the
  human chose "Trigger now" (the `feature-design` skill calls this
  primitive at end-of-flow in interactive mode when the human picks
  the trigger option).
- `designed` — Feature was parked after interactive design with the
  "Stop here" choice; a human (or an automation) is now un-parking
  it. This is the standalone-invocation case.

The primitive normalises the source label so callers don't repeat
the swap logic, and pairs the label change with the status update.
Wraps `apply-label` and `set-issue-status`.

## Output Artefacts

- Updated label state on the named Feature: whichever of
  `in-design`, `interactive-design`, or `designed` was present is
  removed; `in-development` is added (mutually exclusive with the
  three design labels).
- Updated project Status field: set to `In Development`.
- A return value to the caller of shape:
  ```
  { repo: <string>, issue: <int>, source_label: <string>,
    status: "In Development", triggered: <bool> }
  ```
  `source_label` is one of `"in-design"`, `"interactive-design"`,
  or `"designed"` — whichever was removed. `triggered: true` on
  success; on failure this primitive raises rather than returning
  `triggered: false`.

No file artefacts. No state outside the named Feature changes.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy applied
  to `INVALID_TRIGGER_STATE`, `ALREADY_TRIGGERED`, `LABEL_OPERATION_FAILED`,
  `STATUS_OPERATION_FAILED`, `GH_API_FAILED`.

## Dependencies

- `skills/apply-label/SKILL.md` — used in step 4 to swap the design
  label for `in-development` atomically.
- `skills/set-issue-status/SKILL.md` — used in step 5 to set the
  project Status field to `In Development`.

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
     target a Feature in another repo. When given, use it verbatim
     and do NOT call `gh repo view`.

2. **Query the Feature's current labels.** Determines which source
   label to remove and validates the Feature is in a triggerable
   state:

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
     `INVALID_TRIGGER_STATE` (`ERROR`) — the issue is not a Feature.
   - The label set MUST contain exactly one of `in-design`,
     `interactive-design`, or `designed`. Capture which one as
     `<source-label>`.
     - **None present** → raise `INVALID_TRIGGER_STATE` (`ERROR`).
       The Feature is not at a design-phase state and cannot be
       transitioned to in-development. The caller is expected to
       have verified state before invoking.
     - **More than one present** → raise `INVALID_TRIGGER_STATE`
       (`ERROR`). The Feature is in a corrupted state (label
       mismatch); recommend `gh agentic repair`.
   - The label set MUST NOT contain `in-development`. If present,
     raise `ALREADY_TRIGGERED` (`WARN`) — the Feature has already
     been triggered for implementation. Distinguish from
     `INVALID_TRIGGER_STATE` so the caller can choose to treat
     already-triggered as a no-op rather than an error.

4. **Apply the label swap.** The decision rule is trivial — remove
   `<source-label>`, add `in-development`:

   ```
   apply-label(repo=<repo>, issue=<issue>,
               add=["in-development"], remove=[<source-label>])
   ```

   The `apply-label` primitive does the read/edit/re-read atomically
   and verifies the result. On failure, propagate as
   `LABEL_OPERATION_FAILED` (`ERROR`) — preserve the underlying
   error code from `apply-label` in the error detail so the caller
   can diagnose.

5. **Set the project Status field to `In Development`.**

   ```
   set-issue-status(repo=<repo>, issue=<issue>, status="In Development")
   ```

   The `set-issue-status` primitive resolves the project ID, finds
   or creates the project item, and sets the field. On failure,
   propagate as `STATUS_OPERATION_FAILED` (`ERROR`) — preserve the
   underlying error code from `set-issue-status` in the error
   detail.

   **Important:** if step 4 succeeded but step 5 fails, the Feature
   is in an inconsistent state — label says `in-development`,
   project status still says `Designed` (or one of the In Design
   variants). Surface this clearly so the caller can decide whether
   to manually fix or let `gh agentic repair` reconcile.

6. **Return the result to the caller.**

   ```
   { repo: "<repo>", issue: <issue>,
     source_label: "<in-design | interactive-design | designed>",
     status: "In Development",
     triggered: true }
   ```

## Error Handling

- `INVALID_TRIGGER_STATE` from steps 2–3 (issue not found, not a
  Feature, not at a design-phase label, or in a corrupted multi-label
  state) → severity `ERROR`; propagate. Caller bug — they invoked
  the primitive on an issue that isn't ready for trigger.
- `ALREADY_TRIGGERED` from step 3 (Feature is already at
  `in-development`) → severity `WARN`. Caller may treat this as a
  no-op (the desired end state is already reached) or as an error
  (the caller expected to be the trigger source).
- `LABEL_OPERATION_FAILED` from step 4 (`apply-label` raised) →
  severity `ERROR`; propagate. The Feature's label state is
  unchanged. Recommend `gh agentic repair` and retry.
- `STATUS_OPERATION_FAILED` from step 5 (`set-issue-status` raised)
  → severity `ERROR`; propagate. **The Feature is in an inconsistent
  state**: label says `in-development`, project status still at
  the previous design-phase value. The caller / human must reconcile
  manually or via `gh agentic repair`.
- `GH_API_FAILED` from step 2 (the initial label query failed) →
  severity `ERROR`; propagate. The caller decides whether to retry
  — this primitive does not implement retry because the right
  policy depends on the caller's context (rate limit vs auth vs
  network).
- All other errors: propagate (default).
