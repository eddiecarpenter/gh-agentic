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
  - capture-design-plan
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
5. **Publish Design Plan comment.** Load `skills/capture-design-plan.md` and
   fill the canonical template using the outputs of steps 2–4 (acceptance
   criteria, codebase analysis, Refactor Assessment). Because Task sub-issues
   do not yet exist, the `## Tasks` section lists planned task titles with
   `[planned]` placeholders — not `#N` references. Post the filled comment on
   the Feature issue:

   ```bash
   gh issue comment --repo <repo> <feature-number> --body-file <tmp-plan>.md
   ```

   Capture the comment URL returned by `gh issue comment` — it is required
   for the exit block (step 10) and for the amend step (step 7).

   **Halt-on-failure.** If the `gh issue comment` call exits non-zero, the
   design session halts immediately. Print the exact error output followed
   by:

   ```
   REFUSED: Design Plan publication failed — halting before task creation
   ```

   When publication fails, the session **does not proceed to Task creation**,
   **does not create the feature branch**, and **does not apply the
   `in-development` label**. Task emission is blocked until the comment has
   been successfully published and its URL captured; `gh issue comment`
   failure halts the session with a diagnostic — it does not retry silently.
6. Creates Task sub-issues under the Feature (ordered by implementation sequence), ensuring every acceptance criterion is covered by at least one task. If the Refactor Assessment emitted a refactor task, it is the first task in the order.
7. **Amend Design Plan comment with `#N` references.** Append-only: append a
   `### Tasks (created)` subsection to the original Design Plan comment
   listing each created `#N` mapped to its planned title. Use the GitHub
   API edit approach to preserve the original comment as the single
   canonical artefact:

   ```bash
   gh api --method PATCH \
     -H "Accept: application/vnd.github+json" \
     /repos/<owner>/<repo>/issues/comments/<comment-id> \
     -f body="$(cat <tmp-amended-plan>.md)"
   ```

   where `<tmp-amended-plan>.md` is the original plan body with the new
   `### Tasks (created)` subsection appended at the end of the existing
   `## Tasks` section. **Never rewrite prior sections** — the amend is
   strictly append-only so the audit trail between the original `[planned]`
   plan and the created tasks is preserved (see
   `skills/capture-design-plan.md` Rules).
8. Verifies full criteria-to-task coverage before proceeding
9. Creates the feature branch: `feature/<N>-<description>`
10. Applies `in-development` label on the Feature issue.
    **Inline status update** — immediately after applying the `in-development` label, set
    the feature's project status to `In Development` following the pattern in
    `set-issue-status.md`:
    - Verify `AGENTIC_PROJECT_ID` is set — hard-fail if not
    - Resolve the issue node ID
    - Find or create the project item
    - Resolve the Status field and option IDs
    - Set status to `In Development`
11. Emits the canonical exit block (see `skills/session-exit.md`). The
    `Produced` section includes a line summarising the Refactor Assessment
    outcome and a line with the Design Plan comment URL. Shape:

    ```
    === Feature Design Session — Completed ===

    Produced:
      - Design Plan comment: <url>
      - M task sub-issues created (#N–#N) under Feature #N
      - Feature branch feature/N-<description> created
      - Acceptance-criteria-to-task coverage verified (all K criteria mapped)
      - in-development label applied to Feature #N
      - Refactor Assessment: <N refactor tasks emitted | no refactor needed — scope searched: <list>>

    Blocked: none

    Next: automation: dev-session (in-development label on #N)
    ```

12. **Terminate the session.** Immediately after the exit block, invoke the host
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
- **Task emission is blocked until the Design Plan comment (step 5) has been
  successfully published and its URL captured.** `gh issue comment` failure
  halts the session with a diagnostic — it does not retry silently. On
  publish failure the session does not create tasks, does not create the
  feature branch, and does not apply the `in-development` label. See
  `skills/capture-design-plan.md` for the comment template.

## Next Step

The Dev Session triggers automatically when `in-development` is applied.
