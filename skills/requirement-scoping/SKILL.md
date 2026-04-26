---
name: requirement-scoping
description: Decomposes a Requirement into one or more well-formed Feature issues through a conversational, agent-led artefact walk — exploration, framing, MVP, decomposition, acceptance criteria, UX triage, deployment, parking lot — and triggers selected Features for headless design via the in-design label. Use when a human wants to scope a Requirement that has reached backlog into Features. Use even when the user doesn't explicitly say "feature scoping" — phrases like "let's scope this requirement", "break this requirement into features", "scope requirement #N", "decompose into features" should trigger this skill.
triggers: human-interactive
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/step-skip-rule.md
  - skills/prompt-user/SKILL.md
  - skills/gh-agentic/SKILL.md
  - skills/apply-label/SKILL.md
  - skills/set-issue-status/SKILL.md
  - skills/post-issue-comment/SKILL.md
emits-exit-block: true
exit-hands-to: "automation: feature-design (in-design label on triggered Features) | human: re-trigger held Features when their blockers clear"
---

# Requirement Scoping

## Goal

Decompose a Requirement (in `backlog` stage) into one or more
well-formed Feature issues through a structured artefact walk, then
trigger selected Features for headless Feature Design. Produces
ready-for-design Features (with full acceptance criteria, deployment
strategy, and UX triage decision) and transitions the parent
Requirement from `Scoping` to `Scheduled` on the project board.

## Output Artefacts

The skill has six valid terminal outputs — exactly one fires per
invocation:

**A. All Features triggered.** Every Feature created received the
`in-design` label and project status `In Design`. Requirement
transitioned to `scheduled`. Headless Feature Design will pick up
each triggered Feature.

**B. Some Features held.** Some Features triggered, others held at
`backlog` (cross-repo dependency, awaiting upstream PR, or
`needs-ux-design` blocking until UX Design completes). Requirement
transitioned to `scheduled`.

**C. All Features held.** Features created but none triggered (all
blocked by deps or UX). Requirement transitioned to `scheduled`.

**D. Cancelled mid-scope.** The human aborted during the artefact
walk. No Features created. Requirement reverted to `backlog`. No
project status change persists.

**E. Already scoped.** Detected on entry: the picked Requirement is
at `scheduled` and has child Features. Skill exits clean with a
pointer to the existing Features. No-op.

**F. Orphan re-entry.** Detected on entry: the picked Requirement
is at `scoping` (a prior session left it there). The skill asks
the human whether to continue the scoping fresh (clears any state
and starts over) or revert to `backlog`. No automated resume — a
new agent has no access to the prior conversation.

In all create-issue cases (A, B, C):
- Each Feature issue is created in the active repo with labels
  `feature, backlog`, and additionally `needs-ux-design` if
  artefact 7 returned "yes".
- Each Feature is wired as a sub-issue of the parent Requirement.
- A confirmation-gate transcript shows the human's explicit
  acceptance of each artefact before it was persisted.

No file artefacts. The GitHub issues are the durable record. The
parking lot (artefact 9) is captured in the parent Requirement's
body as a `## Parking Lot` section, not as separate artefacts.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy applied
  to `USER_CANCELLED`, `ISSUE_CREATION_FAILED`, `STATUS_TRANSITION_FAILED`,
  `REVISION_LOOP_LIMIT`, `ARCHITECTURE_MISSING`.
- `skills/definitions/verification-procedure.md` — change-pinning
  rule (Feature creation is verified by querying GitHub).
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement
  rule preventing silent skipping. Conditional-step carve-out
  applies to artefacts 7 (UX triage downstream) and revision loops.

## Dependencies

- `skills/prompt-user/SKILL.md` — used at every artefact gate for
  confirm/revise/cancel, and for trigger confirmation if Features ≤4.
- `skills/gh-agentic/SKILL.md` — used in step 1 to query the picked
  Requirement, and after Feature creation to verify the issues
  landed.
- `skills/apply-label/SKILL.md` — used for label transitions on the
  Requirement (`backlog`→removed, `scoping`→added; later `scoping`→removed,
  `scheduled`→added) and for Feature creation labels and the
  `in-design`/`backlog` swap during trigger.
- `skills/set-issue-status/SKILL.md` — used for project status
  transitions: Requirement to `Scoping` (entry), `Scheduled` (exit);
  Features to `In Design` (when triggered).
- `skills/post-issue-comment/SKILL.md` — used to surface decomposition
  context or the parking lot to the parent Requirement when relevant.

## Steps

The **step-skip rule** from `skills/definitions/step-skip-rule.md`
applies to every step below: no step may be skipped without the agent
first emitting, in its response stream, which step is being skipped
and the concrete reason why.

**Resolving the active repo.** Anywhere a step refers to the
"active repo", resolve it once in step 1 via:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

and reuse the value. Never hard-code the repo.

**Required labels precondition.** Step E (issue creation) applies
the labels `feature`, `backlog`, and conditionally `needs-ux-design`
to created Features, plus swaps `backlog`→`in-design` during trigger
confirmation. All these labels must exist on the active repo. Per
the framework's defense-in-depth approach, the agent does not
auto-create labels — that is repo-setup work owned by `gh agentic
check` / `repair`. If a label is missing at the apply moment,
propagate `ISSUE_CREATION_FAILED` and recommend `gh agentic repair`.

**Required project Status options precondition.** The Requirement's
project status moves through `Backlog` → `Scoping` → `Scheduled`,
and triggered Features move to `In Design`. All four option names
must exist on the project's Status field; if any is missing,
`set-issue-status` will raise `STATUS_OPTION_NOT_FOUND` and we
propagate as `STATUS_TRANSITION_FAILED`.

**Conditional-step carve-out.** Steps marked "only if …" are not
skipped when their precondition does not fire — they are simply
*not applicable*. The skip rule does not apply, and no
skip-justification is needed. In this skill: artefact 7's downstream
(applying `needs-ux-design`) is conditional on the artefact's
"yes" answer; the revision sub-loop in any artefact is conditional
on the human choosing "Revise"; the impact-delta re-confirmation in
later artefacts is conditional on a prior artefact being revised.

**Artefact-level disciplines.** Three rules apply across the
artefact walk (steps in section C):

- **Anti-fabrication clause (non-negotiable).** Every artefact
  recorded must be traceable to something the human actually said
  in the conversation. The agent MAY propose content (it is
  expected to — that is how scoping moves forward); the human
  MUST confirm before the proposal is persisted. The agent never
  decides on the human's behalf. Paraphrasing for grammar is fine;
  inventing a role, outcome, criterion, or scope decision the human
  did not state is not.
- **Confirm-or-revise gate per artefact.** Each of the 9 artefacts
  has an explicit `prompt-user` gate before the skill moves to the
  next artefact. Options: Confirm / Revise / Cancel. No artefact
  is silently accepted.
- **Per-revision diff (mandatory).** When an artefact is revised,
  before re-rendering it for confirmation the agent must emit — in
  its response stream — a per-field diff showing only the changed
  fields:
  ```
  Revision N — changes:
    <field>:
      was: <previous value>
      now: <new value>
    <unchanged fields are omitted>
  ```
**State model & cancel semantics.** This skill performs four
sequential GitHub-side transitions whose recoverability differs.
The agent must know which state it is in to handle cancellation and
failure correctly.

| Transition | Where | Effect | Skill-recoverable? |
|---|---|---|---|
| **T0 → T1** | step 4 | Requirement Backlog → Scoping (label + status) | Yes — revertible to Backlog |
| **T1 → T2** | step 18 | Create Feature issue(s) with labels + sub-issue link to parent | **No — point of no return.** Created issues cannot be auto-removed |
| **T2 → T3** | step 21 | Triggered Features Backlog → In Design (label + status) | Partial — failed triggers leave Features at Backlog |
| **T3 → T4** | step 23 | Requirement Scoping → Scheduled (label + status) | Partial — failed transition leaves Requirement at Scoping with Features in their final states |

**Cancel rules by state:**

- **Before T1** (during Requirement pick or exploration) → clean
  exit; no mutations to revert.
- **At T1** (Requirement at Scoping, no Features yet) → revert
  the Requirement to Backlog (label + status) and exit (Output D).
- **At T2 or later** (Features have been created) → the skill
  CANNOT cleanly undo. Surface the partial state to the human:
  which Features were created, their current labels and statuses,
  the Requirement's current state. Recommend manual cleanup
  (close orphan Features) or completing the work manually (apply
  `in-design` to selected Features and transition the Requirement
  to Scheduled). Do NOT auto-revert the Requirement to Backlog
  when Features exist — they would be left as orphans pointing
  to a backlog-stage parent.

The issue-creation confirmation gate (step 17) MUST warn the human
about this boundary before T2 fires:
> "Once you confirm, Feature issues will be created in GitHub.
> Cancellation after this point cannot remove the created issues
> automatically; the session would exit with the Features in place
> and the Requirement still at Scoping."

**Failure-during-transition rules:**

- **T0 → T1 fails partway** (e.g., label applied but status not
  set) → raise `STATUS_TRANSITION_FAILED`. Exit. The Requirement
  is in a label/status mismatch state; the next session's pre-entry
  check (step 3) detects and surfaces it. Recommend
  `gh agentic repair`.
- **T1 → T2 partial** (Feature 1 created, Feature 2 fails) → raise
  `ISSUE_CREATION_FAILED`. DO NOT continue with trigger
  confirmation — the Feature set is incomplete and partial trigger
  on a partial set leaves the framework in a worse state. Surface
  which Features succeeded and which didn't; recommend keeping the
  successful Features (already valid pipeline artefacts), closing
  any partial-creation orphans manually, and re-running scoping
  for the failed ones (which will create them as additional
  Features under the same parent).
- **T2 → T3 partial** (some Features triggered, others failed) →
  raise `STATUS_TRANSITION_FAILED`. DO NOT proceed to T3 → T4 —
  the Requirement transition is conditional on all triggers
  succeeding. Surface which Features are at `in-design` vs still
  at `backlog`; recommend manual re-trigger. The Requirement
  remains at Scoping.
- **T3 → T4 fails** → raise `STATUS_TRANSITION_FAILED`. Features
  are in their intended states; the Requirement is stuck at
  Scoping. Recommend `gh agentic repair` and manual re-run of
  the final transition only.

**Orphan re-entry refinement.** When step 3 detects the picked
Requirement at `scoping`, the same `gh agentic status requirement
<N> --raw` query also returns a `linked_features:` line. Branch
on it:

- **No child Features** → T1 leftover. Offer Continue scoping
  (start over) or Revert to backlog (the existing flow).
- **Has child Features** → T2-or-later leftover. The skill cannot
  cleanly resume from artefact 1 (would duplicate Features) and
  cannot cleanly revert (Features reference the parent). Surface
  the existing Features to the human and recommend they:
  - Inspect the partial Features
  - Either complete the work manually (apply `in-design` and
    transition the Requirement) or close the orphan Features
    *before* reverting the Requirement to Backlog.

  Exit cleanly. The skill does not auto-recover from this state.

- **Impact-delta on revision.** When a previously-confirmed artefact
  is revised, the agent must re-evaluate downstream artefacts that
  depend on it. Re-confirm only those flagged as affected; leave
  unaffected ones as-is.

  Scope-aware propagation:
  - **Revising a Requirement-level artefact (1, 2, or 3)** can
    affect every per-Feature artefact downstream — across all
    Features. Evaluate each Feature's per-Feature artefacts
    individually and only re-confirm those genuinely affected.
  - **Revising a per-Feature artefact (4, 5, 6, 7, or 8) for one
    Feature** affects only later per-Feature artefacts *for that
    same Feature*. Other Features' artefacts are not in scope.
  - **Revising artefact 9 (parking lot)** does not propagate
    downstream — it is the wrap-up.

  Surface the delta reasoning in the response stream:
  ```
  Impact delta from Revision N (artefact M):
    - Artefact P (Feature X) → affected (because <reason>) — re-confirm
    - Artefact Q (Feature Y) → not affected — keep as-is
  ```

---

### Section A — Setup

1. **Announce the session.** Print the banner verbatim before any
   tool call:

   ```
   ==========================================================
   === Requirement Scoping Session — Started                  ===
   ==========================================================
   You are now in Requirement Scoping mode. We will pick a
   Requirement, walk through 9 artefacts to define one or
   more Features, and trigger selected Features for design.
   ==========================================================
   ```

   Then resolve the active repo per the rule above and hold as
   `<active-repo>`.

   **Architecture context.** Read `docs/ARCHITECTURE.md` if it
   exists and hold its contents as Slice SA context for the
   conversation. If it does not exist, surface a warning to the
   human:

   ```
   Note: docs/ARCHITECTURE.md is missing. Scoping will proceed
   without architectural context, which means Slice SA mapping
   will be limited and architectural decisions made during this
   session will not have a baseline to anchor against.
   Recommended: pause this session, run solution-architecture to
   create the file, then resume.
   ```

   Do not hard-fail — the precondition is enforced at the
   requirements-session boundary. Proceeding without
   ARCHITECTURE.md is a degraded mode the human is choosing.

2. **Pick the parent Requirement.** Query the active repo's
   Requirements:

   ```bash
   gh agentic status requirements --raw
   ```

   The output is TSV: `number  stage  title  blocked_by  owning_repo`.
   Filter to entries with `stage = backlog` (the candidates for
   scoping). Then branch:

   - **No backlog Requirements** → render to the human ("There are
     no Requirements at backlog ready for scoping. Capture one
     first via requirements-session.") and exit cleanly.
   - **Exactly one backlog Requirement** → render it and ask
     `prompt-user`:
     ```
     prompt-user(
       question: "Scope this Requirement?",
       header: "Requirement #<N>: <title>",
       options: [
         {label: "Yes, scope it", description: "Continue."},
         {label: "Cancel", description: "Exit the session."}
       ]
     )
     ```
   - **Multiple backlog Requirements** → render the list as plain
     conversation (`#<N>  <title>`, one per line) and ask the human
     to reply with the number. Parse the reply:
     - A number matching a rendered `#<N>` → continue with that one.
     - "cancel" → raise `USER_CANCELLED` (`WARN`), exit.
     - Anything else → ask for clarification once. Cap at 3 invalid
       attempts before exiting with `USER_CANCELLED`.

3. **Pre-entry stage check.** Query the picked Requirement's full
   metadata:

   ```bash
   gh agentic status requirement <N> --raw
   ```

   Read the `stage:` line. Branch:

   - `backlog` → continue to step 4 (normal flow).
   - `scoping` → orphan re-entry (Output F). Apply the
     **Orphan re-entry refinement** from the Steps preamble: read
     the `linked_features:` line returned by the same
     `gh agentic status requirement <N> --raw` call to determine
     whether this is a T1 leftover (no children) or T2-or-later
     leftover (has children).

     **No child Features (T1 leftover):**
     ```
     prompt-user(
       question: "This Requirement is mid-scoping (stage: scoping). A prior session was interrupted before any Features were created. Continue from scratch, or revert to backlog?",
       header: "Orphan scoping detected — no Features yet",
       options: [
         {label: "Continue scoping (start over)", description: "Run the artefact walk fresh; previous conversation state is not recoverable."},
         {label: "Revert to backlog", description: "Set the Requirement back to backlog and exit. Re-invoke later."},
         {label: "Cancel", description: "Exit without changes."}
       ]
     )
     ```
     - Continue → proceed to step 5 (no transition needed; already
       at scoping). Skip step 4.
     - Revert → use `apply-label` to remove `scoping`, add `backlog`,
       and `set-issue-status` to set status to `Backlog`. Exit cleanly.
     - Cancel → `USER_CANCELLED`, exit (no changes).

     **Has child Features (T2-or-later leftover):** the skill cannot
     cleanly resume or revert. Render the existing Features to the
     human:
     ```
     This Requirement is mid-scoping (stage: scoping) and already has
     child Features:
       - #<F1> <title> [labels]
       - #<F2> <title> [labels]
       - ...
     The skill cannot cleanly resume (artefact walk would create
     duplicates) and cannot cleanly revert (Features reference this
     parent). Recommended actions, manual:
       1. Inspect each Feature; if the partial work is correct,
          apply `in-design` (and project status In Design) to those
          you want to trigger, then run set-issue-status to transition
          the Requirement to Scheduled.
       2. If the partial work is wrong, close the orphan Features
          (gh issue close), then re-run requirement-scoping which will
          revert the Requirement to Backlog and start fresh.
     Exiting now without changes.
     ```
     Exit cleanly with Output F (the variant explicitly says "no
     auto-recovery — manual action required").
   - `scheduled` → already-scoped (Output E). The
     `gh agentic status requirement <N> --raw` output already
     includes a `linked_features:` line listing the child Features —
     read it from the same query, surface the list to the human,
     and exit cleanly with a no-op message.
   - Any other stage → raise `UNEXPECTED_STAGE` (`ERROR`); the
     framework is in an unexpected state. Recommend `gh agentic check`.

4. **Transition Requirement: Backlog → Scoping.** Apply the label
   change and project status atomically:

   ```
   apply-label(repo=<active-repo>, issue=<N>,
               add=["scoping"], remove=["backlog"])
   set-issue-status(repo=<active-repo>, issue=<N>, status="Scoping")
   ```

   On any failure, propagate `STATUS_TRANSITION_FAILED` (`ERROR`).

---

### Section B — Exploration

5. **Open the conversation.** Ask the human:

   > "Tell me about this Requirement. What's the underlying need,
   > who has it, and how does it relate to the existing system?"

   Wait for the human's reply.

   **Mode-shift contract.** This is the exploration phase: open
   conversation, no artefacts yet. The agent's job is to listen,
   ask clarifying questions, surface architectural context from
   `docs/ARCHITECTURE.md`, and help the human gather their thoughts.
   No prompt-user gates fire in this section.

6. **Slice SA mapping.** Within the conversation, assess where this
   Requirement sits relative to the existing architecture:

   - **Linear addition** — extends an existing pattern (e.g., "another
     report like the existing ones"). Architectural impact: minimal.
     Exploration is short.
   - **Extension** — reuses architecture but adds surface (new
     endpoint, new screen, but uses existing data model and
     patterns). Architectural impact: moderate.
   - **Novel** — introduces patterns not in the foundation (new
     subsystem, new external integration, new NFR). Architectural
     impact: significant. Exploration is deep; may require updates
     to `docs/ARCHITECTURE.md`.

   Surface the assessment to the human in the conversation
   (informational, not a prompt-user). If novel and
   `docs/ARCHITECTURE.md` needs updating, propose the update inline
   — but do NOT write to the file in this session. Capture the
   proposed update as part of the parking lot (artefact 9) for
   the human to address via `solution-architecture` after scoping.

7. **Exit exploration.** Ask the human:

   ```
   prompt-user(
     question: "Ready to start the artefact walk?",
     header: "Exploration — ready to structure?",
     options: [
       {label: "Yes, start the walk",
        description: "Move to artefact 1 (raw idea summary)."},
       {label: "More discussion first",
        description: "Continue exploring; ask another question."},
       {label: "Cancel scoping",
        description: "End the session; no Features captured."}
     ]
   )
   ```

   - Start → continue to Section C.
   - More discussion → loop back to step 5; ask another clarifying
     question; re-prompt.
   - Cancel → revert Requirement to `backlog` (apply-label +
     set-issue-status), raise `USER_CANCELLED` (`WARN`), exit (Output D).

---

### Section C — Artefact walk (9 artefacts)

Artefacts 1, 2, 3, and 9 are **Requirement-level** — they run once
per scoping session. Artefacts 4 through 8 are **per-Feature** —
they run once per Feature (so N times when artefact 3 decides on
N parallel Features).

For the common N=1 case the walk is exactly 9 artefacts; for N>1
it grows by 5 per additional Feature (N=2 → 14 artefacts; N=3 → 19).

The pivot is artefact 3 (decomposition checkpoint), which decides
whether the Requirement spawns 1 or N Features. Everything before
the pivot is Requirement-level; everything between the pivot and
artefact 9 is per-Feature.

**Each artefact follows the same gate pattern.** The agent proposes
content, surfaces it to the human, and invokes `prompt-user`:

```
prompt-user(
  question: "Does this <artefact name> look right?",
  header: "Artefact <N>: <name>",
  options: [
    {label: "Confirm",      description: "Continue to the next artefact."},
    {label: "Revise",       description: "Tell me what to change."},
    {label: "Cancel",       description: "End the session; revert Requirement to backlog."}
  ]
)
```

- **Confirm** → persist the artefact, continue to the next.
- **Revise** → ask the human what to change (free-text), apply the
  per-revision diff discipline, re-render, re-prompt. Cap at 5
  revisions per artefact; on the 5th raise `REVISION_LOOP_LIMIT`
  (`WARN`) and surface the current draft with a recommendation to
  Confirm-as-is or Cancel.
- **Cancel** → revert Requirement to `backlog`, raise
  `USER_CANCELLED` (`WARN`), exit (Output D).

8. **Artefact 1 — Raw idea summary (Requirement-level).** A 1–3 sentence plain-language
   summary of what this Requirement is about. The agent drafts from
   the Requirement issue body and the exploration conversation.

9. **Artefact 2 — Problem statement (Requirement-level).** 2–4 sentences explaining the
   underlying problem (not a solution). Anchored in a specific user
   role and the consequence of the current state. Drafted from the
   parent Requirement's `## Business Need` section plus the
   exploration conversation.

10. **Artefact 3 — Decomposition checkpoint (Requirement-level).**
    Decide whether this Requirement spawns one Feature with ordered
    tasks, or multiple parallel Features. The decision is informed
    by the framing from artefacts 1–2; per-Feature detail (MVP, AC,
    deployment) has not yet been captured — this pivot is *before*
    the per-Feature work, not after it.

    Use a multi-choice prompt instead of confirm/revise:

    ```
    prompt-user(
      question: "How should this Requirement be decomposed?",
      header: "Artefact 3: Decomposition checkpoint",
      options: [
        {label: "Single Feature, ordered tasks",
         description: "One branch, one PR, sequential work."},
        {label: "Multiple parallel Features",
         description: "Independent work; each gets its own branch + PR."},
        {label: "Cancel scoping",
         description: "End the session."}
      ]
    )
    ```

    **Three-dimensional cost principle (apply before recommending
    parallel).** Before suggesting multiple Features, weigh:
    - **Token cost**: each parallel Feature requires its own full
      Design + Dev session.
    - **Build cost**: more branches = more CI runs = more build minutes.
    - **Time overhead**: parallel Features require coordination
      and merge ordering.

    If the work fits comfortably in a single Dev session, recommend
    one Feature with ordered tasks and explain the cost of
    splitting. Only recommend parallel when the work is substantial
    enough that parallelism delivers real value. Surface the
    recommendation and reasoning in the response stream BEFORE the
    prompt-user call so the human sees the rationale.

    Branch on the answer:
    - **Single Feature** → continue artefacts 4–8 once (then
      artefact 9 to wrap up).
    - **Multiple parallel Features** → ask the human to name the
      Features (free-text reply, one per line). Then run artefacts
      4–8 once per Feature, prefixing each prompt's header with the
      Feature name (e.g., `Artefact 4 — <Feature name>`). After all
      Features have walked through 4–8, run artefact 9 once.
    - **Cancel** → revert to backlog, raise `USER_CANCELLED`, exit.

11. **Artefact 4 — Feature definition + User Story (per-Feature).**
    A short paragraph describing what the Feature delivers, plus a
    User Story in the canonical format:
    ```
    As a <role>
    I want <capability>
    So that <outcome>
    ```
    All three slots must trace to specific things the human said in
    exploration or earlier artefacts (anti-fabrication). For
    multi-Feature scoping, each Feature has its own User Story —
    typically a refinement or slice of the parent Requirement's.

12. **Artefact 5 — MVP scope (per-Feature).** The smallest version
    of the Feature that delivers real value. Push toward minimum:
    every item in scope must be necessary for the value the Feature
    claims to deliver. Items that are nice-to-have go into the
    parking lot (artefact 9), not into MVP.

13. **Artefact 6 — Acceptance Criteria (per Feature).** At minimum
    three criteria, all in **Given/When/Then** format. Cover at
    least:
    - One success case (the happy path)
    - One failure case (an explicit error or rejection)
    - One edge case (boundary, concurrent, unusual input)

    Reject solution-criteria. If the human says "the system shall
    use Postgres", convert to outcome-criteria: "Given <context>,
    When <action>, Then <observable outcome>". Capability/technology
    choices belong in the Feature body, not in AC.

    Format:
    ```
    Given <preconditions>
    When  <action>
    Then  <observable outcome>
    ```

14. **Artefact 7 — UX triage (per Feature).** A binary yes/no
    question:

    ```
    prompt-user(
      question: "Does this Feature involve any UX or UI changes?",
      header: "Artefact 7: UX triage — <Feature name>",
      options: [
        {label: "No", description: "No UX or UI work needed."},
        {label: "Yes", description: "UX/UI changes are required; the Feature will be flagged for the UX Design phase."},
        {label: "Cancel scoping", description: "End the session."}
      ]
    )
    ```

    - **No** → no flag; the Feature can proceed straight to headless
      Feature Design when triggered.
    - **Yes** → record `UX impact: required` in the Feature body
      and flag the Feature with `needs-ux-design` at issue creation
      time. The dedicated UX Design session will produce the actual
      design later.
    - **Cancel** → revert to backlog, raise `USER_CANCELLED`, exit.

    **Important:** scoping does not produce UX content (sketches,
    Figma, descriptions). The yes/no answer is the only output of
    this artefact. UX content is the dedicated UX Design phase's
    deliverable.

15. **Artefact 8 — Deployment strategy (per Feature).** Multi-choice:

    ```
    prompt-user(
      question: "How should this Feature reach users once deployed?",
      header: "Artefact 8: Deployment strategy — <Feature name>",
      options: [
        {label: "No switch — deployed and immediately live",
         description: "Bug fixes, MVP phase, or infrastructure changes."},
        {label: "Feature switch — hidden until release decision",
         description: "Default for features and enhancements."},
        {label: "Functionality switch — permanent, gated by licence/tier",
         description: "Enters pipeline as its own Requirement; flag and exit."},
        {label: "Preview switch — user opt-in to new experience",
         description: "Enters pipeline as its own Requirement; flag and exit."}
      ]
    )
    ```

    Branch on the answer:

    - **No switch** for a Feature or enhancement → ask `prompt-user`
      for the reason (free-text). Record the reason in the Feature
      body. (Bug fixes default to no switch and don't need extra
      reasoning.)
    - **Feature switch** → ask `prompt-user` for switch mode:
      ```
      prompt-user(
        question: "Switch mode?",
        header: "Feature switch mode",
        options: [
          {label: "Permanent disable",
           description: "Use when work may be incomplete or breaking."},
          {label: "Toggle",
           description: "Code is safe; release is pending."}
        ]
      )
      ```
      Then ask via plain conversation for the proposed flag name
      (free-text; agent suggests a name based on Feature title;
      human confirms or revises). Record both in the Feature body.
      Note that switch removal will become a follow-up Requirement
      after full rollout.
    - **Functionality switch** or **Preview switch** → these are
      product decisions that warrant their own Requirement. Surface
      to the human:
      ```
      Functionality/Preview switches enter the pipeline as Requirements
      in their own right. Capture this Feature without the switch
      decision; address the licensing/preview model via a separate
      Requirement once this Feature is scoped.
      ```
      Continue scoping the Feature *without* the switch wired up —
      the switch concern is parked.

    See `concepts/feature-switches.md` for the full taxonomy.

16. **Artefact 9 — Parking lot review (Requirement-level).** Before issue creation,
    surface the running list of out-of-scope ideas the conversation
    produced (items pushed out of MVP, deferred questions,
    architectural updates that need `solution-architecture`). Render
    as plain conversation, then `prompt-user`:

    ```
    prompt-user(
      question: "Does this parking lot capture the side-issues correctly?",
      header: "Artefact 9: Parking lot",
      options: [
        {label: "Confirm", description: "Persist as a section in the parent Requirement's body."},
        {label: "Revise", description: "Add, remove, or edit items."},
        {label: "Skip parking lot", description: "Nothing to park; continue."},
        {label: "Cancel scoping", description: "End the session."}
      ]
    )
    ```

    On Confirm or Skip, continue. On Revise, loop. On Cancel, revert
    and exit.

    If the parking lot has items, append them to the parent
    Requirement as a `## Parking Lot` section using
    `post-issue-comment` (or by editing the issue body — caller's
    choice). This is the durable record.

---

### Section D — Issue creation

17. **Render the per-Feature summary to the human.** For each
    Feature about to be created, render the full body verbatim in
    a fenced markdown block prefaced by:
    ```
    Here is Feature <name> as it would be filed:
    ```

    The body shape is:
    ```markdown
    ## User Story

    **As a** <role>
    **I want** <capability>
    **So that** <outcome>

    ## Problem Statement

    <2–4 sentences, from artefact 2>

    ## MVP Scope

    <items in scope, from artefact 5>

    ## Acceptance Criteria

    - **Given** <…> **When** <…> **Then** <…>
    - **Given** <…> **When** <…> **Then** <…>
    - **Given** <…> **When** <…> **Then** <…>

    ## UX Impact

    <"None" — no UX changes required>
    OR
    <"Required — flagged with needs-ux-design. The UX Design phase will produce the design before this Feature can enter Feature Design.">

    ## Deployment Strategy

    <one of: No switch / Feature switch (mode + flag name) / Functionality switch / Preview switch>

    ## Notes

    <optional: switch reasoning, related context>

    ## Parent Requirement

    Closes #<parent-N> (when this Feature delivers the full Requirement)
    OR
    Part of #<parent-N> (when multiple Features share the parent)
    ```

    Then surface the **point-of-no-return warning** to the human as
    plain conversation BEFORE the prompt-user call (see "State model
    & cancel semantics" in the Steps preamble for the full rules):

    ```
    ⚠ Once you confirm, Feature issues will be created in GitHub.
       Cancellation after this point cannot remove the created
       issues automatically — the session would exit with the
       Features in place and the Requirement still at Scoping,
       requiring manual cleanup.
    ```

    Then invoke a single confirmation prompt covering all Features
    being created:

    ```
    prompt-user(
      question: "Create these Feature issues?",
      header: "Confirm Feature creation",
      options: [
        {label: "Yes, create them",
         description: "<N> Feature(s) will be created in <active-repo>. Point of no return."},
        {label: "Revise — go back to a specific artefact",
         description: "Pick which artefact to revisit."},
        {label: "Cancel scoping",
         description: "End the session; no Features created."}
      ]
    )
    ```

18. **Create each Feature issue.** Build the labels list from artefact 7's
    answer:

    - If UX triage = "no" → labels are `feature,backlog`
    - If UX triage = "yes" → labels are `feature,backlog,needs-ux-design`

    Write the Feature body from step 17 to a temporary file using
    the agent's `Write` tool — never via shell `echo` or heredoc, as
    user-supplied content may contain shell metacharacters (backticks,
    dollar signs, quotes) that would corrupt the file. Then invoke:

    ```bash
    gh issue create \
      --repo "<active-repo>" \
      --title "<Feature title>" \
      --label "<labels>" \
      --body-file <path-to-temp-file>
    ```

    Capture the resulting issue number `<F>` and URL from the
    command's stdout. The output is the full URL
    (`https://github.com/<active-repo>/issues/<F>`); parse `<F>`
    from the trailing path segment.

    **Feature title rule.** ≤70 characters. Noun-phrase summary of
    the outcome. NOT a verb-imperative ("Add CSV export"). NOT
    `feat:`-prefixed. Examples: *"Backlog visibility for product
    managers"*, *"Idempotent webhook processing"*. If no concise
    title fits, ask the human directly rather than truncating.

    **Verification gate.** After each create, query the issue back:

    ```bash
    gh agentic status feature <F> --raw
    ```

    Verify the issue exists with the expected labels. If missing
    or inconsistent, raise `ISSUE_CREATION_FAILED` (`ERROR`).

    **Partial-creation handling.** If Feature K of N fails (Features
    1..K-1 already exist on GitHub), STOP — do not continue creating
    Features K+1..N, and DO NOT proceed to step 19, 20, or beyond.
    Per "State model & cancel semantics", T1→T2 partial leaves the
    framework in a state the skill cannot auto-recover from. Surface:

    ```
    Partial Feature creation:
      - #<F1> created successfully (labels: ...)
      - ...
      - Feature K (<title>) failed: <gh stderr>
      - Features K+1..N not attempted

    The Requirement remains at Scoping. The successfully-created
    Features are valid pipeline artefacts; the failed one needs
    investigation. Recommended:
      - Run `gh agentic repair`
      - Re-invoke requirement-scoping; the orphan re-entry flow will
        detect the existing Features and surface them. From there,
        either close them and start over, or complete the work
        manually.
    ```

    Exit with `ISSUE_CREATION_FAILED`.

19. **Wire sub-issue relationships.** For each created Feature,
    establish the Feature → parent Requirement link via GitHub's
    sub-issue mechanism.

    GraphQL operates on node IDs, not issue numbers, so resolve
    each issue's node ID first (same pattern used in
    `set-issue-status` step 3):

    ```bash
    # Parent Requirement node ID (resolve once at the start of
    # this step, reuse across all child Features):
    PARENT_NODE_ID=$(gh api graphql -f query='
      query($owner:String!, $name:String!, $num:Int!) {
        repository(owner:$owner, name:$name) {
          issue(number:$num) { id }
        }
      }' \
      -F owner="${active_repo%/*}" \
      -F name="${active_repo#*/}" \
      -F num="<requirement-N>" \
      --jq '.data.repository.issue.id')

    # Per Feature, resolve the child node ID then link:
    CHILD_NODE_ID=$(gh api graphql -f query='
      query($owner:String!, $name:String!, $num:Int!) {
        repository(owner:$owner, name:$name) {
          issue(number:$num) { id }
        }
      }' \
      -F owner="${active_repo%/*}" \
      -F name="${active_repo#*/}" \
      -F num="<F>" \
      --jq '.data.repository.issue.id')

    gh api graphql -f query='
      mutation($parent:ID!, $child:ID!) {
        addSubIssue(input:{issueId:$parent, subIssueId:$child}) {
          issue { id }
        }
      }' \
      -F parent="$PARENT_NODE_ID" \
      -F child="$CHILD_NODE_ID"
    ```

    On any failure (node-ID lookup or the mutation itself), propagate
    `ISSUE_CREATION_FAILED`. The Feature body's `Closes #<parent>` /
    `Part of #<parent>` line is a fallback; the API-level sub-issue
    link is the durable signal.

---

### Section E — Trigger confirmation

20. **Render the list and ask which Features to trigger.** For
    each created Feature, render `#<F>  <title>  [needs-ux-design]?`
    (annotate Features that have the UX flag — they CANNOT be
    triggered for design until the UX Design phase clears them).

    Branch by Feature count:

    - **1–4 Features without UX flag** → use `prompt-user`:
      ```
      prompt-user(
        question: "Which Features should be triggered for design now?",
        header: "Trigger confirmation",
        options: [
          ...one option per non-flagged Feature, label "#<F> <title>"...,
          {label: "All", description: "Trigger every non-flagged Feature."},
          {label: "None", description: "Hold all at backlog."}
        ]
      )
      ```
    - **>4 Features OR any UX-flagged Feature** → use plain
      conversation. Render the list, ask the human to reply with
      numbers (`"1, 3"`, `"all"`, or `"none"`). Parse the reply:
      - Numbers matching rendered `#<F>` (and not flagged
        `needs-ux-design`) → trigger those.
      - "all" → trigger every non-flagged Feature.
      - "none" → trigger nothing.
      - A number that is flagged `needs-ux-design` → tell the human
        the Feature is blocked and ask them to revise their pick.
        Cap at 3 invalid attempts.

21. **Apply triggers atomically (per selected Feature).** For each
    selected Feature:

    ```
    apply-label(repo=<active-repo>, issue=<F>,
                add=["in-design"], remove=["backlog"])
    set-issue-status(repo=<active-repo>, issue=<F>, status="In Design")
    ```

    **Partial-trigger handling.** If trigger K of M fails, STOP —
    do not attempt the remaining triggers, and DO NOT proceed to
    step 22, 23, or beyond. Per "State model & cancel semantics",
    T2→T3 partial means the Requirement transition (T3→T4) is
    unsafe. Surface:

    ```
    Partial trigger:
      Triggered (now at In Design):
        - #<F1> in-design ✓
        - ...
      Failed (still at backlog):
        - Feature K: <stderr>
        - Features K+1..M not attempted
      Not selected (held at backlog by design):
        - ...

    The Requirement remains at Scoping. Recommended:
      - Run `gh agentic repair` to investigate the failure
      - Manually re-trigger the failed Features (apply in-design
        and run set-issue-status to In Design)
      - Once all intended Features are at In Design, manually
        transition the Requirement to Scheduled (apply scheduled
        label, remove scoping, set-issue-status to Scheduled)
    ```

    Raise `STATUS_TRANSITION_FAILED`.

22. **Annotate held Features.** For each non-triggered Feature,
    post a comment via `post-issue-comment`:

    ```
    Held at backlog — not triggered during scoping.
    Reason: <reason from the agent's understanding>
    Re-trigger: apply the in-design label and run set-issue-status
    to In Design when ready.
    ```

    The reason is one of: `needs-ux-design (UX Design phase pending)`,
    `cross-repo dependency (awaiting upstream PR)`, or `deliberate
    hold by human`.

---

### Section F — Closeout

23. **Transition Requirement: Scoping → Scheduled.**

    ```
    apply-label(repo=<active-repo>, issue=<requirement-N>,
                add=["scheduled"], remove=["scoping"])
    set-issue-status(repo=<active-repo>, issue=<requirement-N>, status="Scheduled")
    ```

    On any failure, propagate `STATUS_TRANSITION_FAILED`.

24. **Emit the exit block** matching the actual outcome. All
    variants conform to the same Produced / Blocked / Next shape:

    **Output A — All Features triggered:**
    ```
    === Requirement Scoping Session — Completed ===

    Produced:
      - Feature #<F1> created (triggered for design)
      - Feature #<F2> created (triggered for design)
      - Requirement #<N> transitioned: scoping → scheduled

    Blocked: none

    Next: automation: feature-design (in-design on #<F1>, #<F2>)
    ```

    **Output B — Some held:**
    ```
    === Requirement Scoping Session — Completed ===

    Produced:
      - Feature #<F1> created (triggered for design)
      - Feature #<F2> created (held at backlog — needs-ux-design)
      - Requirement #<N> transitioned: scoping → scheduled

    Blocked: #<F2> — UX Design phase pending

    Next: automation: feature-design (in-design on #<F1>);
          human: run UX Design for #<F2>, then re-trigger
    ```

    **Output C — All held:**
    ```
    === Requirement Scoping Session — Completed ===

    Produced:
      - Feature #<F1> created (held at backlog — needs-ux-design)
      - Feature #<F2> created (held at backlog — cross-repo dep)
      - Requirement #<N> transitioned: scoping → scheduled

    Blocked: #<F1> — UX Design pending; #<F2> — upstream PR

    Next: human: clear blockers and re-trigger each Feature
    ```

    **Output D — Cancelled mid-scope:**
    ```
    === Requirement Scoping Session — Cancelled ===

    Produced: nothing

    Blocked: nothing

    Next: Requirement #<N> reverted to backlog. Re-invoke
          requirement-scoping when ready to scope again.
    ```

    **Output E — Already scoped (early exit from step 3):**
    ```
    === Requirement Scoping Session — No-op ===

    Produced: nothing — Requirement #<N> already scheduled.

    Existing Features: #<F1>, #<F2>, ...

    Next: nothing for scoping. The existing Features are already
          in the pipeline.
    ```

    **Output F — Orphan re-entry:**
    ```
    === Requirement Scoping Session — Reverted ===

    Produced: Requirement #<N> reverted: scoping → backlog

    Blocked: none

    Next: re-invoke requirement-scoping when ready to start fresh.
    ```

25. **Terminate the session.** Per `emits-exit-block: true`, invoke
    the host runtime's session-close API if available; otherwise
    halt. No further work in this session.

## Verification

Run the framework checks against this skill:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/requirement-scoping/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/requirement-scoping/SKILL.md
```

Pass criteria: both commands exit 0.

**Behavioural-ceiling note.** Passing the framework checks verifies
structure (frontmatter, sections, trigger phrasings) and reference
integrity — not that the agent honours the anti-fabrication clause,
the per-revision diff, or the impact-delta rule at runtime. Those
disciplines are self-policed. "Passes verification" means the skill
is well-formed, not that scoping output will be high quality.

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
  the `GROUND_TRUTH` entry for `requirement-scoping`.

## Error Handling

- `USER_CANCELLED` (raised at any cancel point — Requirement pick,
  any artefact gate, exploration exit, trigger confirmation) →
  severity `WARN`. End cleanly. If the Requirement was transitioned
  to `Scoping`, revert it to `Backlog` (label + status) before exit.
- `ISSUE_CREATION_FAILED` from steps 18–19 (gh CLI failed, label
  missing, sub-issue link failed, verification mismatch) → severity
  `ERROR`; propagate. Surface the gh stderr and recommend
  `gh agentic repair`. Note: at this point one or more Features may
  have been created successfully; the agent must surface which
  succeeded and which didn't, leaving the human to decide whether
  to clean up partials manually or re-run scoping.
- `STATUS_TRANSITION_FAILED` (raised by `set-issue-status` or
  `apply-label`) → severity `ERROR`; propagate. The framework's
  state may be inconsistent (e.g., label says `scoping` but project
  status says `Backlog`); recommend `gh agentic repair`.
- `REVISION_LOOP_LIMIT` from any artefact (5 revisions elapsed) →
  severity `WARN`; surface the current draft, recommend Confirm-as-is
  or Cancel, end the artefact gate. The human picks; the skill does
  not auto-decide.
- `ARCHITECTURE_MISSING` from step 1 → severity `WARN` only (the
  hard gate is at requirements-session, not here). Continue in
  degraded mode; surface the warning to the human.
- `UNEXPECTED_STAGE` from step 3 → severity `ERROR`; propagate. The
  framework's state is unexpected; recommend `gh agentic check`.
- All other errors: propagate (default).
