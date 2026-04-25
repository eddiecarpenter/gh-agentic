---
name: dev-session
description: Implements every open Task sub-issue on the feature branch in order, commits per task, verifies acceptance criteria coverage, and exits cleanly so the workflow can open the PR. Use when GitHub Actions triggers this session automatically on a Feature issue receiving the in-development label — never run interactively.
category: Session
triggers: "automation: in-development"
loads:
  - session-init
  - gh-agentic-tool
  - set-issue-status
  - refactor-assessment
  - session-exit
  - notify-user
emits-exit-block: true
exit-hands-to: "automation: github-actions pushes branch and opens PR"
---

# Dev Session — Stage 4

## ⛔ Automation-Only — Do Not Execute Interactively

This session is triggered exclusively by GitHub Actions when a Feature issue receives
the `in-development` label. It must never be run manually by an agent in an interactive session.

If you are reading this skill in an interactive session, stop immediately and print:

```
REFUSED: Dev Session is automation-only.
It runs automatically when in-development is applied.
Do not execute this session interactively.
```

Do not proceed past this point in an interactive context.

---

## Purpose

Implement all open Task sub-issues on the feature branch, in order.

## When it Runs

Triggered automatically by GitHub Actions when a Feature issue is labelled `in-development`.

## What the Agent Does

1. Verifies it is on the correct feature branch — never works on main
2. Reads the Feature issue and extracts acceptance criteria for end-of-session verification
3. Queries open Task sub-issues on the Feature, ordered by issue number
4. **Recovery detection** — before processing tasks, check for `recovery.md`:
   - If `recovery.md` **does not exist**: proceed as a fresh start — no recovery
     behaviour occurs. Continue to step 5.
   - If `recovery.md` **exists**: read it and perform the branch mismatch check:
     - Compare the `Branch` field in `recovery.md` with `git branch --show-current`
     - If they **do not match**: warn the human with the mismatch details and ask
       whether to (a) treat this as a fresh start and overwrite `recovery.md`, or
       (b) stop so the human can investigate. **Do not proceed until the human responds.**
     - If they **match**: enter **recovery mode**:
       - Parse the `## Completed Tasks` section to get the list of already-completed
         task issue numbers
       - Verify each completed task's GitHub issue is actually closed
       - Log: `Recovery mode active — skipping N completed tasks: #X, #Y, #Z`
       - Skip those tasks in step 5 — resume from the first incomplete task
5. For each Task in order (skipping tasks completed in recovery mode):
   - Reads the task issue and understands what must be built
   - **Reuse & Refactor Check.** Invoke skills/refactor-assessment.md before writing any new code. For **each new function, type, module, or schema** the task will introduce, record one of the three outcomes defined in `refactor-assessment.md` (as-is / via-refactor / none). If the task introduces no new symbols, record `n/a` with a one-line description. **The check must be performed before any new symbol is written.** If the agent finds it has not performed the check, the session **halts** for that task and reports the error — silent progression is forbidden. "I didn't look" is not a permitted outcome.
   - Implements the work described — after each significant step (file created,
     function complete, tests written), writes an intra-task checkpoint to `recovery.md`
     with a `## Current Task` section and pushes immediately:
     ```bash
     git add recovery.md
     git commit -m "chore: recovery checkpoint — <brief description> (#feature-issue)"
     git push
     echo "=== Intra-task checkpoint pushed ==="
     ```
   - When a complete unit of work is done (a module and its tests written and
     passing), commits and pushes the code immediately — does not wait for the
     full task to be complete. The commit body carries the canonical `Reuse:`
     trailer for any new symbols introduced by the unit:
     ```bash
     git add -A
     git commit -m "$(cat <<'MSG'
     feat: <unit description> (#feature-issue)

     <optional short paragraph>

     Reuse: <as-is|via-refactor|none|n/a> — <reason or reference>
     MSG
     )"
     git push
     echo "=== Unit committed and pushed ==="
     ```
     Then updates `recovery.md` to reflect the completed unit and pushes again.
   - Builds and tests — stops immediately on failure and reports the exact error
   - Commits the task. The subject is unchanged; the body carries the
     canonical `Reuse:` trailer recording the outcome of the Reuse & Refactor
     Check (one trailer per distinct decision if the task introduced multiple
     unrelated new symbols):
     ```
     feat: [task description] — task N of N (#feature-issue)

     <optional short paragraph>

     Reuse: <as-is|via-refactor|none|n/a> — <reason or reference>
     ```
   - Closes the task issue
   - **Writes `recovery.md`** — after the commit and close, write `recovery.md` to
     the repo root with the current progress state (see format below), then:
     ```bash
     git add recovery.md
     git commit -m "chore: update recovery.md — task N of N (#feature-issue)"
     git push
     echo "=== Checkpoint saved — task N of N complete, recovery.md pushed ==="
     ```
     This must happen *after* each successful task commit and *before* starting the
     next task. It ensures that if the session dies, the next session can resume from
     the recorded state.
6. Verifies each acceptance criterion has test coverage — stops if any criterion is uncovered
7. **Archive `recovery.md`** — after all tasks are complete and criteria verified:
   - If `recovery.md` **does not exist** (e.g. a single-task feature that completed on
     first run before any recovery write, or the file was never created): skip archival
     gracefully — do not fail.
   - If `recovery.md` **exists**:
     ```bash
     mkdir -p recovery-logs
     git mv recovery.md recovery-logs/recovery-log-<feature-issue-number>.md
     git add recovery-logs/
     git commit -m "chore: archive recovery.md for #<feature-issue-number>"
     ```
   - This must happen *before* the branch is pushed and the PR is opened.
8. When all tasks are closed and criteria verified, emit the canonical exit
   block (see `skills/session-exit.md`):

   ```
   === Dev Session — Completed ===

   Produced:
     - M tasks closed (#N–#N)
     - M commits on feature/<N>-<description>
     - Acceptance criteria coverage verified (K of K)
     - recovery.md archived to recovery-logs/recovery-log-<N>.md

   Blocked: none

   Next: automation: workflow pushes branch and opens PR closing #<N>
   ```

9. **Terminate the session.** Immediately after the exit block, invoke the host
   runtime's session-close API if exposed; otherwise halt. The workflow pushes
   the branch and opens the PR automatically — no further work in this session
   (see RULEBOOK — Session Termination).

## recovery.md Format

After each task commit, write `recovery.md` to the repo root with exactly this structure:

```markdown
# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #<feature-issue-number>            |
| Branch              | <current-branch-name>              |
| Last commit         | <git rev-parse --short HEAD>       |
| Total tasks         | <total-task-count>                 |
| Last updated        | <ISO 8601 timestamp>               |

## Completed Tasks

### #<issue-number> — <task-title>
- **Implemented:** <one or two sentences describing what was built>
- **Files changed:** <comma-separated list of key files>
- **Decisions:** <any decisions made that affect remaining tasks, or "None">

## Remaining Tasks

- [ ] #<issue-number> — <task-title> ← current
- [ ] #<issue-number> — <task-title>
```

**Field definitions:**
- **Feature issue** — the parent Feature issue number (e.g. `#197`)
- **Branch** — the current branch name from `git branch --show-current`
- **Last commit** — short SHA from `git rev-parse --short HEAD`
- **Total tasks** — the total number of Task sub-issues at session start
- **Last updated** — ISO 8601 timestamp (`date -u +%Y-%m-%dT%H:%M:%SZ`)
- **Completed Tasks** — each task as a subsection with implementation summary, files changed, and decisions
- **Current Task** — present during task execution only; updated after each significant implementation step with status, last step, next step, and any notes for a recovering session
- **Remaining Tasks** — each task not yet completed; mark the next task with `← current`

### Current Task format (intra-task checkpoints)

During task implementation, replace or add the `## Current Task` section:

```markdown
## Current Task

### #<issue-number> — <task-title>
- **Status:** in-progress
- **Last step:** <what was just completed, e.g. "created X.go", "wrote tests for Y">
- **Next step:** <what comes next>
- **Notes:** <anything a recovering session needs to know, or "None">
```

The `Notes` field must record the state of the Reuse & Refactor Check for
the current task so a recovering session knows whether to re-run it. Use one
of:

- `Reuse check pending` — the task is underway but the check has not yet been
  performed.
- `Reuse check complete — outcome: <as-is|via-refactor|none|n/a> — <reason>` —
  the check has been performed and the outcome is captured (and will appear
  in the eventual task-commit trailer).

Remove the `## Current Task` section when writing the post-task checkpoint (task complete).

## Rules

- Never commit on main
- Never skip a failing test — fix it before moving to the next task
- Never claim a task complete without running build and tests
- A feature is not complete until all acceptance criteria have test coverage
- Report exact command output on any failure
- Follow the standards in `standards/<stack>.md` exactly
- **Reuse & Refactor Check is mandatory per task.** No new function, type,
  module, or schema may be committed without an accompanying `Reuse:` trailer
  in the commit body recording one of the outcomes defined in
  `skills/refactor-assessment.md` (as-is / via-refactor / none / n/a). If the
  check was not performed, the session halts — "I didn't look" is not a
  permitted outcome.
- **Inline status updates**: this skill does not apply pipeline labels (the workflow
  applies `in-review`). If a future change adds a pipeline label transition here, it
  must include an inline project status update following `set-issue-status.md` —
  hard-fail if `AGENTIC_PROJECT_ID` is not set

## Notification

Before exiting, notify the user: "PR #N is ready for your review."

## Next Step

The workflow pushes the branch and opens a PR with `Closes #N`.
Human review happens in the PR. If review comments need addressing, the
**PR Review Session (Stage 4b)** recipe handles that.
