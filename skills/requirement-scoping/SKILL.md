---
name: requirement-scoping
description: Decomposes a Requirement into one or more well-formed Feature issues through a conversational, agent-led artefact walk — exploration, framing, MVP, decomposition, acceptance criteria, interactive-design triage, deployment, parking lot — and triggers selected Features for design via the in-design or interactive-design label. Use when a human wants to scope a Requirement that has reached backlog into Features. Use even when the user doesn't explicitly say "feature scoping" — phrases like "let's scope this requirement", "break this requirement into features", "scope requirement #N", "decompose into features" should trigger this skill.
triggers: human-interactive
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/step-skip-rule.md
  - skills/definitions/state-model-pattern.md
  - skills/definitions/render-before-confirm.md
  - skills/prompt-user/SKILL.md
  - skills/gh-agentic/SKILL.md
  - skills/apply-label/SKILL.md
  - skills/set-issue-status/SKILL.md
  - skills/trigger-design/SKILL.md
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
Requirement from `Scoping` to `Ready to Implement` on the project board.

## Output Artefacts

The skill has six valid terminal outputs — exactly one fires per
invocation:

**A. All Features triggered.** Every Feature created received the
`in-design` label and project status `In Design`. Requirement
transitioned to `ready-to-implement`. Headless Feature Design will pick up
each triggered Feature.

**B. Some Features held.** Some Features triggered, others held at
`backlog` (target-repo dependency awaiting an upstream PR, or deliberate
hold by human). Requirement transitioned to `ready-to-implement`.

**C. All Features held.** Features created but none triggered (all
blocked by deps or held by human). Requirement transitioned to
`ready-to-implement`.

**D. Cancelled mid-scope.** The human aborted during the artefact
walk. No Features created. Requirement reverted to `backlog`. No
project status change persists.

**E. Already scoped.** Detected on entry: the picked Requirement is
at `ready-to-implement` and has child Features. The skill offers
the human three paths (extend / replace / no-op):

- **No-op exit** → exits clean with a pointer to the existing
  Features. Variant of E.
- **Extend** → transitions Requirement back to `scoping`, runs the
  artefact walk to add new sibling Features (existing untouched),
  transitions back to `ready-to-implement`. Outputs A/B/C apply
  for the newly-created Features.
- **Replace** → human picks existing Features to close; closes
  them; then proceeds as Extend for the replacements. Outputs
  A/B/C apply for the new Features; closed Features are reported
  in the exit block.

**F. Orphan re-entry.** Detected on entry: the picked Requirement
is at `scoping` (a prior session left it there). The skill asks
the human whether to continue the scoping fresh (clears any state
and starts over) or revert to `backlog`. No automated resume — a
new agent has no access to the prior conversation.

In all create-issue cases (A, B, C):
- Each Feature issue is created in the active repo with labels
  `feature, backlog`, and additionally `needs-interactive-design` if
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
- `skills/definitions/render-before-confirm.md` — the turn-boundary
  rule for every artefact gate: rendered content is the final output
  of its turn; the `prompt-user` Confirm/Revise/Cancel call comes on
  the next turn, so the human always sees what they are approving.

## Dependencies

- `skills/prompt-user/SKILL.md` — used at every artefact gate for
  confirm/revise/cancel, and for trigger confirmation if Features ≤4.
- `skills/gh-agentic/SKILL.md` — used in step 1 to query the picked
  Requirement, and after Feature creation to verify the issues
  landed.
- `skills/apply-label/SKILL.md` — used for label transitions on the
  Requirement (`backlog`→removed, `scoping`→added; later `scoping`→removed,
  `ready-to-implement`→added).
- `skills/set-issue-status/SKILL.md` — used for project status
  transitions on the Requirement: to `Scoping` (entry) and
  `Ready to Implement` (exit).
- `skills/trigger-design/SKILL.md` — used in step 21 to transition each
  selected Feature from `backlog` to `in-design` (or `interactive-design`,
  if the Feature carries `needs-interactive-design`) with project status
  `In Design`. Encapsulates the trigger-label decision so this skill
  doesn't repeat it.
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
the labels `feature`, `backlog`, and conditionally `needs-interactive-design`
to created Features, plus swaps `backlog`→`in-design` during trigger
confirmation. All these labels must exist on the active repo. Per
the framework's defense-in-depth approach, the agent does not
auto-create labels — that is repo-setup work owned by `gh agentic
check` / `repair`. If a label is missing at the apply moment,
propagate `ISSUE_CREATION_FAILED` and recommend `gh agentic repair`.

**Required project Status options precondition.** The Requirement's
project status moves through `Backlog` → `Scoping` → `Ready to Implement`,
and triggered Features move to `In Design`. All four option names
must exist on the project's Status field; if any is missing,
`set-issue-status` will raise `STATUS_OPTION_NOT_FOUND` and we
propagate as `STATUS_TRANSITION_FAILED`.

**Conditional-step carve-out.** Steps marked "only if …" are not
skipped when their precondition does not fire — they are simply
*not applicable*. The skip rule does not apply, and no
skip-justification is needed. In this skill: artefact 7's downstream
(applying `needs-interactive-design`) is conditional on the artefact's
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
  is silently accepted. Per
  `skills/definitions/render-before-confirm.md`, the artefact is
  rendered as the final output of its turn and the `prompt-user`
  gate is invoked on the *next* turn — never render the artefact and
  prompt for its approval in the same turn, or the human is asked to
  confirm content they cannot see.
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
**State model & cancel semantics.** This skill follows the pattern
in `skills/definitions/state-model-pattern.md`. The concrete tier
table below is this skill's specifics; the universal cancel rules
and failure-during-transition rules come from the definition.

Transition errors raised by this skill: `STATUS_TRANSITION_FAILED`
(label/status flips), `ISSUE_CREATION_FAILED` (Feature creation).

| Transition | Where | Effect | Skill-recoverable? |
|---|---|---|---|
| **T0 → T1** | step 4 | Requirement Backlog → Scoping (label + status) | Yes — revertible to Backlog |
| **T1 → T2** | step 18 | Create Feature issue(s) with labels + sub-issue link to parent | **No — point of no return.** Created issues cannot be auto-removed |
| **T2 → T3** | step 21 | Triggered Features Backlog → In Design (label + status) | Partial — failed triggers leave Features at Backlog |
| **T3 → T4** | step 23 | Requirement Scoping → Ready to Implement (label + status) | Partial — failed transition leaves Requirement at Scoping with Features in their final states |

The issue-creation confirmation gate (step 17) is where the
"point-of-no-return" warning required by the definition fires;
see step 17 for the exact phrasing.

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

   **Project context reads.** Two further repo-root files inform
   the scoping conversation when present; read each if it exists,
   skip silently if not (they are optional, not load-bearing):

   - `docs/PROJECT_BRIEF.md` — the project's high-level brief
     (one level above ARCHITECTURE.md). Use for vocabulary
     alignment when discussing capability domains.
   - `AGENTS.md` — repo-specific agent rules and project metadata
     loaded at session bootstrap. Already in scope via the
     bootstrap auto-load; this is a reminder not to re-read.

   Hold the contents available for reference; do NOT quote them
   verbatim into Feature bodies (they're context, not source).

   **Federation manifest detection.** Check whether the active repo
   is a federation requirements repo by testing for `FEDERATION.md`
   at the repo root:

   - **Present** → read it with the `Read` tool (NOT shell parsing)
     and hold the parsed repos-and-purposes list as `<federation>`.
     The manifest is YAML of the shape:
     ```yaml
     repos:
       - name: owner/repo-a
         purpose: <what work lives in this repo>
       - name: owner/repo-b
         purpose: <…>
     ```
     `<federation>.repos` is the candidate target set for feature
     placement (artefact 3). Do NOT re-validate the manifest here —
     `gh agentic check` (#824/#827) owns manifest integrity. If the
     file is present but unreadable or obviously malformed, surface
     a one-line warning and point the human at `gh agentic check`,
     then proceed treating `<federation>` as unset (single-repo
     behaviour) rather than guessing target repos.
   - **Absent** → leave `<federation>` unset. The repo is single
     topology: every Feature is created in `<active-repo>` and no
     target-repo question is ever asked. The entire artefact walk
     below is textually unchanged from the single-repo flow.

   Throughout the steps below, behaviour gated on "`<federation>` is
   held" applies ONLY when the manifest was present; when unset, the
   gated prose is not applicable (no skip-justification needed — it
   is a conditional-step carve-out).

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
          the Requirement to Ready to Implement.
       2. If the partial work is wrong, close the orphan Features
          (gh issue close), then re-run requirement-scoping which will
          revert the Requirement to Backlog and start fresh.
     Exiting now without changes.
     ```
     Exit cleanly with Output F (the variant explicitly says "no
     auto-recovery — manual action required").
   - `ready-to-implement` → already-scoped, but the human may want
     to extend or replace. The `gh agentic status requirement <N>
     --raw` output's `linked_features:` line lists the existing
     child Features. Surface them to the human and ask:

     ```
     prompt-user(
       question: "This Requirement is already scoped (stage: ready-to-implement) with the Features listed above. Why are you re-invoking?",
       header: "Already-scoped Requirement",
       options: [
         {label: "Add more Features (extend scope)",
          description: "Create new sibling Features under this Requirement. Existing Features are not modified."},
         {label: "Replace existing Feature(s)",
          description: "Close some existing Features and create replacements."},
         {label: "No-op — exit",
          description: "I'm here by mistake; do nothing."}
       ]
     )
     ```

     Branch on the answer:

     - **No-op** → exit cleanly with Output E (the no-op variant).

     - **Add more Features (extend)** → continue to step 4 with a
       transition variation: instead of `backlog → scoping`, the
       transition is `ready-to-implement → scoping` (apply
       `scoping`, remove `ready-to-implement`, set status to
       `Scoping`). Then proceed normally through the artefact walk.
       At artefact 3 (decomposition), the existing Features are
       surfaced as context: the human is choosing how many
       *additional* Features to spawn, not redoing the existing
       ones. Artefacts 4–8 run only for the new Features (the
       existing Features remain untouched in their current state).
       At step 21 (trigger), only the newly-created Features go
       through the trigger flow; existing Features keep their
       current trigger state. At step 23 (closeout), transition
       Requirement back to `ready-to-implement`.

     - **Replace existing Feature(s)** → ask the human which
       existing Features to close (free-text reply with the
       numbers, e.g., `"#F1, #F3"`). For each named Feature, run:
       ```
       gh issue close <F> --reason "Replaced during requirement-scoping re-run"
       ```
       Then proceed as in the extend case above: the artefact walk
       creates replacement Features under the same parent
       Requirement; at step 21 only the new Features go through
       trigger; at step 23 the Requirement returns to
       `ready-to-implement`.

     **Important:** the artefact walk in extend/replace mode
     should NOT re-do artefacts 1 and 2 (the Requirement-level
     framing) — those were settled when the Requirement was first
     scoped. Surface them to the human as known context but skip
     the confirm/revise gate. Start the walk from artefact 3
     (decomposition) and proceed normally from there.

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

**Per-Feature header threading (only when `<federation>` is held).**
Once artefact 3 has assigned a `<target-repo>` to each Feature, every
per-Feature artefact gate (4–8) for that Feature MUST show the target
repo in its header, so the human always sees which implementation repo
the Feature they are shaping will target — e.g. `Artefact 4 —
<Feature name> (targets <target-repo>)`. The same `(targets …)`
annotation appears in the step-17 creation render. The Feature itself
is always created in `<active-repo>` (the control plane); the target
is recorded in its "Target repo" field. When `<federation>` is unset,
headers carry no repo annotation (single-repo flow, unchanged).

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

    **One-repo rule and target-repo assignment (only when
    `<federation>` is held).** In a federation, the control plane is
    where Features live: **every Feature is created in the
    control-plane repo (`<active-repo>`)** and records the single
    implementation repo it targets via the "Target repo" ProjectV2
    field. The target repo is data on a control-plane issue, not the
    repo the issue is created in. Two sub-rules apply at this
    checkpoint, BEFORE any per-Feature artefact (4–8) runs:

    1. **One-repo rule (split).** A Feature whose work spans two
       repos must be split into one Feature per repo, each
       describing what it does in that repo and carrying its own
       target. When the human names a Feature (or the single-Feature
       framing) that clearly straddles two manifest repos, surface
       the split and propose the per-repo Features:
       ```
       Feature "<name>" spans <repo-A> and <repo-B>. A Feature must
       target one repo, so I'll split it into:
         - <name> (→ <repo-A>): <what it does there>
         - <name> (→ <repo-B>): <what it does there>
       ```
       Confirm the split with the human before proceeding. The split
       Features then each walk artefacts 4–8 independently.

    2. **Target-repo assignment (propose / confirm / override /
       validate).** For each Feature (after any split), propose a
       target repo drawn from `<federation>.repos`, citing the
       manifest purpose that justifies it, then let the human confirm
       or override:
       ```
       prompt-user(
         question: "Which repo should Feature \"<name>\" target? I propose <owner/repo> — <manifest purpose>.",
         header: "Target repo — <name>",
         options: [
           {label: "<owner/repo> (proposed)",
            description: "<manifest purpose for the proposed repo>"},
           ...one option per other manifest repo, label "<owner/repo>"...,
           {label: "Cancel scoping",
            description: "End the session."}
         ]
       )
       ```
       If the manifest has more than four repos, render the
       candidate list as plain conversation (one `owner/repo —
       purpose` per line) and ask the human to reply with the target,
       per the >4-option convention used elsewhere in this skill.
       The chosen target MUST be one of `<federation>.repos` — if the
       human supplies a repo not in the manifest, reject it and
       re-prompt ("only manifest repos are valid targets"). Capture
       the confirmed target's **bare repo name** (the part after `/`;
       the owner is always the control-plane owner) as `<target-repo>`
       for that Feature and hold it through the rest of the walk.
       `<target-repo>` is the value written to the "Target repo" field
       at creation (step 18b) — it is NOT the creation repo.

    When `<federation>` is unset (single topology), neither sub-rule
    applies: there is no target-repo question, no "Target repo" field
    is set, every Feature is created in `<active-repo>` as today, and
    the per-Feature headers below carry no repo annotation.

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

    **Behaviour-coverage rule.** For any Feature that delivers
    user-facing behaviour — CLI commands, API endpoints, UI
    interactions, or any change a user can directly invoke or
    observe — at least one AC MUST describe what the user sees or
    experiences when the feature works correctly. ACs that only
    describe internal implementation (e.g. "the code calls function
    X", "a test asserts Y arguments", "the build passes", "the
    config uses Z") are *insufficient on their own*. They may exist
    alongside behaviour ACs but cannot be the complete AC set.

    Examples of failing patterns (implementation-only ACs):
    - "`newAuthCmd()` wires `claudeRefresh` using `exec.Command("claude", "auth", "login")`"
    - "A unit test in `internal/cli/` verifies the closure invokes `claude` with args `["auth", "login"]`"
    - "`go build ./...` passes cleanly"

    The corresponding behaviour AC that was missing:
    - "Given the user has no active Claude session, When they run `gh agentic auth login`, Then an interactive login prompt appears in their terminal and they can complete the Claude authentication flow"

    If the human supplies only implementation ACs, surface this gap
    explicitly before closing artefact 6. Add the missing behaviour
    AC or confirm with the human that no user-facing behaviour is
    introduced (e.g. a pure internal refactor with no CLI change).

14. **Artefact 7 — Interactive-design triage (per-Feature).** A
    binary yes/no question. The decision is whether this Feature's
    *design phase* must run in foreground (interactive) — typically
    because it involves UX/UI work, novel architecture, or anything
    else that cannot be settled headlessly.

    ```
    prompt-user(
      question: "Is this Feature interactive-only — does its design phase need to be done in the foreground (interactive)?",
      header: "Artefact 7: Interactive-design triage — <Feature name>",
      options: [
        {label: "No",
         description: "Headless design is fine. Feature will be triggered with the in-design label."},
        {label: "Yes",
         description: "Design must run interactively (UX/UI work, novel architecture, or other reasons). Feature will be flagged with needs-interactive-design."},
        {label: "Cancel scoping",
         description: "End the session."}
      ]
    )
    ```

    - **No** → no flag; the Feature is eligible for headless design
      when triggered.
    - **Yes** → flag the Feature with `needs-interactive-design` at
      issue creation. Recorded in the Feature body as the design
      mode. The Feature is still eligible for triggering — the
      design skill checks the label at runtime and runs in
      foreground.
    - **Cancel** → revert to backlog, raise `USER_CANCELLED`, exit.

    **Note on UX work specifically.** If the Feature is itself a
    UX/design Feature (its deliverable is design notes and a Figma
    URL, not running code), the human handles this in the body of
    the Feature and selects "Yes" here. There is no separate "UX
    Design" session type — UX work is just a Feature whose design
    phase runs interactively, with design notes committed to the
    feature branch like any other Feature.

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

    When `<federation>` is held, the preface names the target repo
    the Feature will carry — the issue itself is filed in the
    control-plane repo `<active-repo>`, and `<target-repo>` is recorded
    in its "Target repo" field:
    ```
    Here is Feature <name> (targets <target-repo>) as it would be filed in <active-repo>:
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

    ## Design Mode

    <"Headless" — design phase can run automated when this Feature is triggered>
    OR
    <"Interactive — flagged with needs-interactive-design. The design phase must be invoked in foreground; headless automation skips this Feature.">

    <If interactive: a one-line reason — "UX/UI work" / "novel architecture" / etc.>

    ## Deployment Strategy

    <one of: No switch / Feature switch (mode + flag name) / Functionality switch / Preview switch>

    ## Notes

    <optional: switch reasoning, related context>

    ## Parent Requirement

    Closes #<parent-N> (when this Feature delivers the full Requirement)
    OR
    Part of #<parent-N> (when multiple Features share the parent)
    ```

    **Pre-flight label check.** All Features are created in
    `<active-repo>` (the control plane), in every topology —
    federation Features are filed on the control plane and carry their
    target in a field, not by being created elsewhere. `gh agentic
    check` already guarantees the control plane carries the `feature`,
    `backlog`, and `needs-interactive-design` labels, so no per-repo
    label pre-flight is needed (conditional-step carve-out — there is
    no second repo to check).

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

    - If artefact 7 = "No" (headless design) → labels are `feature,backlog`
    - If artefact 7 = "Yes" (interactive-design) → labels are `feature,backlog,needs-interactive-design`

    Write the Feature body from step 17 to a temporary file using
    the agent's `Write` tool — never via shell `echo` or heredoc, as
    user-supplied content may contain shell metacharacters (backticks,
    dollar signs, quotes) that would corrupt the file. Then create the
    Feature in `<active-repo>` — the control-plane repo — **regardless
    of topology**. Features always live on the control plane; in a
    federation the target implementation repo is recorded in the
    "Target repo" field (step 18b), not by creating the issue
    elsewhere:

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

    **Verification gate.** After each create, query the issue back
    in `<active-repo>`:

    ```bash
    gh issue view <F> --repo "<active-repo>" --json number,labels --jq '{n:.number, labels:[.labels[].name]}'
    ```

    Verify the issue exists with the expected labels. If missing
    or inconsistent, raise `ISSUE_CREATION_FAILED` (`ERROR`).

    **Partial-creation handling.** If Feature K of N fails (Features
    1..K-1 already exist on GitHub), STOP — do not continue creating
    Features K+1..N, and DO NOT proceed to step 18a, 19, 20, or
    beyond. Per "State model & cancel semantics", T1→T2 partial
    leaves the framework in a state the skill cannot auto-recover
    from. All Features are in `<active-repo>`:

    ```
    Partial Feature creation (all in <active-repo>):
      - #<F1> created successfully (labels: ...)
      - ...
      - Feature K "<title>" failed: <gh stderr>
      - Features K+1..N not attempted

    The Requirement remains at Scoping. The successfully-created
    Features are valid pipeline artefacts; the failed one needs
    investigation. Recommended:
      - Run `gh agentic repair` for the control-plane project
      - Re-invoke requirement-scoping; the orphan re-entry flow will
        detect the existing Features and surface them. From there,
        either close them and start over, or complete the work
        manually.
    ```

    Exit with `ISSUE_CREATION_FAILED`.

18a. **Add each Feature to the GitHub Project.** For each created
    Feature `<F>`, add it to the project board (so it appears in
    pipeline / kanban views AND so its "Target repo" field can be set
    in step 18b). The project ID lives in `AGENTIC_PROJECT_ID`; the
    project number is the trailing integer of the project URL. The
    Feature lives in `<active-repo>`, so the add uses the
    `<active-repo>` issue URL. Capture the returned **item id** for
    step 18b:

    ```bash
    gh project item-add <project-number> \
      --owner "${active_repo%/*}" \
      --url "https://github.com/<active-repo>/issues/<F>" \
      --format json --jq '.id'
    ```

    On failure → surface as `WARN`; do NOT block scoping (but note
    step 18b cannot run without the item id — re-run via
    `gh agentic repair` to add the item, then set the field). The
    Feature exists on GitHub and is wired as a sub-issue; missing
    project membership is a presentational gap that the pipeline
    workflow's `add-issue-to-project.yml` will repair on the next
    label change. Surface the failed adds in the exit block.

18b. **Set the "Target repo" field (only when `<federation>` is
    held).** Record the target implementation repo on the
    control-plane Feature by setting its "Target repo" ProjectV2 text
    field to `<target-repo>` (the bare repo name from artefact 3).
    Resolve the field id once, then update the item captured in 18a:

    ```bash
    # Resolve the "Target repo" field id once (reuse across Features):
    FIELD_ID=$(gh api graphql -f query='query($p:ID!){node(id:$p){... on ProjectV2{fields(first:50){nodes{... on ProjectV2FieldCommon{id name}}}}}}' \
      -F p="$AGENTIC_PROJECT_ID" \
      --jq '.data.node.fields.nodes[] | select(.name=="Target repo") | .id')

    gh api graphql -f query='
      mutation($p:ID!, $i:ID!, $f:ID!, $v:String!) {
        updateProjectV2ItemFieldValue(input:{
          projectId:$p, itemId:$i, fieldId:$f, value:{text:$v}
        }) { projectV2Item { id } }
      }' \
      -F p="$AGENTIC_PROJECT_ID" \
      -F i="<item-id from 18a>" \
      -F f="$FIELD_ID" \
      -F v="<target-repo>"
    ```

    If `FIELD_ID` is empty, the project is missing the "Target repo"
    field — run `gh agentic repair` (which provisions it) and re-try.
    When `<federation>` is unset, skip this step entirely (no target).

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

    # Per Feature, resolve the child node ID then link. The Feature
    # lives in <active-repo> — the same repo as the parent
    # Requirement — so this is a same-repo sub-issue link:
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

    **Same-repo link.** Both the Requirement and the Feature live in
    `<active-repo>` (the control plane), so the sub-issue link is
    always within a single repo — there is no cross-repo or
    cross-owner concern (this reverses the cross-repo sub-issue
    placement of #825; the target implementation repo is now carried
    by the "Target repo" field set in step 18b, not by where the
    issue lives).

    On any other failure (node-ID lookup or the mutation itself),
    propagate `ISSUE_CREATION_FAILED`. The Feature body's
    `Closes #<parent>` / `Part of #<parent>` line is a fallback; the
    API-level sub-issue link is the durable signal.

---

### Section E — Trigger confirmation

20. **Render the list and ask which Features to trigger.** For
    each created Feature, render `#<F>  <title>  [interactive]?`
    (annotate Features flagged with `needs-interactive-design`).
    These Features ARE eligible for triggering — the design skill
    detects the label at runtime and runs in foreground for them.
    The human is informed so they know which Features will need
    interactive attention.

    Also surface a recommended trigger order if dependencies between
    Features are evident in their bodies (e.g., one Feature
    references another by issue number). Recommend serial ordering
    for dependency chains; parallel triggers when independent. The
    human can accept or override.

    Branch by Feature count:

    - **1–4 Features** → use `prompt-user`:
      ```
      prompt-user(
        question: "Which Features should be triggered for design now?",
        header: "Trigger confirmation",
        options: [
          ...one option per Feature, label "#<F> <title>"...,
          {label: "All", description: "Trigger every Feature."},
          {label: "None", description: "Hold all at backlog."}
        ]
      )
      ```
    - **>4 Features** → use plain conversation. Render the list,
      ask the human to reply with numbers (`"1, 3"`, `"all"`, or
      `"none"`). Parse the reply:
      - Numbers matching rendered `#<F>` → trigger those.
      - "all" → trigger every Feature.
      - "none" → trigger nothing.
      - Anything else → ask once for clarification, cap at 3
        invalid attempts (then `USER_CANCELLED`).

21. **Apply triggers atomically (per selected Feature).** Invoke the
    `trigger-design` primitive once per selected Feature. The
    primitive reads the Feature's labels, picks `interactive-design`
    if `needs-interactive-design` is present and `in-design`
    otherwise, swaps the trigger label against `backlog`, and sets
    project status to `In Design`. This skill does not repeat the
    decision rule.

    ```
    trigger-design(issue=<F>, repo=<active-repo>)
    ```

    The Feature lives in `<active-repo>` (the control plane) in every
    topology, so the trigger always targets `<active-repo>` — the same
    as omitting `repo`. Do not reimplement `trigger-design`'s
    label-choice logic here.

    The primitive returns `{ trigger_label, status, triggered: true }`
    on success. Capture `trigger_label` per Feature so step 22 (held
    annotation) and step 24 (exit block) can report which Features
    landed on `in-design` vs `interactive-design`.

    On failure, propagate the primitive's error code:

    - `INVALID_TRIGGER_STATE` — the Feature is not at `backlog`. This
      should not happen mid-walk (we just created the Features at
      `backlog` in step 18); if it does, treat as
      `STATUS_TRANSITION_FAILED` and surface for investigation.
    - `ALREADY_TRIGGERED` (`WARN`) — the Feature is already at
      `in-design` or `interactive-design`. Treat as a no-op for that
      Feature: capture its existing trigger label, count it as
      triggered, and continue with the remaining selected Features.
    - `LABEL_OPERATION_FAILED` / `STATUS_OPERATION_FAILED` /
      `GH_API_FAILED` — propagate as `STATUS_TRANSITION_FAILED` and
      apply the partial-trigger handling below.

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
        transition the Requirement to Ready to Implement (apply ready-to-implement
        label, remove scoping, set-issue-status to Ready to Implement)
    ```

    Raise `STATUS_TRANSITION_FAILED`.

22. **Annotate held Features.** For each non-triggered Feature,
    post a comment via `post-issue-comment(repo=<active-repo>,
    issue=<F>, body=…)` — held Features, like all Features, live in
    the control-plane repo:

    ```
    Held at backlog — not triggered during scoping.
    Reason: <reason from the agent's understanding>
    Re-trigger: apply the trigger label (in-design or
    interactive-design depending on needs-interactive-design label
    presence) and run set-issue-status to In Design when ready.
    ```

    The reason is one of: `target-repo dependency (awaiting an upstream
    PR)`, or `deliberate hold by human`. (Features flagged
    `needs-interactive-design` are no longer auto-held; they're
    triggered to `interactive-design` when selected.)

---

### Section F — Closeout

23. **Transition Requirement: Scoping → Ready to Implement.**

    ```
    apply-label(repo=<active-repo>, issue=<requirement-N>,
                add=["ready-to-implement"], remove=["scoping"])
    set-issue-status(repo=<active-repo>, issue=<requirement-N>, status="Ready to Implement")
    ```

    On any failure, propagate `STATUS_TRANSITION_FAILED`.

24. **Emit the exit block** matching the actual outcome. All
    variants conform to the same Produced / Blocked / Next shape.

    **Target annotation (only when `<federation>` is held).** Each
    created Feature is rendered as `#<F> (targets <repo>)` so the
    session record shows which implementation repo each Feature
    targets. All Features live in `<active-repo>` (the control plane);
    the annotation reports the "Target repo" field value. When
    `<federation>` is unset, the `(targets …)` suffix is omitted.

    **Output A — All Features triggered:**
    ```
    === Requirement Scoping Session — Completed ===

    Produced:
      - Feature #<F1> (targets repo-a) created (triggered for design)
      - Feature #<F2> (targets repo-b) created (triggered for design)
      - Requirement #<N> transitioned: scoping → ready-to-implement

    Blocked: none

    Next: automation: design (in-design on headless Features);
          human: run design interactively for any interactive-design Features
    ```

    **Output B — Some held:**
    ```
    === Requirement Scoping Session — Completed ===

    Produced:
      - Feature #<F1> (targets repo-a) created (triggered for design)
      - Feature #<F2> (targets repo-b) created (held at backlog — target awaiting upstream PR)
      - Requirement #<N> transitioned: scoping → ready-to-implement

    Blocked: #<F2> — upstream PR

    Next: automation: design (in-design on #<F1>);
          human: re-trigger #<F2> when upstream PR merges
    ```

    **Output C — All held:**
    ```
    === Requirement Scoping Session — Completed ===

    Produced:
      - Feature #<F1> (targets repo-a) created (held at backlog — target awaiting upstream PR)
      - Feature #<F2> (targets repo-b) created (held at backlog — deliberate hold)
      - Requirement #<N> transitioned: scoping → ready-to-implement

    Blocked: #<F1> — upstream PR; #<F2> — human chose to hold

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

    Produced: nothing — Requirement #<N> already ready-to-implement.

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
