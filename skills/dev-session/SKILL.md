---
name: dev-session
description: Implements a Feature's tasks in order — reading the rationale comment and the ordered Task sub-issues, then walking each open task (read body, implement with reuse discipline, run tests, commit with the prescribed message format, push, close the task issue) until all tasks are done. On exit, the surrounding GitHub Actions workflow opens the PR and transitions the Feature to in-review. Use when workflow automation fires on in-development label apply against a Feature whose feature-design phase has produced a rationale, ordered tasks, and a feature branch. Headless only; humans running implementation interactively use this skill as a guide.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/cp-execution-context.md
  - skills/definitions/concurrency-beacon.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/step-skip-rule.md
  - skills/definitions/commit-discipline.md
  - skills/gh-agentic/SKILL.md
  - skills/apply-label/SKILL.md
emits-exit-block: true
exit-hands-to: "automation: workflow pushes (no-op if agent already pushed), applies in-verification, triggers compliance-verify (Stage 5)"
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
  `in-verification` after pushing. This skill never applies
  `in-verification` itself.
- **No PR created** — the compliance-verify stage (Stage 5) opens
  the PR after all ACs are verified.

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

- `skills/definitions/cp-execution-context.md` — the control-plane
  execution context (#873): cwd is the project code
  (`$AGENTIC_PROJECT_DIR`) where code, commits, and the feature branch
  live; docs, the rulebook, and the Feature / Task issues live on the
  control plane (`$AGENTIC_CP_ROOT`). Read docs from
  `$AGENTIC_CP_ROOT/docs`; route Task / label / comment operations to
  the control plane; commit and push only inside the project; never
  write to the control-plane checkout.
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
order. The reuse outcome, test/build pass-before-commit rule,
commit format, and push-after-commit rule come from
`skills/definitions/commit-discipline.md`. This skill applies that
discipline at task granularity:

- The `<context tag>` in the commit subject is `— task <K> of <M> (#<feature>)`.
- The persistent-failure error codes are `TEST_FAILED_PERSISTENT`,
  `BUILD_FAILED_PERSISTENT`, `TASK_BLOCKED` (raised by this skill
  per the discipline's "fix-before-commit, raise if stuck" rule).

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

6a. **Recovery probe.** A prior dev-session may have left a
    `recovery.md` file at the repo root with mid-task progress. The
    open-vs-closed task state (step 7) is the coarse cursor;
    `recovery.md` is the fine cursor inside a task that was
    interrupted before its closing commit landed.

    ```bash
    test -f recovery.md && cat recovery.md || echo MISSING
    ```

    Parse the file when present. The expected shape:

    ```markdown
    # Dev Session Recovery — Feature #<N>

    ## Completed Tasks
    - #<T_a> — <title>  (commit <sha>)
    - #<T_b> — <title>  (commit <sha>)

    ## Current Task
    Task <K> of <M>: #<T_K> — <title>
    Progress: <one-line status>
    Files in flight: <paths>
    ```

    Cross-check the listed completed tasks against GitHub state:
    each cited issue must actually be closed AND the cited commit
    must be present on the local branch. If both true, treat as
    truly completed (skip in step 7's filter). If either is false,
    the recovery file is stale or a prior run partially failed —
    surface the inconsistency and treat the task as still open.

    Hold the verified completed-set as `<recovered-completed>` and
    the current-task pointer (if any) as `<resume-task>`.

    Missing recovery.md → fresh run; `<recovered-completed>` is
    empty.

6b. **Compliance-feedback probe.** Check whether a prior
    compliance-verify run left feedback on this Feature. This is
    how dev-session knows it is a _fix run_, not a fresh run.

    ```bash
    gh issue view <N> --repo <active-repo> --json comments \
      --jq '[.comments[] | select(.body | startswith("<!-- compliance-feedback:v1 -->"))]
             | last | .body'
    ```

    - **Comment found** → hold as `<compliance-feedback>`. This
      run is a fix run: the agent must address the specific ACs
      listed in the feedback, not re-implement the whole feature.
      Surface the feedback in the session banner:

      ```
      === Compliance Feedback Detected ===
      This is a fix run. The following ACs require work:
      <compliance-feedback excerpt>
      ===================================
      ```

      When walking tasks (Section B), after completing each task
      check whether it addresses a compliance feedback item and
      note it in the commit body.

    - **No comment found** → fresh run; `<compliance-feedback>` is
      null. Normal task walk.

---

### Section B — Task walk

7. **Filter to open tasks.** Compute `<open-tasks>` =
   `[t for t in <tasks> if t.state == OPEN AND t not in <recovered-completed>]`,
   preserving order. The recovery probe (step 6a) trims tasks
   already completed by a prior run.

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

   **Per-unit commits (permitted, not required).** Tasks vary in
   size; for non-trivial tasks the agent MAY commit and push
   intermediate units of work as they become coherent ("a complete
   sub-piece — refactored helper, schema migration, isolated test
   suite"). Each intermediate commit follows the same
   commit-discipline (reuse outcome, tests pass, build pass) and
   uses a body that names what the unit is. The task's CLOSING
   commit (step 12) is still mandatory and is what triggers the
   issue close in step 14. Per-unit commits in between are opt-in
   and reduce the size of the "currently in flight" diff that a
   recovery run would need to re-establish.

   **Recovery checkpoint (between sub-pieces).** After completing
   any intermediate unit, OR at any natural break inside a long
   task, write/update `recovery.md` at the repo root with the
   shape documented in step 6a, then commit + push the recovery
   file as a `chore: recovery checkpoint` commit:

   ```bash
   git add recovery.md
   git commit -m "chore: recovery checkpoint — task <K> of <M>

   Reuse: opt-out — checkpoint metadata, not derived from existing code"
   git push origin "$BRANCH"
   ```

   Recovery checkpoints are independent of per-unit commits — they
   are orientation breadcrumbs, not deliverables. A task with no
   intermediate units may still emit one or more checkpoints. The
   checkpoint commit will be visible in the merged PR and that's
   acceptable; it is the cost of robust resume.

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

15pre. **Build + test gate.** Before any closeout work, run the
    verification gate for every stack the feature touched.

    **Stack detection (multi-stack aware).** The skill enumerates
    which language standards apply to *this Feature's* changes —
    not a project-wide setting — by looking at the diff against
    `origin/main`:

    ```bash
    git diff --name-only origin/main...HEAD
    ```

    Classify each changed path against the stack markers below.
    The set of distinct matches is `<stacks>`:

    | Stack | Marker (any of these matches) |
    |---|---|
    | `go` | path ends in `.go`, OR any `go.mod`/`go.sum` touched |
    | `typescript` | path under a directory containing `package.json`, AND ends in `.ts`, `.tsx`, `.mts`, `.cts`, `.js`, `.jsx`, `.mjs`, `.cjs`, OR `package.json`/`package-lock.json`/`tsconfig*.json` touched |
    | `react` | as `typescript` PLUS file contains JSX (`.tsx`/`.jsx`) under a project that imports React. The `typescript` gate covers React too — see `standards/react.md`; the gate is identical. |
    | `java` | path ends in `.java`, OR any `pom.xml`/`build.gradle*` touched |

    Documentation-only changes (`.md`, `docs/**`) and workflow
    files (`.github/workflows/**`) do not select any gate — they
    are not testable by the language toolchain. A diff that is
    pure docs falls through to "no gate applies for this feature
    — continue to step 15a" (still requires the AC coverage gate;
    docs Features must still satisfy their ACs).

    If `vars.PROJECT_STACK` is set, it overrides the detection —
    use only that stack. Useful for single-stack repos where
    detection adds no value, or for testing.

    **For each stack in `<stacks>`:**

    1. **Manifest pre-check** — confirm the stack's manifest is
       present in the repo before attempting any gate commands:

       | Stack | Probe |
       |---|---|
       | `go` | `test -f go.mod` |
       | `typescript` / `react` | `test -f package.json` (root, or in the closest enclosing directory of every changed JS/TS file) |
       | `java` | `test -f pom.xml` OR `test -f build.gradle` OR `test -f build.gradle.kts` |

       If the manifest is absent, record `<stack>: SKIPPED — no
       manifest` and move on to the next stack. This is not a
       failure; the change is not within a project of that stack.

    2. Load `standards/<stack>.md` and find the
       `## Verification Gate (build + test)` section.
    3. Execute the listed commands verbatim, in order, from the
       repo root (or the directory containing the manifest, for
       monorepo-nested projects).
    4. Record the per-stack outcome.

    **Per-stack outcomes:**

    - **Manifest absent** → record `<stack>: SKIPPED — no manifest`
      and continue. Not a failure.
    - **All commands exit zero** → record `<stack>: PASS` in
      working memory.
    - **Any command exits non-zero** → the dev session has produced
      broken code. Do NOT exit. Loop back: read the failure output,
      identify the offending change (most often the task most
      recently committed), and fix it. Commit the fix with the
      task-N-of-M format (a fix commit counts as a continuation of
      the task whose work it corrects). Re-run the gate (all
      stacks). Repeat until every stack reports PASS or SKIPPED.
      The dev session exits only on a clean gate across every
      detected stack.
    - **Toolchain unavailable** — manifest IS present but the
      toolchain isn't on PATH (e.g. `go.mod` exists but `which go`
      exits non-zero) → record `<stack>: SKIPPED —
      <toolchain> not installed on runner; install via
      <remediation>; CI is the backstop`. Continue to the next
      stack. This is NOT a fail and the dev session is permitted
      to exit (downstream compliance will run the same gate
      against the same runner and emit a matching warning;
      the PR-time CI is the authoritative backstop).

    **No standards file** for a detected stack (no
    `standards/<stack>.md` exists, or the file has no
    `## Verification Gate` section) → raise
    `VERIFICATION_GATE_UNDEFINED` (`ERROR`) and exit Output D.
    Adding the language to `standards/` is a framework-level fix,
    not work the dev session can route around.

    Record every per-stack outcome in working memory for inclusion
    in the exit block (step 18). The exit block lists each stack
    + its commands + its result (PASS / FAIL / SKIPPED with
    reason).

15a. **Acceptance-criteria coverage gate.** Before the closeout
    label flips, verify that every Feature acceptance criterion
    is satisfied by the work that just landed.

    Re-read the Feature issue body and extract the canonical AC
    list (`AC-1` ... `AC-N`). Then for each AC:

    - Locate the task(s) whose `Satisfies feature acceptance
      criteria:` line cited that AC index.
    - Confirm those tasks are CLOSED with their commits on the
      branch.
    - Confirm test coverage exists that exercises the AC's
      observable behaviour. The agent inspects the test suite
      (test files in the changed range, plus any newly-added
      tests) and confirms they assert the AC's stated outcome.

    Compute `<uncovered-ac>` = AC indices where any of the three
    conditions failed.

    - **`<uncovered-ac>` empty** → continue to step 16.
    - **`<uncovered-ac>` non-empty** → raise
      `AC_COVERAGE_INCOMPLETE` (`WARN`). Surface which AC are
      uncovered and why (no task cites them, the citing task is
      open, or no test asserts the outcome). Exit with Output D.
      The Feature is not done; the human inspects.

    For Features whose body has no `## Acceptance Criteria`
    section at all (older Features predating the new
    feature-design spec), log the omission and skip the gate —
    no enforcement is possible without the AC list.

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

16a. **Archive the recovery file.** If `recovery.md` exists at the
    repo root (it will, given step 9 wrote it during the run),
    archive it under `recovery-logs/recovery-log-<N>.md` and
    remove the live file from the branch:

    ```bash
    mkdir -p recovery-logs/
    git mv recovery.md recovery-logs/recovery-log-<N>.md
    git commit -m "chore: archive recovery log for #<N>

    Reuse: opt-out — checkpoint metadata, not derived from existing code"
    git push origin "$BRANCH"
    ```

    The archive lives on the feature branch and merges with the PR
    so the recovery trail is captured in the merged history. If
    `recovery.md` is missing (a fresh, fast run that produced no
    checkpoints), skip this step.

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

    Verification gate: PASS (stacks: <stack-1>[, <stack-2>...])
      - <stack-1>: <command-1> ✓ ; <command-2> ✓ ; ...
      - <stack-2>: <command-1> ✓ ; <command-2> ✓ ; ...

    Blocked: none

    Next: workflow applies in-verification, triggering compliance-verify
    ```

    The "Verification gate" line is mandatory in Output A and
    Output B. Omitting it is a protocol violation per
    `standards/<stack>.md` → Verification Gate. The exact commands
    shown match what was actually run in step 15pre, per stack.
    For multi-stack Features (e.g. a feature touching both Go and
    TypeScript), one line per stack appears. A Features that
    touches only docs / workflow files may show "Verification
    gate: NOT APPLICABLE (no stacks touched)" — but step 15pre's
    AC coverage gate still ran.

    **Output B — Resumed and completed:**
    ```
    === Dev Session — Completed (resumed) ===

    Produced (this run):
      - <K> task commit(s) on feature/<N>-<slug>
      - <K> task issue(s) closed: #<T_x>, ...

    Already complete on entry:
      - <M-K> task(s) closed by a prior run

    All <M> tasks for #<N> are closed.

    Next: workflow applies in-verification, triggering compliance-verify
    ```

    **Output C — Re-run no-op:**
    ```
    === Dev Session — No-op ===

    Entered with zero open tasks. All <M> tasks for #<N> were
    already closed.

    Next: workflow applies in-verification, triggering compliance-verify.
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
    push (no-op since the agent pushed), apply `in-verification`,
    set project status — triggering the compliance-verify stage.

## Error Handling

**Slot-release rule.** Per `skills/definitions/concurrency-beacon.md`
— every error path AND every graceful exit AFTER step 5 (beacon
claimed) MUST best-effort remove `development-in-progress`.

- `INVALID_DEV_STATE` from steps 2–5 (Feature missing, wrong
  labels, no rationale, no tasks, slot-claim failed) → severity
  `ERROR`; propagate. Caller / workflow bug — invoked dev-session
  on a Feature not ready for implementation.
- `BRANCH_MISSING` from step 6 (no remote branch) or step 13 (push
  failed) → severity `ERROR`; propagate. The Feature should never
  have reached `in-development` without a branch; if push fails
  mid-session, the local commit is preserved and a re-run will
  push it.
- `AC_COVERAGE_INCOMPLETE` from step 15a (an acceptance criterion
  is uncovered after the per-task walk completed) → severity
  `WARN`; propagate. The Feature is not done; the human inspects
  the uncovered AC and either retroactively adds tests or
  re-scopes via `requirement-scoping` extend mode.
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
