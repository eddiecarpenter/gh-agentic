---
name: feature-scoping
description: Decomposes a Requirement issue into one or more well-formed Feature issues with acceptance criteria, UX design, and deployment strategy, and hands selected features to Feature Design via the in-design label. Use when a human opens the Feature Scoping (Stage 2) recipe to scope a backlog requirement into features.
category: Session
triggers: human-interactive
loads:
  - session-init
  - gh-agentic-tool
  - capture-feature
  - set-issue-status
  - ask-user
  - session-exit
emits-exit-block: true
exit-hands-to: "automation: feature-design (in-design label on triggered features) | human (for held features)"
---

# Feature Scoping — Stage 2

## Purpose

Decompose a Requirement into one or more well-formed Feature issues.
Design any UX/UI impact now — not during implementation.

Scoping is a mandatory phase — it produces the Feature issue artefact that design depends on.
This session is used when scoping was not completed inline during the Requirements Session.

## When to Run

When a Requirement issue is in `backlog` state and scoping was not completed inline.
Run this before Feature Design — you cannot design what has not been scoped.

## How to Start

Open Goose and select the **Feature Scoping (Stage 2)** recipe.

## Requirement Label Transitions

During a scoping session, the agent manages the parent requirement's lifecycle labels:

1. **Session start** (after loading the requirement): removes `backlog`, applies `scoping`
2. **Session end** (after all features created and `in-design` applied): removes `scoping`, applies `scheduled`

The full requirement label lifecycle: **Backlog → Scoping → Scheduled → Done**

## What the Agent Does

1. Prints: `=== Feature Scoping Session (Phase 2) — Started ===`
2. **Verify framework version is current:**
   ```bash
   gh agentic info
   ```
   - Local version matches control plane → proceed
   - Version mismatch → run `gh agentic mount` to sync, then confirm versions match before continuing
3. Lists available requirements in `backlog` state
3. Waits for the human to select a requirement
4. Transitions the requirement from `backlog` to `scoping`.
   **Inline status update** — immediately after applying the `scoping` label, set the
   project status to `Scoping` following the pattern in `set-issue-status.md`:
   - Verify `AGENTIC_PROJECT_ID` is set — hard-fail if not
   - Resolve the issue node ID
   - Find or create the project item
   - Resolve the Status field and option IDs
   - Set status to `Scoping`
5. Works through seven artefacts to define the feature.
   **Present each artefact to the human and wait for explicit confirmation before
   proceeding to the next.** Do not batch artefacts or produce the next one until
   the human has approved or revised the current one.

   Every artefact confirmation below is delegated to `skills/ask-user.md` — the
   interaction shape, option constraints, and typed fallback behaviour live
   there. Do not restate them here.

   - **Raw idea summary** — invoke `skills/ask-user.md` with Shape 1 (confirm/revise) on the proposed summary.
   - **Problem statement** — invoke `skills/ask-user.md` with Shape 1 (confirm/revise) on the proposed statement.
   - **Feature definition** — includes a user story statement in `As a [user], I want [goal], so that [benefit]` format. Invoke `skills/ask-user.md` with Shape 1 (confirm/revise) on the proposed feature definition plus user story.
   - **MVP scope** — invoke `skills/ask-user.md` with Shape 1 (confirm/revise) on the proposed MVP.
   - **Parallel/serial checkpoint** — invoke `skills/ask-user.md` with Shape 2 (multi-choice) presenting *parallel features* vs. *single feature with ordered tasks*, with the recommended option flagged. Independent work → multiple features (parallel). Sequential work → one feature with ordered tasks (same branch, same PR). Never creates multiple features with implied serial dependencies.

     **Three-dimensional cost principle** — before recommending parallel features, weigh:
     - **Token cost**: each parallel feature requires its own full Design + Dev session
     - **Build cost**: more branches = more CI runs = more build minutes
     - **Time overhead**: parallel features require coordination and merge ordering

     If the combined work fits comfortably in a single dev session, recommend one feature
     with ordered tasks and explain the cost of splitting. Only recommend parallel features
     when the work is substantial enough that parallelism delivers real value. Record the
     recommendation and reasoning in the scoping summary.
   - **Acceptance criteria** — use Given/When/Then format for every criterion (not checkboxes, not prose). Minimum three criteria: one success case, one failure case, and at least one edge case. Invoke `skills/ask-user.md` with Shape 1 (confirm/revise) on the proposed criteria.
   - **UX design (if applicable)** — invoke `skills/ask-user.md` with Shape 1 (confirm/revise) on the proposed UX block.
   - **Deployment strategy** — invoke `skills/ask-user.md` with Shape 2 (multi-choice) asking *"How should this feature reach users once deployed?"* with these options:
     - **No switch** — deployed and immediately live (appropriate for bug fixes, MVP phase, or infrastructure changes)
     - **Feature switch** — hidden until a release decision is made (default for features and enhancements)
     - **Functionality switch** — permanent, gated by licence or tier (enters pipeline as a requirement in its own right)
     - **Preview switch** — user opt-in to a new experience while old version remains (enters pipeline as a requirement in its own right)

     Downstream confirmations — each via `skills/ask-user.md`:
     - If **Feature switch** is chosen → invoke Shape 2 to confirm mode (`permanent disable` when work may be incomplete or breaking; `toggle` when code is safe but release is pending); then invoke Shape 1 (confirm/revise) on the proposed flag name. Note: switch removal is a follow-up requirement after full rollout.
     - If **No switch** is chosen for a feature or enhancement → invoke Shape 2 or free-text ask-user to capture the reason; record it in the issue body.

     See `concepts/feature-switches.md` for the full taxonomy.
   - **Parking lot review** — invoke `skills/ask-user.md` with Shape 1 (confirm/revise) on the parking lot entries.
6. **Impact delta on rejection or modification** — when the human rejects or modifies
   a proposed feature after others have already been accepted:
   - Re-evaluate all previously accepted features: does this rejection or change affect
     their scope, dependencies, or ordering?
   - Surface only features flagged as affected and invoke `skills/ask-user.md`
     with Shape 1 (confirm/revise) for each affected feature, so the human
     re-confirms or edits it
   - Features not flagged are not re-presented — they remain accepted as-is
7. Verifies user story is present and complete before creating the issue
8. Creates Feature issues in the domain repo with `feature` + `backlog` labels
9. Wires sub-issue relationship: Feature → parent Requirement
10. **Explicit trigger confirmation** — invoke `skills/ask-user.md` presenting the full list of agreed features and asking *"Which of these features should be triggered for design now?"* The human replies with a selection (numbers, `all`, or free text naming the features).
    - Apply `in-design` only to features the human explicitly selects — and remove the `backlog` label in the same operation. A feature carries one status label at a time.
    - **Inline status update** — for each feature that receives the `in-design` label,
      immediately set its project status to `In Design` following the pattern in
      `set-issue-status.md`:
      - Verify `AGENTIC_PROJECT_ID` is set — hard-fail if not
      - Resolve the issue node ID
      - Find or create the project item
      - Resolve the Status field and option IDs
      - Set status to `In Design`
    - Features not selected remain at `backlog` with a note in the issue body:
      `> Not triggered during scoping — awaiting human decision.`
    - For features held due to cross-repo dependencies, leave at `backlog` and document
      the dependency in the issue
11. Transitions the requirement from `scoping` to `scheduled`.
    **Inline status update** — immediately after applying the `scheduled` label, set the
    requirement's project status to `Scheduled` following the pattern in `set-issue-status.md`:
    - Verify `AGENTIC_PROJECT_ID` is set — hard-fail if not
    - Resolve the issue node ID
    - Find or create the project item
    - Resolve the Status field and option IDs
    - Set status to `Scheduled`
12. Emits the canonical exit block (see `skills/session-exit.md`) matching the
    actual outcome. All three variants conform to the same Produced / Blocked /
    Next shape.

    **All features triggered:**
    ```
    === Feature Scoping Session (Phase 2) — Completed ===

    Produced:
      - Feature issue #N created (triggered for design)
      - Feature issue #N created (triggered for design)
      - Requirement #N transitioned: scoping → scheduled

    Blocked: none

    Next: automation: feature-design (in-design label on #N, #N)
    ```

    **Some features held:**
    ```
    === Feature Scoping Session (Phase 2) — Completed ===

    Produced:
      - Feature issue #N created (triggered for design)
      - Feature issue #N created (held at backlog — cross-repo dependency)
      - Requirement #N transitioned: scoping → scheduled

    Blocked: #N — depends on <feature/PR reference> to merge first

    Next: automation: feature-design (in-design label on #N); human: re-trigger #N once the upstream PR merges
    ```

    **No features triggered (all held):**
    ```
    === Feature Scoping Session (Phase 2) — Completed ===

    Produced:
      - Feature issue #N created (held at backlog — cross-repo dependency)
      - Feature issue #N created (held at backlog — cross-repo dependency)
      - Requirement #N transitioned: scoping → scheduled

    Blocked: #N, #N — all depend on <feature/PR reference> to merge first

    Next: nothing — re-trigger each feature once the upstream PR merges
    ```

13. **Terminate the session.** Immediately after emitting the exit block, invoke
    the host runtime's session-close API if exposed; otherwise halt. No further
    work may occur in this session (see RULEBOOK — Session Termination).

## Outputs

- One or more Feature issues in the domain repo, each written using the `capture-feature.md` template
- `in-design` label applied — triggers automatic Feature Design Session
- Parent requirement transitioned from `scoping` to `scheduled`

## Rules

- Serial vs parallel decomposition: independent capabilities → separate features; sequential capabilities → one feature with ordered tasks; never create multiple features with implied serial dependencies
- **Three-dimensional cost principle**: before recommending parallel features, weigh token cost, build cost, and time overhead against the parallelism benefit. Batch small independent changes into one feature with ordered tasks unless the work is substantial enough that parallelism delivers real value.
- Push toward MVP — smallest version that delivers real value
- Feature issue structure and format is defined by `capture-feature.md` — follow it exactly
- Acceptance criteria must use Given/When/Then format — not checkboxes, not prose
- UX design must be done now, not deferred to implementation
- Never accept solution criteria — convert to outcome criteria
- If an idea is out of scope, capture it in the parking lot
- **Explicit trigger confirmation**: never apply `in-design` automatically to all agreed features. Present the list and apply only to features the human explicitly selects. Features not selected remain at `backlog`.
- **Impact delta on changes**: when the human rejects or modifies a feature, re-evaluate previously accepted features for impact and re-confirm only those affected
- **Interaction shape is delegated**: every confirmation, selection, and disambiguation moment invokes `skills/ask-user.md` inline. Do not restate option counts, label-length limits, free-text escape rules, or typed-fallback behaviour here — those live in `ask-user.md` and must not drift

## Notification

The exit block (see step 12 above) serves as the session notification. No separate notification needed.

## Next Step

The Feature Design Session triggers automatically when `in-design` is applied.
