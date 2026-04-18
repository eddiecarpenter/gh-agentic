---
name: pr-review-session
description: Processes inline review comments on a PR — answers questions, implements change requests with tests, and escalates ambiguous or scope-changing feedback via the needs-foreground-review label. Use when GitHub Actions triggers this session automatically on a PR review being submitted — never run interactively.
category: Session
triggers: "automation: pr-review-submitted"
loads:
  - session-init
  - gh-agentic-tool
  - session-exit
  - notify-user
emits-exit-block: true
exit-hands-to: "automation: github-actions pushes fixes if any | human: re-review PR"
---

# PR Review Session — Stage 4b

## ⛔ Automation-Only — Do Not Execute Interactively

This session is triggered exclusively by GitHub Actions when a PR review is submitted.
It must never be run manually by an agent in an interactive session.

If you are reading this skill in an interactive session, stop immediately and print:

```
REFUSED: PR Review Session is automation-only.
It runs automatically when a PR review is submitted.
Do not execute this session interactively.
```

Do not proceed past this point in an interactive context.

---

## Purpose

Process review comments left on a PR by a human reviewer.
Answers questions inline and fixes reported issues — all without human intervention.

## When it Runs

Triggered automatically by GitHub Actions when a PR review is submitted with
`CHANGES_REQUESTED` or `COMMENTED` state on a feature branch.

## What the Agent Does

1. Verifies it is on the correct feature branch
2. Fetches all inline review comments from the PR
3. Classifies each comment into one of three categories:
   - **Questions**: answered with an inline reply — no code changes
   - **Change requests / bug reports**: implemented, tested, committed, and replied to
   - **Ambiguous or scope-changing feedback**: when the agent cannot resolve the comment
     with a simple fix (e.g. the fix requires a contract change, broad refactor, or the
     intent is unclear):
     1. Posts a GitHub comment explaining what it cannot resolve and why
     2. Applies the `needs-foreground-review` label to the PR
     3. Exits immediately without making any code changes
4. Emits the canonical exit block (see `skills/session-exit.md`). Match the
   actual outcome; all variants conform to the same shape.

   **Comments resolved (some code changed):**
   ```
   === PR Review Session (Stage 4b) — Completed ===

   Produced:
     - M inline comments resolved on PR #N
     - K questions answered inline
     - J change requests implemented with tests
     - L commits added to feature/<N>-<description>

   Blocked: none

   Next: automation: workflow pushes fixes; human: re-review PR #N
   ```

   **Comments resolved (no code changed — questions only):**
   ```
   === PR Review Session (Stage 4b) — Completed ===

   Produced:
     - M inline comments answered on PR #N

   Blocked: none

   Next: human: re-review PR #N
   ```

   **Escalation (ambiguous or scope-changing feedback):**
   ```
   === PR Review Session (Stage 4b) — Completed ===

   Produced:
     - needs-foreground-review label applied to PR #N
     - Escalation comment posted on PR #N explaining what could not be resolved

   Blocked: PR #N — feedback requires human judgement (contract change, broad refactor, or ambiguous intent)

   Next: human: run Foreground Recovery recipe
   ```

5. **Terminate the session.** Immediately after the exit block, invoke the host
   runtime's session-close API if exposed; otherwise halt. The workflow pushes
   any code changes after the session exits (see RULEBOOK — Session Termination).

## Rules

- Process ALL unresolved comments — do not skip any
- When in doubt, treat a comment as a change request
- Build and test before committing any fix
- Never merge the PR — leave that for human review
- If a fix requires a contract change or broad refactor, escalate: post a comment, apply `needs-foreground-review`, and exit without changes
- When feedback is ambiguous or scope-changing and cannot be resolved with a simple fix, always escalate rather than guessing
- **Inline status updates**: this skill only applies `needs-foreground-review` (not a
  pipeline label). If a future change adds a pipeline label transition here, it must
  include an inline project status update following `set-issue-status.md` — hard-fail
  if `AGENTIC_PROJECT_ID` is not set

## Notification

Before exiting, notify the user: "PR #N has been updated — please re-review and merge if approved."

## Next Step

After the agent pushes its fixes, the human re-reviews the PR.
If approved, the human merges.
