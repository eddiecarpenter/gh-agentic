---
name: requirements-session
description: Captures a new business need as a Requirement issue in GitHub and, when the scope is clear, completes Feature Scoping inline. Use when a human opens the Requirements Session (Stage 1) recipe to record a new idea, need, or enhancement request.
category: Session
triggers: human-interactive
loads:
  - session-init
  - gh-agentic-tool
  - capture-feature
  - set-issue-status
  - session-exit
emits-exit-block: true
exit-hands-to: "automation: feature-design (if in-design applied inline) | human: run Feature Scoping Session"
---

# Requirements Session — Stage 1

## Purpose

Capture business needs as Requirement issues in GitHub.
This is a conversational session — the human drives, the agent listens, challenges, and records.
When the scope is apparent during this session, scoping may also be completed here —
no separate Scoping Session is needed.

## When to Run

Run this session whenever a new business need or idea needs to be captured.

## How to Start

Open Goose and select the **Requirements Session (Stage 1)** recipe.

## What the Agent Does

1. Prints: `=== Requirements Session (Phase 1) — Started ===`
2. **Verify framework version is current:**
   ```bash
   gh agentic info
   ```
   - Local version matches control plane → proceed
   - Version mismatch → run `gh agentic mount` to sync, then confirm versions match before continuing
3. Reads the project brief and existing open requirements
3. Converses with the human to distil raw ideas into clear needs
4. Challenges vague descriptions and solution-framed requirements
5. Creates GitHub Issues with `requirement` + `backlog` or `draft` labels
6. **Confirmation gate** — displays a structured summary and confirms with the human:
   - Issue number and title
   - Business need summary (one or two sentences)
   - Labels applied
   - Asks: *"Does this capture your requirement correctly? (yes / revise)"*
   - **If revise**: ask what to change, edit or recreate the issue, then present the
     summary again — loop until confirmed
   - **If yes**: proceed to step 7
7. Assesses whether the scope is apparent:
   - **If yes** — complete scoping in this session (see Completing Scoping Early below)
   - **If no** — label the requirement `backlog` and exit; the human will run the
     Scoping Session separately

8. Emits the canonical exit block (see `skills/session-exit.md`):

   ```
   === Requirements Session (Phase 1) — Completed ===

   Produced:
     - Requirement issue #N created (labels: requirement, backlog)

   Blocked: none

   Next: human: run Feature Scoping (Stage 2) recipe when ready
   ```

9. **Terminate the session.** Immediately after the exit block, invoke the host
   runtime's session-close API if exposed; otherwise halt. No continuation in
   the same session (see RULEBOOK — Session Termination).

## Completing Scoping Early

When the scope is clear enough to define the Feature(s) without a separate session:

1. Transition requirement: `backlog` → `scoping`
2. Work through the scoping artefacts (same as Feature Scoping Session):
   - User story (`As a / I want / so that`)
   - MVP scope and acceptance criteria
   - Serial vs parallel decomposition
   - UX design (if applicable)
3. Create Feature issue(s) using the `capture-feature` skill.
   **Critical:** the Feature body's parent reference must use the literal
   `Closes part of #<requirement-number>` (or `Closes #<requirement>` for
   single-Feature Requirements) — never `Parent: #N` or any other phrasing.
   The `feature-complete` workflow's parent-parse regex matches only the
   `Closes` form; any other phrasing silently breaks the auto-close-parent
   logic, leaving the Requirement open after all its sub-issues complete.
   See `capture-feature.md` "Parent Requirement" section.
4. Wire sub-issue: Feature → parent Requirement
5. Apply `in-design` to features that are ready to proceed (cross-repo dependency rules apply)
6. Transition requirement: `scoping` → `scheduled`
   (`done` is applied automatically by the feature-complete workflow when all child features are closed)
7. Emit the canonical exit block (see `skills/session-exit.md`) matching the
   actual outcome. Both inline variants conform to the same shape.

   **All features triggered:**
   ```
   === Requirements Session (Phase 1) — Completed ===

   Produced:
     - Requirement issue #N created
     - Feature issue #N created (triggered for design, inline)
     - Requirement #N transitioned: scoping → scheduled

   Blocked: none

   Next: automation: feature-design (in-design label on #N)
   ```

   **Some features held (cross-repo dependency):**
   ```
   === Requirements Session (Phase 1) — Completed ===

   Produced:
     - Requirement issue #N created
     - Feature issue #N created (triggered for design, inline)
     - Feature issue #N created (held at backlog — cross-repo dependency)
     - Requirement #N transitioned: scoping → scheduled

   Blocked: #N — depends on <feature/PR reference> to merge first

   Next: automation: feature-design (in-design label on #N); human: re-trigger #N once the upstream PR merges
   ```

8. **Terminate the session.** Immediately after emitting the exit block, invoke
   the host runtime's session-close API if exposed; otherwise halt. The
   `in-design` label has been applied — GitHub Actions runs the next session.
   Continuing into Feature Design or Dev Session from here is a defect (see
   RULEBOOK — Session Termination).

## Outputs

- One GitHub Issue per discrete business need
- Labels: `requirement` + `backlog` (ready for scoping) or `draft` (still being refined)
- If scoping completed inline: Feature issue(s) created with `in-design` applied

## Rules

- One issue per discrete business need
- If the human is unclear, ask — never invent behaviour
- Label `draft` if still being refined, `backlog` when agreed
- Completing scoping early is not skipping it — the Feature issue artefact must still be
  produced with all required sections (user story, acceptance criteria, parent link)
- Never defer a phase without checking with the human first — see `RULEBOOK.md`

## Next Step

If scoping was completed inline, the Feature Design Session triggers automatically.
If not, when the requirement is in `backlog` state, run the **Feature Scoping (Stage 2)** recipe.
