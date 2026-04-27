---
name: feature-design
description: Designs a Feature by producing a rationale artefact (the Design Plan), creating ordered Task sub-issues, and creating the feature branch. Runs headless when invoked against a Feature carrying in-design, or interactively in foreground when invoked against interactive-design. Headless flow auto-triggers implementation at end-of-flow; interactive flow asks the human to choose trigger-now / park-at-designed / cancel. Use when workflow automation fires on in-design label apply, or when a human is running design interactively on a Feature flagged interactive-design. Use even when the caller doesn't say "feature design" — phrases like "design feature #42", "run the design phase", "do the design for this Feature" should trigger this skill.
triggers: hybrid
user-invocable: true
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/step-skip-rule.md
  - skills/prompt-user/SKILL.md
  - skills/gh-agentic/SKILL.md
  - skills/apply-label/SKILL.md
  - skills/set-issue-status/SKILL.md
  - skills/post-issue-comment/SKILL.md
  - skills/trigger-implementation/SKILL.md
emits-exit-block: true
exit-hands-to: "automation: dev-session (in-development on triggered Features) | human: re-invoke trigger-implementation on parked (designed) Features when ready"
---

# Feature Design

## Goal

Produce the design artefacts for a single Feature so its
implementation can run unattended:

- A **rationale** (the Design Plan) — agent-authored prose explaining
  the architectural assessment, technical approach, key decisions and
  trade-offs, task breakdown logic, risks, and dependencies. Posted
  as a comment on the Feature issue. The dev-session reads it as
  primary input.
- **Ordered Task sub-issues** under the parent Feature — each with
  full content (title, description, acceptance hint, ordering header)
  and the `task` label so the dev-session can find and walk them.
- **A feature branch** in the active repo (`feature/<N>-<slug>`)
  scaffolded off `main`, ready for the dev-session's commits.
- **(Interactive mode only)** Any design products produced during the
  conversation — Figma URLs noted in the rationale, architectural
  diagrams committed to `docs/design/feature-<N>/` on the feature
  branch, design notes committed to the same path.

The skill runs in one of two modes, selected automatically from the
Feature's label set on entry:

- **Headless** — Feature carries `in-design`. Workflow automation
  fired this skill on label apply. No human in the loop. Skill walks
  start-to-finish, posts the rationale, creates the branch and tasks,
  invokes `trigger-implementation` to hand off to dev-session, exits.
- **Interactive** — Feature carries `interactive-design`. Human
  invoked this skill in foreground. Skill runs conversationally —
  exploration, rationale draft, confirm-or-revise, branch, tasks,
  optional design products — and ends with a 3-way prompt: trigger
  implementation now, park at `designed`, or cancel.

## Output Artefacts

- **One rationale comment** on the Feature issue, marked at the head
  with `<!-- design-plan:v1 -->` so dev-session and re-run-detection
  can identify it without parsing.
- **N Task issues** in the active repo (N ≥ 1), each:
  - Labelled `task`
  - Linked as a sub-issue of the parent Feature
  - Body begins with a stable `## Task N of M` header
  - Ordering preserved by issue creation order
- **A feature branch** in the active repo, named
  `feature/<feature-N>-<slug>`, branched off `main`.
- **(Interactive only)** Files under `docs/design/feature-<N>/` on
  the feature branch — committed, not pushed (the dev-session pushes
  along with implementation work).

A return value to the caller (in the headless case, the workflow):
```
{ repo: <string>, feature: <int>, mode: "headless" | "interactive",
  branch: <string>, task_count: <int>,
  exit_state: "in-development" | "designed" | "cancelled" }
```

`exit_state` reflects what the Feature's label is at exit:
- `in-development` — happy path either mode; dev-session takes over.
- `designed` — interactive mode, human picked "Stop here".
- `cancelled` — interactive mode, human cancelled before T1.

The skill's six valid terminal outputs:

**A. Headless complete.** Rationale posted, branch created, tasks
created, `trigger-implementation` succeeded. Feature at
`in-development`. Workflow's dev-session listener picks up.

**B. Headless re-run no-op.** Detected on entry: rationale comment
already exists with `<!-- design-plan:v1 -->`, and/or feature branch
already exists, and/or tasks already exist. Soft-fail per the design
contract: surface what's already in place, exit cleanly, do NOT
re-create. Invoke `trigger-implementation` only if the Feature is
still at `in-design` (the previous run failed at the trigger step).

**C. Interactive — trigger now.** Rationale posted, branch created,
tasks created, optional design products committed. Human chose
"Trigger now" → `trigger-implementation` called → Feature at
`in-development`.

**D. Interactive — parked.** Rationale posted, branch created, tasks
created, optional design products committed. Human chose "Stop here"
→ Feature transitioned `interactive-design` → `designed`; status
`Designed`. Awaits a later `trigger-implementation` call.

**E. Interactive — cancelled pre-T1.** Human cancelled before the
rationale was posted. No GitHub mutations made. Feature unchanged
at `interactive-design`.

**F. Interactive — cancelled post-T1 (partial).** Human cancelled
after one or more artefacts were created. Skill cannot cleanly
revert. Surfaces the partial state and exits with the Feature still
at `interactive-design`. Human must clean up manually or complete
the work.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy for
  `INVALID_DESIGN_STATE`, `RATIONALE_POST_FAILED`,
  `BRANCH_CREATION_FAILED`, `TASK_CREATION_FAILED`,
  `TRIGGER_FAILED`, `USER_CANCELLED`, `REVISION_LOOP_LIMIT`.
- `skills/definitions/verification-procedure.md` — change-pinning
  rule (every artefact creation is verified by querying GitHub /
  the local git repo).
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement
  rule preventing silent skipping. Conditional-step carve-out applies
  to interactive-only steps in headless mode and vice versa.

## Dependencies

- `skills/prompt-user/SKILL.md` — used at every gate in interactive
  mode (rationale confirm, task list confirm, exit choice). NOT
  used in headless mode.
- `skills/gh-agentic/SKILL.md` — used in step 2 to query the
  Feature's full state (`gh agentic status feature <N> --raw`),
  and in step 4's re-run / concurrency detection.
- `skills/apply-label/SKILL.md` — used in step 4 to apply the
  `design-in-progress` beacon and in step 18 / cancel paths to
  remove it; also used to transition `interactive-design` →
  `designed` in the parked-exit branch of step 18.
- `skills/set-issue-status/SKILL.md` — used in step 18's parked
  branch to set project status to `Designed`.
- `skills/post-issue-comment/SKILL.md` — used in step 11 to post
  the rationale.
- `skills/trigger-implementation/SKILL.md` — used at end-of-flow
  to transition the Feature to `in-development` (headless always;
  interactive when human picks "Trigger now").

## Steps

The **step-skip rule** from `skills/definitions/step-skip-rule.md`
applies to every step below: no step may be skipped without the agent
first emitting, in its response stream, which step is being skipped
and the concrete reason why.

**Resolving the active repo.** Resolve once in step 1 via:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

and reuse the value as `<active-repo>`.

**Conditional-step carve-out — mode-gated steps.** Steps marked
"(headless only)" or "(interactive only)" run only in their mode.
The step-skip rule does not require justification when the step is
not applicable to the running mode. Steps without a mode tag run in
both modes.

**State model & cancel semantics.** This skill performs four
sequential GitHub-side mutations whose recoverability differs:

| Transition | Where | Effect | Skill-recoverable? |
|---|---|---|---|
| **T1** | step 11 | Rationale comment posted to Feature issue | Partial — comment can be edited but not unposted |
| **T2** | step 13 | Feature branch created in active repo | Yes — branch can be deleted |
| **T3** | step 16 | Task issues created with sub-issue links | **No — point of no return.** Created issues cannot be auto-removed |
| **T4** | step 18 | Feature label/status transitioned (via `trigger-implementation` or to `designed`) | Partial — depends on the primitive's outcome |

**Cancel rules by state (interactive mode only — headless has no cancel):**

- **Before T1** (during rationale draft) → Output E. No mutations.
  Exit cleanly; Feature unchanged.
- **At T1** (rationale posted, no branch yet) → Output F variant.
  Comment cannot be unposted; the rationale lives on the issue. Mark
  the comment as cancelled by editing it to prepend
  `**[CANCELLED — design did not complete]**` and exit.
- **At T2** (branch created, no tasks yet) → Output F variant.
  Delete the local branch (`git branch -D feature/<N>-<slug>`); the
  remote was never pushed by this skill. Mark the rationale comment
  as cancelled per T1. Exit.
- **At T3 or later** (tasks created) → Output F. Surface the partial
  state — rationale comment, branch, K of M tasks created, label
  unchanged. Recommend either manual cleanup (close orphan tasks,
  delete branch, edit comment) or manual completion. Do NOT
  auto-revert.

**Re-run safety (fail softly).** Step 4 detects whether design has
already been run for this Feature. The skill exits cleanly in any
of these conditions:

- Rationale comment with `<!-- design-plan:v1 -->` marker already
  exists on the Feature.
- Feature branch already exists in the active repo.
- One or more child issues with the `task` label already exist as
  sub-issues of the Feature.

This protects against workflow re-fires (label flap, retried jobs)
without requiring the workflow to be perfectly idempotent. The
skill does not attempt to "complete" a partial run — that is a
human-driven recovery via `gh agentic repair` plus manual finishing.

---

### Section A — Setup

1. **Announce the session.** Print the banner verbatim before any
   tool call:

   ```
   ==========================================================
   === Feature Design Session — Started                       ===
   ==========================================================
   ```

   Resolve the active repo per the rule above and hold as
   `<active-repo>`.

2. **Read the Feature.** Query the full state:

   ```bash
   gh agentic status feature <N> --raw
   ```

   Capture the title, body, labels, and any sub-issue / branch
   metadata the command returns. Hold as `<feature>`.

   On non-zero exit → raise `INVALID_DESIGN_STATE` (`ERROR`) — the
   Feature does not exist or `gh agentic` is unavailable.

3. **Detect mode and validate state.** Inspect `<feature>.labels`:

   - MUST contain `feature`. Else raise `INVALID_DESIGN_STATE` —
     not a Feature.
   - MUST contain exactly one of `in-design`, `interactive-design`.
     - `in-design` only → `<mode> = headless`.
     - `interactive-design` only → `<mode> = interactive`.
     - Both, or neither → raise `INVALID_DESIGN_STATE`. The
       Feature is not in a designable state. Recommend
       `gh agentic repair`.
   - MUST NOT contain any of `designed`, `in-development`,
     `in-review`, `done`. If any present → raise
     `INVALID_DESIGN_STATE` — design has already run (or the
     Feature is past the design phase). The re-run detector in
     step 4 will surface a cleaner Output B; this check is the
     defensive guard for outright corruption.

4. **Entry guards.** Two checks fire in order before any work:

   **4a. Concurrency check.** Look for the `design-in-progress`
   label. The label is a beacon: while set, another session is —
   or recently was — actively designing this Feature.

   ```bash
   gh issue view <N> --repo <active-repo> --json labels \
     --jq '[.labels[].name] | index("design-in-progress")'
   ```

   - **Headless** + label set → another session is running. Exit
     cleanly with Output B variant: "Another design session is in
     flight; this run is a no-op." Do NOT remove the label (it
     belongs to the other session).
   - **Interactive** + label set → render the warning:
     ```
     ⚠ design-in-progress is set on this Feature. Another session
        (workflow run or a separate human session) may be actively
        designing it. Continuing will likely cause conflicts.
     ```
     Then `prompt-user`:
     ```
     prompt-user(
       question: "Another design session may be in progress. Continue anyway?",
       header: "Concurrent design detected",
       options: [
         {label: "Continue anyway",
          description: "I know the other session is dead or stuck. Proceed."},
         {label: "Cancel",
          description: "Exit; the other session keeps its claim."}
       ]
     )
     ```
     - Continue → fall through to 4b. Step 4c will re-claim the slot.
     - Cancel → exit cleanly (Output E variant); do NOT remove the
       label.
   - Label not set → continue to 4b.

   **4b. Re-run safety check (fail softly).** Detect prior-run artefacts:

   - Rationale comment already posted:
     ```bash
     gh issue view <N> --repo <active-repo> --json comments \
       --jq '[.comments[] | select(.body | startswith("<!-- design-plan:v1 -->"))] | length'
     ```
   - Feature branch already exists:
     ```bash
     git ls-remote --heads "https://github.com/<active-repo>" \
       "feature/<N>-*" | wc -l
     ```
   - Child task issues already exist:
     ```bash
     gh issue list --repo <active-repo> --label task \
       --search "parent:<N>" --json number --jq 'length'
     ```

   If any returns non-zero, this is Output B (headless) or a
   warning followed by exit (interactive — the human invoked it
   intentionally and deserves a clear "design has already run" note
   rather than silent re-creation):

   - **Headless** → if Feature label is `in-design`, invoke
     `trigger-implementation(issue=<N>)` to advance the stuck
     Feature; otherwise exit with Output B. Emit the exit block
     and terminate.
   - **Interactive** → render the detected artefacts and exit:
     ```
     Design appears to have already run for this Feature:
       - Rationale comment posted: yes/no
       - Feature branch: <name or "none">
       - Task sub-issues: <count>

     This skill will not re-create them. To redesign, you'll need
     to clean up the prior artefacts manually first.
     ```
     Exit cleanly (Output B variant).

   **4c. Claim the slot.** Apply the `design-in-progress` label to
   mark this session as the active designer:

   ```
   apply-label(repo=<active-repo>, issue=<N>,
               add=["design-in-progress"], remove=[])
   ```

   On failure → raise `INVALID_DESIGN_STATE` (`ERROR`); exit before
   any further work. The label is the lock; without it we cannot
   guarantee single-writer semantics.

   From this point on, every exit path (success, parked, error,
   cancel) MUST remove the label as part of its cleanup. See step
   18 for the happy-path removal and the Error Handling section
   for the failure-path rule.

5. **Architecture context.** Read `docs/ARCHITECTURE.md` if it
   exists; hold its contents as Slice SA context for the rationale.
   If missing, surface a warning to the response stream:

   ```
   Note: docs/ARCHITECTURE.md is missing. Design will proceed
   without architectural baseline. Slice SA mapping in the rationale
   will be limited.
   ```

   Continue. The hard gate for ARCHITECTURE.md lives at
   requirements-session, not here.

---

### Section B — Discussion (interactive mode only)

6. **Open the conversation. (interactive only)** Ask the human:

   > "We're designing Feature #<N>: <title>. What do you want to
   > explore before I draft the rationale? Constraints, alternatives
   > you want considered, UX/UI questions, integration concerns?"

   Wait for the human's reply. Continue the conversation until the
   human signals readiness (free-form, no prompt-user). Surface
   architectural assessment from `docs/ARCHITECTURE.md` inline as
   relevant.

7. **Capture design products if produced. (interactive only)**
   During the conversation the human may share Figma URLs, sketch
   files, or ask the agent to write design notes. Capture these as
   you go; commit nothing yet — they'll be committed alongside the
   branch creation in step 14.

   Hold as `<design-products>`: a list of `{ path: <relative path
   under docs/design/feature-<N>/>, content: <bytes or URL note> }`.

8. **Exit exploration. (interactive only)** Ask:

   ```
   prompt-user(
     question: "Ready to draft the Design Plan?",
     header: "Exploration — ready to structure?",
     options: [
       {label: "Yes, draft it",
        description: "I'll generate the rationale and we'll review it."},
       {label: "More discussion first",
        description: "Continue exploring."},
       {label: "Cancel design",
        description: "End the session; no artefacts created."}
     ]
   )
   ```

   - Draft → continue to Section C.
   - More → loop to step 6.
   - Cancel → Output E. Exit.

---

### Section C — Rationale (the Design Plan)

9. **Generate the rationale.** Author the Design Plan from the
   Feature body, the architecture context (step 5), and (in
   interactive mode) the exploration conversation (steps 6–7).

   Required sections:

   ```markdown
   <!-- design-plan:v1 -->

   # Design Plan — Feature #<N>: <title>

   ## Architectural Assessment

   <Slice SA mapping: linear addition / extension / novel.
    What in docs/ARCHITECTURE.md does this touch or extend?
    What new patterns are introduced, if any?>

   ## Technical Approach

   <How the Feature will be built — components, data flow,
    integration points, libraries. Concrete enough for an
    implementing agent to act without re-deriving.>

   ## Key Decisions & Trade-offs

   <Each decision: what was chosen, what was rejected, why.
    No fabrication — if the conversation surfaced a trade-off,
    record it; if the headless agent inferred one, mark it
    "(inferred from Feature body)".>

   ## Task Breakdown Rationale

   <Why these tasks, why this order. The tasks themselves
    are filed separately as sub-issues; this section explains
    the decomposition. List N tasks by title here as a preview;
    the bodies live on the sub-issues.>

   ## Risks & Open Questions

   <Anything an implementing agent should be cautious about
    or that genuinely could not be resolved at design time.>

   ## Dependencies

   <External libs, services, prior Features that must land
    first. If none, say "none".>
   ```

   The leading HTML comment marker is **load-bearing** — it is how
   re-run detection (step 4) and the dev-session find the rationale
   among the issue's comments. Always include it as the first line.

10. **Confirm rationale. (interactive only)** Render the full
    rationale verbatim in a fenced markdown block, then:

    ```
    prompt-user(
      question: "Does this Design Plan look right?",
      header: "Rationale review",
      options: [
        {label: "Confirm — post it",
         description: "Persist as a comment on the Feature."},
        {label: "Revise",
         description: "Tell me what to change."},
        {label: "Cancel design",
         description: "End the session; no artefacts created."}
      ]
    )
    ```

    - Confirm → continue to step 11.
    - Revise → ask the human what to change (free-text), apply the
      change, re-render, re-prompt. Cap at 5 revisions; on the 5th
      raise `REVISION_LOOP_LIMIT` (`WARN`) and surface the current
      draft with Confirm-as-is or Cancel.
    - Cancel → Output E. Exit.

11. **Post the rationale (T1).** Call `post-issue-comment` with:

    ```
    post-issue-comment(repo=<active-repo>, issue=<N>,
                       body=<rationale-content>)
    ```

    On failure → raise `RATIONALE_POST_FAILED` (`ERROR`); propagate.
    Pre-T1; no cleanup needed.

    Verify the comment exists with the `<!-- design-plan:v1 -->`
    marker via the same query used in step 4. If not, raise
    `RATIONALE_POST_FAILED`.

---

### Section D — Branch and design products

12. **Compute the branch name.** Slug = lowercase Feature title with
    non-alphanumeric runs collapsed to `-`, leading/trailing `-`
    stripped, capped at 30 chars. Branch name:
    `feature/<N>-<slug>`. Example: Feature #42 *"Backlog visibility
    for product managers"* → `feature/42-backlog-visibility-for-pro`.

13. **Create the feature branch (T2).** From the local repo:

    ```bash
    git fetch origin main
    git checkout -b "feature/<N>-<slug>" origin/main
    ```

    On failure (branch already exists locally, network failure,
    detached HEAD, etc.) → raise `BRANCH_CREATION_FAILED` (`ERROR`).
    The post-T1 cancel rule applies: rationale is posted; mark it
    cancelled before exit.

14. **Commit design products. (interactive only)** For each entry
    in `<design-products>` (step 7):

    - Write the content to `docs/design/feature-<N>/<path>` using
      the `Write` tool.
    - Stage with `git add`.

    Then commit:
    ```bash
    git commit -m "design: notes and references for feature #<N>"
    ```

    On failure → raise `BRANCH_CREATION_FAILED` (`ERROR`); the
    branch exists but is not in a clean state. Surface the partial
    state and exit per T2 cancel rules.

    Do NOT push. The dev-session pushes alongside its own commits.

---

### Section E — Tasks

15. **Generate the task list.** From the rationale's "Task Breakdown
    Rationale" section, expand each task to a full body. Every task
    must declare which Feature acceptance criterion (or criteria)
    it satisfies — the link from a task back to the Feature's AC
    is the basis for the coverage gate in step 15a and the
    AC-verification gate at the end of dev-session.

    **Read the Feature's Acceptance Criteria first.** Re-read the
    Feature issue body, locate the `## Acceptance Criteria` section,
    and list each criterion as `AC-1`, `AC-2`, ... `AC-N`. Each
    task body MUST cite at least one AC by its index.

    Task body shape:

    ```markdown
    ## Task <K> of <M>

    **Feature:** #<N> — <title>

    ## Description

    <2–4 sentences on what this task delivers and why it's
     this size / at this point in the order.>

    ## Acceptance Criteria

    - [ ] <Testable, outcome-shaped condition 1 for THIS task>
    - [ ] <Condition 2>
    - [ ] Tests pass

    **Satisfies feature acceptance criteria:** AC-<K1>, AC-<K2>
    <Indices into the Feature's AC list. At least one entry is
     mandatory; multiple are allowed when a task spans criteria.>

    ## Notes

    <Optional: dependencies on earlier tasks in the same
     Feature; libraries to use; pitfalls; "look at file X
     before starting".>
    ```

    `K` and `M` are stable — task #1 of M, task #2 of M, in the
    intended execution order. The checkbox-formatted Acceptance
    Criteria are the dev-session's Definition of Done for the
    task; the `Satisfies feature acceptance criteria:` line is the
    traceability backstop ensuring no Feature AC is left uncovered.

15a. **Acceptance-coverage gate.** Before continuing to step 16,
    verify that every Feature acceptance criterion (`AC-1` through
    `AC-N`) is cited by at least one task's `Satisfies feature
    acceptance criteria:` line.

    Walk the task list; build the union set of AC indices cited.
    Compute `<uncovered>` = Feature AC indices not in the union.

    - **`<uncovered>` empty** → continue to step 16.
    - **`<uncovered>` non-empty** → the task list does not cover
      every Feature AC. Render the uncovered criteria to the human
      (interactive) or log them and add an extra task that covers
      them (headless), then re-run the coverage check. Do NOT
      proceed to issue creation with uncovered AC — the dev-session
      will not be able to mark the Feature complete.

    The gate is non-negotiable: a Feature whose AC are not 1:1
    traceable to tasks is malformed.

16. **Confirm task list. (interactive only)** Render each task as
    a fenced markdown block prefaced by `Task K of M (title):`,
    then:

    ```
    prompt-user(
      question: "Create these task issues?",
      header: "Task list review",
      options: [
        {label: "Confirm — create them",
         description: "<M> Task issues will be created on <active-repo>. Point of no return."},
        {label: "Revise",
         description: "Tell me what to change about the task list."},
        {label: "Cancel design",
         description: "End the session. The rationale comment and branch will be marked cancelled; cleanup may be required."}
      ]
    )
    ```

    - Confirm → step 17. Surface the post-T2 warning to the human:
      ```
      ⚠ Once you confirm, Task issues are created. The skill cannot
         cleanly revert past this point.
      ```
    - Revise → ask free-text; cap at 5 revisions; loop.
    - Cancel → T2 cancel rule. Exit.

17. **Create each Task issue (T3).** For K = 1..M, in order:

    ```bash
    gh issue create \
      --repo "<active-repo>" \
      --title "Task <K>: <task-title>" \
      --label "task,backlog" \
      --body-file <path-to-task-body-file>
    ```

    The `backlog` label pairs with `task` so each task carries the
    same lifecycle-state convention as Requirements and Features
    on creation. The dev-session does NOT close tasks until each
    has a corresponding commit landed.

    Capture the issue number `<T_K>`. Wire as a sub-issue of the
    parent Feature using the same GraphQL pattern as
    `requirement-scoping` step 19 (resolve parent + child node IDs,
    `addSubIssue` mutation).

    On failure → raise `TASK_CREATION_FAILED` (`ERROR`). Surface
    K-1 successfully-created tasks and recommend manual cleanup.

    Verify each task with:
    ```bash
    gh issue view <T_K> --repo <active-repo> --json labels,title --jq .
    ```

    **CRITICAL: Do NOT close task issues after creating them.**
    Tasks remain open until the dev-session commits and closes
    each one in turn. Premature closure breaks the dev-session's
    open-vs-closed cursor (the resume mechanism) and leaves the
    Feature stuck in a state where dev cannot reliably tell which
    tasks were genuinely completed vs accidentally closed at design
    time. This rule is non-negotiable.

---

### Section F — Exit

18. **Release the slot, then hand off to implementation.**

    Before any label transition below, remove the
    `design-in-progress` claim:

    ```
    apply-label(repo=<active-repo>, issue=<N>,
                add=[], remove=["design-in-progress"])
    ```

    Failures here are surfaced as a `WARN` and do not block exit —
    the design work itself is complete. The stale label can be
    cleared by the human if it sticks.

    **Headless mode** → invoke directly:

    ```
    trigger-implementation(issue=<N>)
    ```

    On success → Output A. On failure → raise `TRIGGER_FAILED`
    (`ERROR`); surface that the rationale, branch, and tasks are
    all in place, but the Feature is stuck at `in-design`.
    Recommend `gh agentic repair`. Even on failure, all design
    artefacts are valid — the dev-session can be triggered manually
    once the label transition is fixed.

    **Interactive mode** → ask the human:

    ```
    prompt-user(
      question: "Design complete. What next?",
      header: "Hand-off",
      options: [
        {label: "Trigger implementation",
         description: "Run trigger-implementation now; dev-session picks up headless."},
        {label: "Stop here",
         description: "Park at designed; re-invoke trigger-implementation later."},
        {label: "Cancel",
         description: "Mark the Feature cancelled (post-T3; manual cleanup required)."}
      ]
    )
    ```

    - **Trigger** → invoke `trigger-implementation(issue=<N>)`.
      On success → Output C. On failure → `TRIGGER_FAILED`.
    - **Stop here** → transition the Feature to `designed`:
      ```
      apply-label(repo=<active-repo>, issue=<N>,
                  add=["designed"], remove=["interactive-design"])
      set-issue-status(repo=<active-repo>, issue=<N>, status="Designed")
      ```
      Output D.
    - **Cancel** → Output F. Surface partial state; exit without
      label transition.

19. **Emit the exit block.** Match the actual output:

    **Output A — Headless complete:**
    ```
    === Feature Design Session — Completed (headless) ===

    Produced:
      - Rationale: comment on #<N> (design-plan:v1)
      - Branch: feature/<N>-<slug>
      - Tasks: #<T1>, #<T2>, ... (<M> total)
      - Feature transitioned: in-design → in-development

    Blocked: none

    Next: automation: dev-session (in-development on #<N>)
    ```

    **Output B — Headless re-run no-op:**
    ```
    === Feature Design Session — No-op (already designed) ===

    Detected prior-run artefacts:
      - Rationale: <yes/no>
      - Branch: <name or "none">
      - Tasks: <count>

    No new artefacts produced. <Optional: trigger-implementation
    invoked because Feature was stuck at in-design.>

    Next: automation: dev-session (if trigger fired); else: human
          investigation
    ```

    **Output C — Interactive — trigger now:**
    ```
    === Feature Design Session — Completed (interactive, triggered) ===

    Produced:
      - Rationale: comment on #<N> (design-plan:v1)
      - Branch: feature/<N>-<slug>
      - Design products: <count> file(s) committed
      - Tasks: #<T1>, #<T2>, ... (<M> total)
      - Feature transitioned: interactive-design → in-development

    Blocked: none

    Next: automation: dev-session (in-development on #<N>)
    ```

    **Output D — Interactive — parked:**
    ```
    === Feature Design Session — Parked (designed) ===

    Produced:
      - Rationale: comment on #<N> (design-plan:v1)
      - Branch: feature/<N>-<slug>
      - Design products: <count> file(s) committed
      - Tasks: #<T1>, #<T2>, ... (<M> total)
      - Feature transitioned: interactive-design → designed

    Blocked: none

    Next: human: re-invoke trigger-implementation #<N> when ready
          to start dev work
    ```

    **Output E — Cancelled pre-T1:**
    ```
    === Feature Design Session — Cancelled ===

    Produced: nothing

    Blocked: nothing

    Next: Feature #<N> unchanged at <interactive-design or in-design>.
          Re-invoke feature-design when ready.
    ```

    **Output F — Cancelled post-T1 (partial):**
    ```
    === Feature Design Session — Cancelled (partial state) ===

    Produced before cancel:
      - Rationale: <posted | not posted>
      - Branch: <name | not created>
      - Tasks: <K of M created | none>

    The skill cannot cleanly revert. Recommended:
      <one of:>
      - Edit the rationale comment to remove the cancellation
        marker and re-invoke feature-design to complete the work
      - Close the K orphan task issues, delete the branch
        (locally and remote if pushed), and re-invoke fresh
      - Run gh agentic repair for guidance

    Feature #<N> remains at interactive-design.
    ```

20. **Terminate the session.** Per `emits-exit-block: true`, invoke
    the host runtime's session-close API if available; otherwise
    halt. No further work in this session.

## Verification

Per `skills/definitions/verification-procedure.md` "Section format".
Skill-specific commands:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/feature-design/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/feature-design/SKILL.md
```

Pass criteria: both commands exit 0.
## Error Handling

**Slot-release rule (universal).** Every error path AND every
cancel path that fires AFTER step 4c (the label was claimed) MUST
attempt to remove `design-in-progress` before exit, on a best-effort
basis. If the removal itself fails, surface it as a `WARN` and exit
anyway — the original error is what matters; a stuck beacon is a
secondary concern the human can clear by hand.

- `INVALID_DESIGN_STATE` from steps 2–3 (Feature missing, not a
  Feature, wrong/multiple/missing trigger labels) → severity
  `ERROR`; propagate. Caller bug — they invoked design on a Feature
  not in a designable state.
- `RATIONALE_POST_FAILED` from step 11 (`post-issue-comment` raised
  or verification mismatch) → severity `ERROR`; propagate. Pre-T1;
  no cleanup needed.
- `BRANCH_CREATION_FAILED` from step 13 or 14 (git failure or
  invalid state) → severity `ERROR`; propagate. T1 has fired; mark
  the rationale comment cancelled per T2 cancel rule before exiting.
- `TASK_CREATION_FAILED` from step 17 → severity `ERROR`; propagate.
  Surface K of M tasks created (which already exist as valid issues
  on GitHub); recommend manual cleanup. Do NOT continue.
- `TRIGGER_FAILED` from step 18 — `trigger-implementation` raised →
  severity `ERROR`; propagate. All design artefacts are in place
  and valid. The Feature is stuck at the trigger label; recommend
  `gh agentic repair` followed by manual `trigger-implementation`.
- `USER_CANCELLED` (any cancel point in interactive mode) →
  severity `WARN`. Apply the appropriate cancel rule by state and
  exit.
- `REVISION_LOOP_LIMIT` from step 10 or 16 (5 revisions elapsed) →
  severity `WARN`; surface current draft, recommend Confirm-as-is
  or Cancel.
- All other errors: propagate (default).
