---
name: issue-session
description: Handles a GitHub Issue assigned to the agent — routes by label to either fix a bug on a new branch or answer a question as a comment, and exits cleanly so the workflow can open a PR if code changed. Use when GitHub Actions triggers this session automatically on an issue being assigned to the agent user — never run interactively.
category: Session
triggers: "automation: issue-assigned"
loads:
  - session-init
  - gh-agentic-tool
  - refactor-assessment
  - session-exit
emits-exit-block: true
exit-hands-to: "automation: github-actions opens PR if code changed | human: review comment/PR"
---

# Issue Session — Stage 4c

## ⛔ Automation-Only — Do Not Execute Interactively

This session is triggered exclusively by GitHub Actions when a GitHub Issue is assigned
to the agent user. It must never be run manually by an agent in an interactive session.

If you are reading this skill in an interactive session, stop immediately and print:

```
REFUSED: Issue Session is automation-only.
It runs automatically when a GitHub Issue is assigned to the agent user.
Do not execute this session interactively.
```

Do not proceed past this point in an interactive context.

---

## Purpose

Handle a GitHub Issue that has been assigned to the agent.
Routes by label: fixes bugs or answers questions.

## When it Runs

Triggered automatically by GitHub Actions when a GitHub Issue is assigned to
the agent user (e.g. `goose-agent`).

## What the Agent Does

1. Reads the issue: title, body, and labels
2. Posts an acknowledgement comment
3. Routes by label:
   - **bug**: locates the problem, verifies fix is in safe scope, creates a fix
     branch, performs the **Reuse & Refactor Check** before implementing the
     fix — Invoke skills/refactor-assessment.md before writing any new code.
     For each helper, type, or function introduced by the fix, record one of
     the three outcomes defined in `refactor-assessment.md` (as-is /
     via-refactor / none). If the fix is purely a local change with no new
     symbols, record `n/a` with a short description (e.g.
     `Reuse: n/a — pure local fix`). Then implements the minimal fix, builds
     and tests, and commits — the commit body carries the canonical `Reuse:`
     trailer (workflow pushes and opens the PR):

     ```
     fix: [bug description] (#N)

     <optional short paragraph>

     Reuse: <as-is|via-refactor|none|n/a> — <reason or reference>
     ```
   - **question**: researches the answer, posts a detailed reply, adds `answered` label
     (no code changes, no branch, no PR, no reuse trailer required)
   - **other**: posts a comment asking for a `bug` or `question` label
4. Emits the canonical exit block (see `skills/session-exit.md`). Match the
   actual outcome; all variants conform to the same shape.

   **Bug fix applied:**
   ```
   === Issue Session (Stage 4c) — Completed ===

   Produced:
     - Fix branch fix/<N>-<description> created
     - Minimal fix committed (build and tests pass)

   Blocked: none

   Next: automation: workflow pushes branch and opens PR closing #<N>
   ```

   **Bug out of safe scope:**
   ```
   === Issue Session (Stage 4c) — Completed ===

   Produced:
     - needs-human label applied to issue #<N>
     - Comment posted on #<N> explaining why the fix is out of safe scope

   Blocked: #<N> — fix requires out-of-scope changes (contract modification, broad refactor, or new dependency)

   Next: human: triage and decide how to proceed
   ```

   **Question answered:**
   ```
   === Issue Session (Stage 4c) — Completed ===

   Produced:
     - Reply comment posted on issue #<N>
     - answered label applied to #<N>

   Blocked: none

   Next: nothing
   ```

   **Unlabelled or other:**
   ```
   === Issue Session (Stage 4c) — Completed ===

   Produced:
     - Comment posted on issue #<N> requesting a bug or question label

   Blocked: #<N> — cannot route without a bug or question label

   Next: human: apply the correct label
   ```

5. **Terminate the session.** Immediately after the exit block, invoke the host
   runtime's session-close API if exposed; otherwise halt. The workflow opens a
   PR if code changed (see RULEBOOK — Session Termination).

## Scope Check (bugs only)

Before making any change, the agent verifies:
- Only files directly related to the bug are touched
- No unrelated refactoring
- No new dependencies without approval
- No contract modifications

If the fix requires out-of-scope changes, the agent posts a comment and adds
`needs-human` label instead of proceeding.

**Interaction with the Reuse & Refactor Check.** When the check's outcome is
**reuse via refactor** and the refactor would exceed the safe-scope rules
above (e.g. it touches files unrelated to the bug, changes a public API, or
modifies a contract), the fix is **not** implemented here and is **not**
turned into a new refactor task in Issue Session. Refactor tasks are the
responsibility of Feature Design. Instead, route the issue to the existing
`needs-human` path: add the `needs-human` label, post a comment stating the
bug's root cause, the reuse-via-refactor outcome, and the reason the refactor
cannot fit in-session, and let a human triage.

## Rules

- Narrow scope only — fix exactly what the issue describes, nothing more
- Always post a comment before starting and after finishing
- If in doubt — stop and ask via a comment rather than guessing
- Contract changes always require human approval
- **Reuse & Refactor Check is mandatory for bug fixes.** Every bug-fix commit
  body must include the canonical `Reuse:` trailer recording one of the
  outcomes defined in `skills/refactor-assessment.md` (as-is / via-refactor /
  none / n/a). Bug-fix commits without the trailer are invalid.
- **Out-of-scope reuse outcomes route to `needs-human`.** A `reuse via
  refactor` outcome whose refactor exceeds the Scope Check does not become a
  new refactor task in Issue Session (that is the job of Feature Design) —
  apply `needs-human` and post the triage comment instead.
- **Inline status updates**: this skill only applies `answered` (not a pipeline label).
  If a future change adds a pipeline label transition here, it must include an inline
  project status update following `set-issue-status.md` — hard-fail if
  `AGENTIC_PROJECT_ID` is not set
