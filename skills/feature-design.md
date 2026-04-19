---
name: feature-design
description: Decomposes a Feature issue into ordered Task sub-issues that cover every acceptance criterion, creates the feature branch, and hands off to Dev Session via the in-development label. Use when GitHub Actions triggers this session automatically on a Feature issue receiving the in-design label — never run interactively.
category: Session
triggers: "automation: in-design"
loads:
  - session-init
  - gh-agentic-tool
  - set-issue-status
  - refactor-assessment
  - session-exit
emits-exit-block: true
exit-hands-to: "automation: dev-session (in-development label)"
---

# Feature Design — Stage 3

## ⛔ Automation-Only — Do Not Execute Interactively

This session is triggered exclusively by GitHub Actions when a Feature issue receives
the `in-design` label. It must never be run manually by an agent in an interactive session.

If you are reading this skill in an interactive session, stop immediately and print:

```
REFUSED: Feature Design is automation-only.
It runs automatically when in-design is applied.
Do not execute this session interactively.
```

Do not proceed past this point in an interactive context.

---

## Purpose

Decompose a Feature into ordered Task sub-issues and create the feature branch.

## When it Runs

Triggered automatically by GitHub Actions when a Feature issue is labelled `in-design`.

## What the Agent Does

1. Reads project context and the Feature issue in full
2. Extracts the `## Acceptance Criteria` from the feature issue and lists each criterion — stops if none found
3. Analyses the codebase to understand what exists and what must be built
4. **Refactor Assessment.** Invoke skills/refactor-assessment.md before writing any new code.
   For **each capability the feature will add** (each new function, type,
   module, or schema implied by the scope), record one of the three outcomes
   defined in `refactor-assessment.md`:
   - **Reuse as-is** — the feature will call an existing symbol; no refactor
     task is needed. Record the reference.
   - **Reuse via refactor** — an existing symbol nearly covers the need.
     Emit an **ordered refactor task** at the top of the task list, ahead of
     every feature task that depends on it. The refactor task's body names
     the symbol being extended and the feature tasks it unblocks.
   - **Do not reuse** — the motivation (why the existing code is genuinely
     unsuitable) is captured as a design-artefact note in this session's exit
     block **and** echoed into the body of the feature task that introduces
     the new symbol, using the canonical `Reuse: none — <reason>` annotation.
   If no candidate existed to consider (e.g. a brand-new capability with no
   adjacent code), record it explicitly as a "no refactor needed" note naming
   the scope searched (keywords, symbols, files inspected). Task emission is
   blocked until the assessment has been performed and recorded.
5. Creates Task sub-issues under the Feature (ordered by implementation sequence), ensuring every acceptance criterion is covered by at least one task. If the Refactor Assessment emitted a refactor task, it is the first task in the order.
6. Verifies full criteria-to-task coverage before proceeding
7. Creates the feature branch: `feature/<N>-<description>`
8. Applies `in-development` label on the Feature issue.
   **Inline status update** — immediately after applying the `in-development` label, set
   the feature's project status to `In Development` following the pattern in
   `set-issue-status.md`:
   - Verify `AGENTIC_PROJECT_ID` is set — hard-fail if not
   - Resolve the issue node ID
   - Find or create the project item
   - Resolve the Status field and option IDs
   - Set status to `In Development`
9. Emits the canonical exit block (see `skills/session-exit.md`). The
   `Produced` section includes a line summarising the Refactor Assessment
   outcome. Shape:

   ```
   === Feature Design Session — Completed ===

   Produced:
     - M task sub-issues created (#N–#N) under Feature #N
     - Feature branch feature/N-<description> created
     - Acceptance-criteria-to-task coverage verified (all K criteria mapped)
     - in-development label applied to Feature #N
     - Refactor Assessment: <N refactor tasks emitted | no refactor needed — scope searched: <list>>

   Blocked: none

   Next: automation: dev-session (in-development label on #N)
   ```

10. **Terminate the session.** Immediately after the exit block, invoke the host
    runtime's session-close API if exposed; otherwise halt. No code is written,
    no PR is opened (see RULEBOOK — Session Termination).

## Task Issue Format

Each task issue contains:
- Specific implementation work to perform
- List of files to create or change
- Acceptance criteria (testable conditions)
- **Acceptance criteria coverage** — which feature-level acceptance criterion(a) the task satisfies

## Rules

- Tasks must be ordered — each must be completable independently in sequence
- Every task that adds logic must include a test task or test requirement
- Every acceptance criterion from the feature issue must map to at least one task
- Do not proceed to branch creation until full criteria-to-task coverage is verified
- Do not begin implementation — design only
- Never push files or open a PR in this session
- **Task emission is blocked until the Refactor Assessment (step 4) has been
  performed and its outcome recorded in the design output.** "I didn't look"
  is not a permitted outcome — see `skills/refactor-assessment.md` for the
  procedure, the three permitted outcomes, and the canonical recording format.

## Next Step

The Dev Session triggers automatically when `in-development` is applied.
