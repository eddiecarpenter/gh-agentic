---
name: dev-session
description: Implements a Feature's tasks in order — reading the rationale comment and the ordered Task sub-issues, then walking each open task (read body, implement with reuse discipline, run tests, commit with the prescribed message format, push, close the task issue) until all tasks are done. On exit, the surrounding GitHub Actions workflow opens the PR and transitions the Feature to in-review. Use when workflow automation fires on in-development label apply against a Feature whose feature-design phase has produced a rationale, ordered tasks, and a feature branch. Headless only; humans running implementation interactively use this skill as a guide.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/step-skip-rule.md
  - skills/gh-agentic/SKILL.md
  - skills/apply-label/SKILL.md
emits-exit-block: true
exit-hands-to: "automation: workflow pushes (no-op if agent already pushed), opens PR, applies in-review, sets project status"
---

# Dev Session

## Goal

Take a designed Feature (in-development label, branch on the active
repo, ordered Task sub-issues, rationale comment) and implement its
tasks one by one, leaving the work committed and pushed on the
feature branch with every Task sub-issue closed. The surrounding
GitHub Actions workflow opens the PR and applies `in-review` after
this skill exits.

The skill is headless — invoked by workflow automation when the
`in-development` label is applied. Humans implementing a Feature
interactively use this skill as a reference; they do not invoke it.

## Output Artefacts

- **N task-shaped commits** on the feature branch (`feature/<N>-<slug>`),
  one per task, in task order. Each commit:
  - Subject: `feat: <task description> — task <K> of <M> (#<feature>)`
  - Trailer: `Reuse: <as-is | refactor — <one-line>> | opt-out — <reason>>`
- **All N task sub-issues closed** as their commits land.
- **Branch pushed to origin** after each commit (per-task push).
- **No label transitions** — the surrounding workflow applies
  `in-review` after opening the PR. This skill never applies
  `in-review` itself.
- **No PR created** — the surrounding workflow opens it.

A return value at exit summarising the run:
```
{ repo: <string>, feature: <int>, branch: <string>,
  tasks_total: <int>, tasks_completed_this_run: <int>,
  exit_state: "completed" | "no-op" | "blocked" }
```

`exit_state`:
- `completed` — all tasks closed by end of session.
- `no-op` — entered with all tasks already closed (re-run case).
- `blocked` — the agent could not complete a task (build/test
  refused to pass, ambiguous task body, missing dependency); the
  Feature stays at `in-development` and the human takes over.

The skill's four valid terminal outputs:

**A. Completed.** Walked through K open tasks, all committed,
pushed, closed. Feature still at `in-development`; the workflow's
post-agent steps will push (no-op) and open the PR.

**B. Resumed and completed.** Some tasks were already closed on
entry (a prior run committed them); walked the remaining open ones
and finished. Same exit shape as A.

**C. Re-run no-op.** Entered with zero open tasks. Nothing to do;
exit cleanly. The workflow's PR-creation step is idempotent (`gh pr
create` no-ops when a PR already exists for the branch).

**D. Blocked.** The agent could not finish one of the tasks. Surface
the failure clearly in the exit block (which task, what failed,
what was tried). Feature stays at `in-development`. A human picks
up — typically by editing the task body to clarify, fixing the
underlying environment issue, or re-running once the blocker
clears.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy applied
  to `INVALID_DEV_STATE`, `BRANCH_MISSING`, `TASK_BLOCKED`,
  `TEST_FAILED_PERSISTENT`, `BUILD_FAILED_PERSISTENT`.
- `skills/definitions/verification-procedure.md` — change-pinning
  rule (every commit is verified by reading `git log` after the
  fact; tests are verified by re-running, not by inferring success
  from compile output).
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement
  rule preventing silent skipping. The conditional-step carve-out
  applies to the resume case (already-closed tasks are skipped
  per their state, which is the intended behaviour, not a violation
  of the skip rule).

## Dependencies

- `skills/gh-agentic/SKILL.md` — used in step 2 to read the Feature's
  state (`gh agentic status feature <N> --raw`) and in step 4 to
  list the Task sub-issues.
- `skills/apply-label/SKILL.md` — used in step 5 to claim the
  `development-in-progress` slot and in step 17 to release it.

This skill does NOT load `apply-label` for the Feature's lifecycle
labels — `in-development` → `in-review` is owned by the surrounding
workflow.

## Steps

The **step-skip rule** from `skills/definitions/step-skip-rule.md`
applies. The resume carve-out is the exception: skipping an
already-closed task in the per-task walk is the rule, not a
violation.

**Resolving the active repo.** Resolve once in step 1 via:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

and reuse as `<active-repo>`.

**Concurrency guard (the `development-in-progress` beacon).** Same
pattern as `feature-design`'s `design-in-progress`. Step 5 claims
it; step 17 releases it; the Error Handling section makes
best-effort release universal across error paths.

**Per-task discipline.** Steps 8–14 run once per open task in
order. The discipline:

- **Reuse outcome (mandatory).** Before writing any new function,
  type, module, or schema, record one of three outcomes:
  - `reuse — as-is` — an existing symbol covers the need.
  - `reuse — refactor — <one-line>` — extended an existing symbol.
  - `reuse — opt-out — <reason>` — genuinely new; existing code
    is unsuitable for a stated reason.

  The outcome is written into the commit trailer (step 12). *"I
  didn't look"* is never permitted.

- **Tests are first-class.**
  - All existing tests must pass after the changes.
  - New code must have tests covering success, failure, and edge
    cases.
  - Modified functionality must have its tests updated to match.
  - Tests must be **executed** — writing them without running them
    does not count.
  - The build must pass cleanly before the commit.

  If a test or build fails, fix it before committing. There is no
  retry cap; keep working until passing. If genuinely stuck (no
  forward progress on consecutive attempts, environment issue,
  ambiguous task), raise `TASK_BLOCKED` and exit (Output D).

- **Commit format (per RULEBOOK).**
  ```
  feat: <task description> — task <K> of <M> (#<feature>)

  <optional body>

  Reuse: <outcome>
  ```

- **Push after every commit.** No batching. Each task lands as a
  separate commit on the remote, in order.

---

### Section A — Setup

1. **Announce the session.** Print the banner verbatim before any
   tool call:

   ```
   ==========================================================
   === Dev Session — Started                                  ===
   ==========================================================
   ```

   Resolve the active repo per the rule above and hold as
   `<active-repo>`.

2. **Read the Feature.** Query the full state:

   ```bash
   gh agentic status feature <N> --raw
   ```

   Capture title, body, labels, branch metadata. Hold as `<feature>`.

   On non-zero exit → raise `INVALID_DEV_STATE` (`ERROR`) — the
   Feature does not exist or `gh agentic` is unavailable.

3. **Validate state.** Inspect `<feature>.labels`:

   - MUST contain `feature` and `in-development`.
   - MUST NOT contain any of `in-review`, `done`. If present →
     raise `INVALID_DEV_STATE`. The Feature is past dev; this
     skill should not have been invoked.
   - MUST NOT contain `in-design`, `interactive-design`, or
     `designed`. If present → raise `INVALID_DEV_STATE`. The
     Feature has not been triggered for implementation; the design
     phase is not done.

   Read the rationale: query the Feature's comments for the one
   starting with `<!-- design-plan:v1 -->` (set by `feature-design`).

   ```bash
   gh issue view <N> --repo <active-repo> --json comments \
     --jq '[.comments[] | select(.body | startswith("<!-- design-plan:v1 -->"))]
            | .[0].body'
   ```

   If missing → raise `INVALID_DEV_STATE`. Without the rationale,
   the agent has no architectural context to implement against.

4. **Read the task list.** Query the Feature's child sub-issues
   labelled `task`:

   ```bash
   gh issue list --repo <active-repo> --label task \
     --search "parent:<N>" --state all \
     --json number,title,body,state \
     --jq 'sort_by(.number)'
   ```

   The result is the ordered list (creation order = execution order
   per `feature-design` step 17). Capture as `<tasks>`. Each entry
   has `state ∈ {OPEN, CLOSED}`.

   If `<tasks>` is empty → raise `INVALID_DEV_STATE`. The Feature
   has no tasks; design was not run or was malformed.

5. **Concurrency guard — claim the slot.** Probe for
   `development-in-progress` on the Feature:

   ```bash
   gh issue view <N> --repo <active-repo> --json labels \
     --jq '[.labels[].name] | index("development-in-progress")'
   ```

   - **Set** → another dev session is in flight. Exit cleanly with
     a one-line note: "Another dev session is active; this run is
     a no-op." Do NOT remove the label (it belongs to the other
     session). Do NOT proceed.
   - **Not set** → claim it:

     ```
     apply-label(repo=<active-repo>, issue=<N>,
                 add=["development-in-progress"], remove=[])
     ```

     On failure → raise `INVALID_DEV_STATE` (`ERROR`); exit before
     touching the branch. From this point on, every exit path
     (success, blocked, error) MUST best-effort release the label.

6. **Check out the branch.** From the local repo:

   ```bash
   git fetch origin
   BRANCH="feature/<N>-$(<feature>.title | slugify | head -c 30)"
   git checkout -B "$BRANCH" "origin/$BRANCH"
   ```

   `feature-design` created and pushed the branch (per its step 13).
   If `git checkout` fails (no remote branch) → raise
   `BRANCH_MISSING` (`ERROR`). The Feature should never have been
   triggered for implementation without a branch.

   Verify the branch tip is at or ahead of `origin/main` (no rebase
   surprises). If behind, fast-forward:

   ```bash
   git merge --ff-only origin/main || true
   ```

   Conflicts here are not handled by the skill — raise
   `BRANCH_MISSING` with the underlying error and let a human
   reconcile.

---

### Section B — Task walk

7. **Filter to open tasks.** Compute `<open-tasks>` =
   `[t for t in <tasks> if t.state == OPEN]`, preserving order.

   - **`<open-tasks>` empty** → all tasks already closed. Skip the
     walk; emit Output C in step 18.
   - **Non-empty** → walk through.

8. **Per-task — read.** For the next open task `T_K` (where K is
   its position in `<tasks>` and M is `len(<tasks>)`):

   - Render the task body in the response stream so the reader of
     the run log can see the spec being worked on:
     ```
     === Task <K> of <M>: #<T_K> <title> ===
     <body>
     ```
   - Hold the title and body as the working spec.

9. **Per-task — implement.** Make the code changes the task
   describes. Apply the **reuse discipline** from the preamble:
   record the outcome you'll write into the commit trailer.

   - Files outside the task's scope are off-limits. If the task
     body is ambiguous about scope, lean narrow.
   - If during implementation it becomes clear the task is
     unbuildable as written (the rationale or the dependency chain
     is wrong), raise `TASK_BLOCKED` (`WARN`) and exit with Output D.
     Do NOT silently re-scope a task; that's a human-review event.

10. **Per-task — verify tests.** Run the project's test suite
    against the changed code.

    - **All pass** → continue to step 11.
    - **Failures** → diagnose, fix, re-run. Loop until passing.
      If the loop is making no forward progress (same failure
      across consecutive attempts, environment-shaped errors, etc.),
      raise `TEST_FAILED_PERSISTENT` (`ERROR`) and exit with
      Output D.

    The agent must not commit with failing tests. Per RULEBOOK's
    universal testing rules:
    - New logic requires new tests covering success, failure, edge
      cases.
    - Modified logic requires updated tests.
    - Tests that don't run don't count.

11. **Per-task — verify build.** Run the project's build against
    the changed code.

    - **Pass** → continue to step 12.
    - **Fail** → diagnose, fix, re-run. If genuinely stuck, raise
      `BUILD_FAILED_PERSISTENT` (`ERROR`) and exit with Output D.

12. **Per-task — commit.** Use the canonical format:

    ```bash
    git add <files>
    git commit -m "feat: <task description> — task <K> of <M> (#<feature>)

    <optional body>

    Reuse: <outcome>"
    ```

    Verify the commit landed:

    ```bash
    git log -1 --format='%s%n%b'
    ```

    The subject MUST match the format above; the trailer MUST start
    with `Reuse:`. If either is wrong, amend the commit before
    proceeding (this is the one place amend is permitted — the
    commit just landed, no published history).

13. **Per-task — push.** Per RULEBOOK's modified push rule (this
    skill, not the workflow, is responsible):

    ```bash
    git push origin "$BRANCH"
    ```

    Verify:
    ```bash
    git log -1 origin/$BRANCH --format='%H'
    ```

    matches the local HEAD. On push failure (network, permissions,
    non-fast-forward) → raise `BRANCH_MISSING` (`ERROR`) and exit
    with Output D. Do NOT close the task issue if the push didn't
    land — the task remains open so a re-run can re-push the local
    commit.

14. **Per-task — close the task issue.**

    ```bash
    gh issue close <T_K> --repo <active-repo>
    ```

    Verify the task is closed:
    ```bash
    gh issue view <T_K> --repo <active-repo> --json state \
      --jq .state
    ```

    Should equal `"CLOSED"`. If not → raise `TASK_BLOCKED` (`WARN`)
    and exit with Output D — the local commit is good, the push is
    good, but the task issue did not close. Surface the partial state.

15. **Loop.** Continue from step 8 with the next open task. When
    `<open-tasks>` is exhausted, fall through to Section C.

---

### Section C — Closeout

16. **Verify completion.** Re-query the task list:

    ```bash
    gh issue list --repo <active-repo> --label task \
      --search "parent:<N>" --state open --json number \
      --jq 'length'
    ```

    Should be 0. If non-zero (a task we thought we closed is still
    open, or a new task appeared mid-session), raise
    `TASK_BLOCKED` (`WARN`) — surface the discrepancy and exit
    Output D.

17. **Release the slot.** Remove the `development-in-progress`
    beacon:

    ```
    apply-label(repo=<active-repo>, issue=<N>,
                add=[], remove=["development-in-progress"])
    ```

    Failures here are surfaced as `WARN` and do not block exit —
    the dev work itself is complete. The stale label can be cleared
    by the human if it sticks.

18. **Emit the exit block.** Match the actual outcome:

    **Output A — Completed:**
    ```
    === Dev Session — Completed ===

    Produced:
      - <K> task commit(s) on feature/<N>-<slug>
      - <K> task issue(s) closed: #<T_a>, #<T_b>, ...
      - All <M> tasks for #<N> are closed

    Blocked: none

    Next: workflow opens the PR and applies in-review
    ```

    **Output B — Resumed and completed:**
    ```
    === Dev Session — Completed (resumed) ===

    Produced (this run):
      - <K> task commit(s) on feature/<N>-<slug>
      - <K> task issue(s) closed: #<T_x>, ...

    Already complete on entry:
      - <M-K> task(s) closed by a prior run

    All <M> tasks for #<N> are closed.

    Next: workflow opens the PR and applies in-review
    ```

    **Output C — Re-run no-op:**
    ```
    === Dev Session — No-op ===

    Entered with zero open tasks. All <M> tasks for #<N> were
    already closed.

    Next: workflow's PR-creation step is idempotent — it will
          open the PR if one does not already exist.
    ```

    **Output D — Blocked:**
    ```
    === Dev Session — Blocked ===

    Completed before block:
      - <K> task commit(s) on feature/<N>-<slug>
      - <K> task issue(s) closed

    Blocked on:
      - Task #<T_block> <title>: <reason — TEST_FAILED_PERSISTENT |
                                         BUILD_FAILED_PERSISTENT |
                                         TASK_BLOCKED | BRANCH_MISSING>
      - <one-paragraph diagnosis: what was tried, what failed>

    Feature #<N> remains at in-development. Tasks #<T_block> and
    later remain open. Human picks up: clarify the task body, fix
    the environment, or take over the implementation manually.
    ```

19. **Terminate the session.** Per `emits-exit-block: true`, invoke
    the host runtime's session-close API if available; otherwise
    halt. The surrounding workflow's post-agent steps run next:
    push (no-op since the agent pushed), `gh pr create` (no-op
    if a PR already exists), apply `in-review`, set project status.

## Verification

Run the framework checks against this skill:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/dev-session/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/dev-session/SKILL.md
```

Pass criteria: both commands exit 0.

### Mechanical checks

Run by `verify-skill-mechanical.py`:

- `all_sections_present` — every mandatory section heading exists.
- `frontmatter_required_fields(name, description, triggers, loads)`.
- `frontmatter_name_valid` — kebab-case, matches filename.
- `description_within_length_limit` — ≤ 1024 chars.
- `description_assertive` — contains "Use when" + assertive clause.
- `description_third_person`.
- `references_resolve` — every `loads:` path resolves to a file.

### Ground-truth checks

Run by `check-description-triggers.py`:

- `description_triggers_appropriately` — phrasings classified per
  the `GROUND_TRUTH` entry for `dev-session`.

## Error Handling

**Slot-release rule (universal).** Every error path AND every
graceful exit AFTER step 5 (the slot was claimed) MUST attempt to
remove `development-in-progress` before exit, on a best-effort
basis. If the removal itself fails, surface it as a `WARN` and exit
anyway — the original error is what matters.

- `INVALID_DEV_STATE` from steps 2–5 (Feature missing, wrong
  labels, no rationale, no tasks, slot-claim failed) → severity
  `ERROR`; propagate. Caller / workflow bug — invoked dev-session
  on a Feature not ready for implementation.
- `BRANCH_MISSING` from step 6 (no remote branch) or step 13 (push
  failed) → severity `ERROR`; propagate. The Feature should never
  have reached `in-development` without a branch; if push fails
  mid-session, the local commit is preserved and a re-run will
  push it.
- `TASK_BLOCKED` from step 9 (task is unimplementable as written),
  step 14 (issue did not close), or step 16 (final task list
  inconsistent) → severity `WARN`; propagate. The Feature stays at
  `in-development`; the human takes over the specific task.
- `TEST_FAILED_PERSISTENT` from step 10 (test loop made no forward
  progress) → severity `ERROR`; propagate with full failure output.
  The agent does not "give up" silently — explicit diagnosis is
  the value.
- `BUILD_FAILED_PERSISTENT` from step 11 (same shape as
  `TEST_FAILED_PERSISTENT` but for the build) → severity `ERROR`;
  propagate.
- All other errors: propagate (default).
