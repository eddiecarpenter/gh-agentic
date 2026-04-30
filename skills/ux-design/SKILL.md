---
name: ux-design
description: Creates and extends a project's canonical UX specification at docs/UX_DESIGN.md — a long-lived, governed artefact sibling to docs/ARCHITECTURE.md that defines the project's UX rules, decision-axis verdicts, sanctioned deviations, and standards. Runs at project birth (init mode) when the project has UI scope, on demand to extend rules or promote sanctioned deviations to canonical rules (extend mode), and from feature-design's interactive flow when a deviation is decided (add-deviation mode). Use when a human is bootstrapping the UX spec for a new project, extending the spec with new sections or rules, promoting a frequently-cited deviation to a canonical rule, or recording a freshly-decided deviation that arose during interactive design. Use even when the caller doesn't say "ux-design" — phrases like "create the UX spec", "set up the UX rules", "add a UX rule", "promote this deviation", or "log this UX exception" should trigger this skill.
triggers: human
user-invocable: true
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/step-skip-rule.md
  - skills/definitions/verification-procedure.md
  - skills/ux-design/baseline.md
emits-exit-block: true
exit-hands-to: human (in init / extend modes) or feature-design (when called from feature-design's interactive flow in add-deviation mode)
---

# UX Design

## Goal

Produce, maintain, and extend a project's canonical UX specification at
`docs/UX_DESIGN.md` — a single durable file that:

- Names the project's UX rules using stable IDs (`R-<AREA>-<TOPIC>-<NN>`).
- Records project-specific verdicts on the cross-cutting decision axes
  (validation timing, modal dismissal, required indicator policy, row
  click semantics, toast position, etc.) with rationale and citations.
- References design-system tokens by name (not value) so the doc stays
  in sync with the theme.
- Carries a **Sanctioned Deviations** section as an append-only
  precedent index — every legitimate deviation, written as a principle
  with a stable ID (`D-<AREA>-<TOPIC>-<NN>`), so future features facing
  similar territory either re-use the precedent (standardisation) or
  add a new principle (refinement).

The skill does **not** design. It does not draw layouts, choose
palettes, or write copy. The human is the designer; the skill is a
checklist that walks them through the decisions a careful UX practice
would make, captures their answers, and enforces the discipline that
keeps the resulting doc useful: best-practice defaults, principle-shaped
deviations, and a feedback loop that flags drift back into the rule set.

## Output Artefacts

The skill has three valid terminal outputs — exactly one fires per
invocation:

**A. `docs/UX_DESIGN.md` created (init mode).**
- File written on the current branch with all canonical sections
  populated, an empty `## Sanctioned Deviations` section, and a
  preamble carrying the project's standards declaration and tunable
  `deviation_threshold`.
- A confirmation-gate transcript showing the human's explicit
  acceptance of each section before persistence.
- (Brownfield only) A GitHub Requirement issue listing inconsistencies
  between current code and the canonical rules — the bug list, kept
  separate from the doc.

**B. `docs/UX_DESIGN.md` extended (extend mode).**
- The same file with one of: a new rule appended to a section, an
  existing rule revised, a deviation promoted to a canonical rule
  (the deviation entry is marked `Promoted to R-<NEW-ID>` and the new
  rule is added).
- All changes committed (but not pushed — the human owns push and PR).

**C. Sanctioned deviation appended (add-deviation mode).**
- A new `### D-<AREA>-<TOPIC>-<NN>` block appended to the
  `## Sanctioned Deviations` section, OR an existing deviation cited
  by the calling Feature when an existing principle covers the case.
- An inline drift-flag surfaced when the area's deviation count
  reaches the `deviation_threshold` (default 3+). The flag does NOT
  block the deviation being added; it surfaces the meta-finding to
  the human so they can choose to amend the rule, refactor the code,
  or accept the deviation.

In all cases the working tree is staged for commit but never
auto-committed; the human reviews and pushes.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy applied
  to `USER_CANCELLED`, `INIT_PRECONDITION_FAILED`,
  `FILE_WRITE_FAILED`, `REVISION_LOOP_LIMIT`,
  `DEVIATION_REASON_REJECTED`, `MODE_INVALID`.
- `skills/definitions/verification-procedure.md` — the change-pinning
  rule (file existence and structural completeness verified after
  every write).
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement
  rule preventing silent skipping of any decision axis or section.
- `skills/ux-design/baseline.md` — universal best-practice rules
  shipped with the skill. Every project inherits these unless
  explicitly overridden in its own `docs/UX_DESIGN.md`.

## Dependencies

- `skills/prompt-user/SKILL.md` — used at every decision-axis prompt,
  every confirm/revise gate, and the deviation walk-through.
- `skills/gh-agentic/SKILL.md` — used in step 1 to read repo
  metadata (stack, UI scope from `AGENTIC_STACK`) and to file the
  brownfield Requirement issue if applicable.
- `skills/post-issue-comment/SKILL.md` — used in brownfield init to
  surface the inconsistency-bug-list as a Requirement issue body.

## Steps

The **step-skip rule** from `skills/definitions/step-skip-rule.md`
applies to every step below: no step may be skipped without the agent
first emitting, in its response stream, which step is being skipped
and the concrete reason why.

**Calling primitives — non-negotiable.** Throughout this skill,
expressions of the form `prompt-user(...)`, `post-issue-comment(...)`,
etc. are NOT documentation, NOT pseudocode, and NOT suggestions of
what the agent should do equivalently inline. They are explicit
instructions to invoke the named primitive **via the agent's Skill
tool**. The agent MUST NOT substitute an equivalent inline call
(e.g., `AskUserQuestion` directly instead of `prompt-user`); doing
so loses the primitive's UX features, error handling, and structured
return value. If the agent is about to bypass a primitive, it must
stop and emit a step-skip justification first naming which primitive
it is replacing and why.

**Resolving the active repo.** Resolve once in step 1 via:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

and reuse the value as `<active-repo>`.

**Section-gate pattern.** Decision-axis prompts and per-section
review gates use a uniform shape:

1. Agent renders the question (with the best-practice default
   pre-selected and a one-line citation of why it's the default).
2. `prompt-user` with options: **Accept default**, **Override**,
   **Tell me more**, **Cancel**.
   - Accept default → record the default verdict, continue.
   - Override → ask the human for their answer (free text or a
     refined option set), record, continue.
   - Tell me more → render the citation paragraph from
     `baseline.md` for this axis; loop back to the prompt.
   - Cancel → raise `USER_CANCELLED` (`WARN`); end skill cleanly.
     Any in-progress draft is discarded; no file is written.

The default-pre-selection is the load-bearing UX feature: a human
who accepts every default produces a doc that is already
best-practice-compliant. Conscious overrides are conscious; they
get a recorded reason in the doc.

---

### Section A — Setup

1. **Announce the session and resolve mode.** Print the banner
   verbatim:

   ```
   ==========================================================
   === UX Design Session — Started                          ===
   ==========================================================
   You are now in UX Design mode. This skill creates and
   maintains your project's canonical UX specification at
   docs/UX_DESIGN.md.

   This skill does not design. You are the designer; this
   is a checklist that captures your project's UX decisions
   into a doc that future features (and the agent reading
   them) can rely on.
   ==========================================================
   ```

   Resolve the active repo per the rule above. Then determine the
   skill's mode:

   - If invoked from another skill (e.g., `feature-design`) with an
     explicit `mode` argument, use that.
   - If `docs/UX_DESIGN.md` does NOT exist on the current branch,
     default to **init**.
   - If `docs/UX_DESIGN.md` exists and the caller didn't specify a
     mode, ask via `prompt-user`:
     ```
     prompt-user(
       question: "What would you like to do?",
       header: "Pick mode",
       options: [
         {label: "Extend rules / promote a deviation",
          description: "Add a new rule, revise an existing one, or promote a frequently-cited deviation to a canonical rule."},
         {label: "Record a deviation",
          description: "Log a sanctioned deviation that arose during design."},
         {label: "Cancel",
          description: "End the session."}
       ]
     )
     ```

   Branch on the answer:
   - **init** → Section B (Init).
   - **Extend** → Section C (Extend).
   - **Record a deviation** → Section D (Add-deviation).
   - **Cancel** → `USER_CANCELLED` (`WARN`); exit.

2. **Load the baseline and any existing doc.** Read
   `skills/ux-design/baseline.md` into working memory — the
   universal best-practice rules used as defaults. If
   `docs/UX_DESIGN.md` exists, read it too — its canonical sections
   override defaults; its `## Sanctioned Deviations` section is the
   precedent index for add-deviation mode.

   Read repo metadata via `gh agentic info --raw` (or equivalent):
   - `AGENTIC_STACK` — used to confirm UI scope.
   - Project component library / theme path (asked from human if
     not in metadata).

   For init mode, fail with `INIT_PRECONDITION_FAILED` (`ERROR`) if:
   - `docs/ARCHITECTURE.md` does not exist (UX spec sits alongside
     SA; SA is the architectural anchor and must come first).
   - The project does not have UI scope (the wizard's UI flag is
     false). Surface the message: *"This project does not appear
     to have UI scope per AGENTIC_STACK. ux-design is for UI
     projects. Re-run after UI work is added, or override by
     re-invoking with `--force`."* Exit cleanly.

---

### Section B — Init mode

3. **Detect greenfield vs brownfield.** Inspect the repo:

   - Greenfield = no UI code exists yet (no frontend framework
     scaffold detected, no component files matching the project's
     stack convention).
   - Brownfield = UI code exists. The project has de-facto patterns
     to audit.

   Surface the detection to the human and confirm via `prompt-user`
   (the human knows the repo better than the agent).

4. **(Greenfield path)** Walk the decision axes. For each axis listed
   in `baseline.md` (validation timing, modal dismissal, required
   indicator, row click, toast position, empty-state, destructive
   recovery, toast persistence, touch-target floor, plus
   foundation/layout/forms/buttons/tables/modals/feedback section
   prompts), apply the **Section-gate pattern** above. Record the
   human's verdict + rationale per axis.

   The output of this walk is the body of every canonical section in
   `docs/UX_DESIGN.md`.

5. **(Brownfield path)** Run the six-step audit from
   `DESIGN_NOTES.md` §4 (the design notes the framework was bootstrapped
   from):

   1. **Audit existing code.** Inventory the de-facto patterns
      across the codebase — buttons, forms, modals, toasts,
      tables. Use a systematic file walk with `Glob` + `Read`.
   2. **Identify conflicts.** Where the same UX concern has
      multiple solutions (e.g., 3 different button-confirmation
      patterns).
   3. **Score against best practice.** For each conflict, the
      modern best-practice answer wins by default — current
      code does not get a presumption of correctness.
   4. **Extract canonical rules.** Write rules prescriptively,
      best-practice-first. Apply the **Section-gate pattern** —
      the agent proposes the rule, the human confirms or
      overrides.
   5. **Capture intentional deviations.** Per conflict, the human
      identifies cases where the deviation is the right call.
      Each gets a deviation entry per the schema in Section D.
   6. **File unintentional deviations as a separate Requirement
      issue.** Use `gh issue create` (or `post-issue-comment` on a
      pre-existing issue) to file a `requirement,backlog`-labelled
      issue listing the inconsistencies. Each item cites the
      canonical rule ID it violates. The doc is forward-looking;
      the bug list is transient.

   Critical: structurally separate "what the rule says" from "what
   the code does". The latter never dictates the former.

6. **Render the draft and confirm.** Compose the full document body
   from the captured decisions. Render it to the conversation:

   ```
   Here is the docs/UX_DESIGN.md that would be filed:

   <verbatim file content, fenced>
   ```

   Then `prompt-user`:

   ```
   prompt-user(
     question: "Does this UX_DESIGN.md look right?",
     header: "Confirm UX spec",
     options: [
       {label: "Yes — write it",
        description: "Write to docs/UX_DESIGN.md and stage for commit."},
       {label: "Revise a section",
        description: "Pick which section to revisit."},
       {label: "Cancel",
        description: "End the session; no file written."}
     ]
   )
   ```

7. **Write the file.** Use the agent's `Write` tool — never via
   shell `echo` or heredoc, since user-supplied content may contain
   shell metacharacters. The first line MUST be the marker:

   ```
   <!-- ux-design:v1 -->
   ```

   so audits and downstream skills (`feature-design`'s deviation
   detection, the release-time drift report) can identify the file
   by content rather than path.

   Stage the file with `git add docs/UX_DESIGN.md`. Do NOT commit
   on the human's behalf. The human reviews `git status` and
   commits.

   On `Write` failure → `FILE_WRITE_FAILED` (`ERROR`).

8. **(Brownfield only)** File the inconsistency-bug-list as a
   Requirement issue. Title: `chore(ux): reconcile <count> existing
   inconsistencies with docs/UX_DESIGN.md`. Body lists each
   inconsistency with rule ID + file path + one-line description.
   Use `gh issue create` with labels `requirement,backlog`. Surface
   the issue number to the human.

9. **Exit init mode.** Emit the init-mode exit block (Output A).

---

### Section C — Extend mode

10. **Pick the operation.** `prompt-user`:

    ```
    prompt-user(
      question: "What change do you want to make?",
      header: "Extend operation",
      options: [
        {label: "Add a new rule",
         description: "Append a rule to an existing section, or add a new section."},
        {label: "Revise an existing rule",
         description: "Update the wording or scope of an existing R-<AREA>-<TOPIC>-<NN>."},
        {label: "Promote a deviation to a canonical rule",
         description: "Convert a sanctioned deviation cited 3+ times into a canonical rule."},
        {label: "Cancel",
         description: "End the session."}
      ]
    )
    ```

11. **Add or revise a rule.** Walk the human through the rule's
    fields: `id`, `area`, `topic`, `MUST/SHOULD/MAY` semantics,
    rule statement, rationale, anti-example pair (do this / not
    that), token references, file-path scope (for lint-ability).

    The agent proposes a rule shape per the section's existing
    style; the human edits. Apply the **Section-gate pattern**.

12. **Promote a deviation.** Surface deviations with citation
    counts ≥ `deviation_threshold` and ask which to promote. For
    the chosen deviation:

    1. Draft a new canonical rule from the deviation's `Applies
       when` (becomes the rule's scope) and `Reason` (becomes the
       rule's rationale).
    2. The agent proposes; the human confirms via the
       Section-gate pattern.
    3. Append the new rule to the appropriate section.
    4. Mark the original deviation entry: append a line
       `**Promoted to R-<NEW-ID> on <date> after <count>
       citations.**` Do NOT delete the entry — preserve the
       audit trail.

13. **Render, confirm, write.** Same shape as steps 6–7 but
    against the modified file rather than a fresh one. Stage for
    commit; do not auto-commit.

14. **Exit extend mode.** Emit the extend-mode exit block.

---

### Section D — Add-deviation mode

15. **Receive the candidate from the caller.** When invoked from
    `feature-design`'s interactive flow, the call carries:

    - `feature_number` — the Feature # the deviation arose under.
    - `rule_id` — the canonical rule the candidate deviates from
      (`R-<AREA>-<TOPIC>-<NN>`).
    - `proposed_deviation` — a free-text statement of what is being
      done differently.
    - `proposed_reason` — a free-text statement of why.

    When invoked standalone by a human (rare; usually for
    after-the-fact logging), prompt for these fields.

16. **Surface existing deviations in the area.** Read the
    `## Sanctioned Deviations` section, filter to entries whose
    `Rule deviated from` matches `<AREA>` of the candidate
    (i.e., other deviations from rules in the same area). For each,
    render to the human:

    ```
    Existing deviations in this area:

    ### D-<AREA>-<TOPIC>-<NN>: <title>
    Applies when: <condition>
    Reason: <reason>
    Cited by: <list>

    ### D-<AREA>-<TOPIC>-<NN>: ...
    ```

    Then `prompt-user`:

    ```
    prompt-user(
      question: "Does any existing deviation cover this case?",
      header: "Reuse precedent?",
      options: [
        {label: "Yes — D-<AREA>-<TOPIC>-<NN> applies",
         description: "Cite this deviation; no new entry needed."},
        ... (one option per existing deviation, up to 4) ...,
        {label: "No — this is genuinely new",
         description: "Walk through a new deviation entry."},
        {label: "Cancel",
         description: "End without recording. The calling skill's hard-block remains in effect."}
      ]
    )
    ```

    Branch:
    - **An existing deviation applies** → step 17 (cite, no new
      entry).
    - **Genuinely new** → step 18 (walk through a new entry).
    - **Cancel** → `USER_CANCELLED` (`WARN`); the caller's
      hard-block stays active.

17. **Cite an existing deviation.** Append the calling Feature to
    the chosen deviation's `Cited by` list:

    ```
    - Feature #<feature_number> — <one-line context from caller>
    ```

    Stage `docs/UX_DESIGN.md`. Continue to step 19.

18. **Walk through a new deviation entry.** Apply the 5-field
    schema, with **the agent pressure-testing the Reason field**:

    1. **Rule deviated from**: `R-<AREA>-<TOPIC>-<NN>` (taken from
       caller's `rule_id`).
    2. **Applies when**: ask the human for the condition that
       triggers the deviation. Pressure-test: *"Restate this as a
       property that another feature might also meet, not as a
       description of this Feature's specifics."*
    3. **Deviation**: what is done differently (from caller's
       `proposed_deviation`).
    4. **Reason**: why the deviation is correct under the
       Applies-when condition. **Anti-pattern check (mandatory):**
       reject the Reason if it:
       - Names a specific Feature (`Feature #N`).
       - Names a specific file or component (`ScenarioBuilder.tsx`).
       - Refers to "this case" / "this feature" / "this screen".

       On rejection, surface to the human:

       *"The Reason field references a specific case — restate as
       a principle. What's the property of the case that justifies
       the deviation, independent of the case itself? If you can't
       generalise the reason, the deviation may be a one-off bug
       rather than a sanctioned exception."*

       Loop until the Reason passes the check. Cap at 5 iterations
       — on the 5th, raise `DEVIATION_REASON_REJECTED` (`WARN`)
       and surface to the human: "Reason still references specific
       cases after 5 attempts. The deviation is not ready to be
       canonicalised. Either reconsider whether this is a real
       deviation, or ask for help formulating the principle."
    5. **Cited by**: initialise with the calling Feature.

    Assign a new ID `D-<AREA>-<TOPIC>-<NN>` (next sequence number
    in the area).

    Render the proposed entry; confirm via the Section-gate
    pattern. Append to `## Sanctioned Deviations`.

19. **Inline drift-flag check.** Count deviations in the area
    after this addition. If count ≥ `deviation_threshold` (read
    from the doc preamble, default 3):

    Surface to the human:

    *"§<AREA> now has <count> sanctioned deviations. This is at
    or above the drift threshold (<threshold>). Two diagnoses to
    consider:*

    *- **Gap**: the canonical rule may not fit the territory we
       keep working in. Consider amending or replacing it via
       `ux-design` extend mode.*
    *- **Drift**: we may not be following our own recommendations.
       Consider whether the deviations are bugs to fix in code
       rather than principles to canonicalise.*

    *Continuing for now; the release-time audit will re-check this
    threshold and surface it in the release notes."*

    The flag is **informational, not blocking**. The deviation has
    already been logged; the flag's purpose is to surface the
    meta-finding so the human can act on it later.

20. **Write the file.** Same as step 7. Stage for commit.

21. **Exit add-deviation mode.** Emit the add-deviation exit
    block (Output C). Hand control back to the calling skill
    (typically `feature-design`).

---

### Section E — Verification

22. **Verify the file.** Read the written file back. Confirm:
    - The leading `<!-- ux-design:v1 -->` marker is present.
    - All canonical sections are present (init mode) or the
      modified section is structurally intact (extend / add-deviation
      modes).
    - The `## Sanctioned Deviations` section exists (even if
      empty).
    - The preamble carries `deviation_threshold` and the standards
      declaration.

    Verification failure → `FILE_WRITE_FAILED` (`ERROR`).

---

### Section F — Exit

23. **Emit the exit block** matching the actual outcome.

    **Output A — Init complete:**
    ```
    === UX Design Session — Completed (init) ===

    Produced:
      - docs/UX_DESIGN.md (staged for commit)
      - <count> canonical rules across <count> sections
      - <count> sanctioned deviations seeded from brownfield audit
      - <Requirement issue # if brownfield, else "no issue filed">

    Next: human: review the staged changes, commit, and push.
          requirements-session will now accept UX-flavoured
          Requirements.
    ```

    **Output B — Extend complete:**
    ```
    === UX Design Session — Completed (extend) ===

    Produced:
      - docs/UX_DESIGN.md updated (staged for commit)
      - <one-line summary of the change: rule added / revised /
        deviation promoted>

    Next: human: review the staged change and commit.
    ```

    **Output C — Deviation recorded:**
    ```
    === UX Design Session — Completed (add-deviation) ===

    Produced:
      - <D-NEW-ID added | existing D-X-Y-NN cited by Feature #N>
      - <drift flag if threshold reached, else "no drift flag">

    Next: feature-design proceeds with the deviation logged.
          The hard-block on the calling skill is now released.
    ```

    **Output D — Cancelled:**
    ```
    === UX Design Session — Cancelled ===

    Produced: nothing. Working tree returns to its state before
    this skill ran.
    ```

24. **Terminate the session.** Per `emits-exit-block: true`,
    invoke the host runtime's session-close API if available;
    otherwise halt.

## Verification

Per `skills/definitions/verification-procedure.md` "Section format".
Skill-specific commands:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/ux-design/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/ux-design/SKILL.md
```

Pass criteria: both commands exit 0.

## Error Handling

- `USER_CANCELLED` (any cancel point) → severity `WARN`. End cleanly.
  Discard any in-progress draft; working tree returns to pre-skill
  state.
- `INIT_PRECONDITION_FAILED` from step 2 (init mode preconditions:
  ARCHITECTURE.md missing, project lacks UI scope) → severity `ERROR`;
  propagate. Surface remediation.
- `FILE_WRITE_FAILED` from step 7, 13, 20, or 22 (Write tool errored,
  or post-write verification found a missing section / missing marker) →
  severity `ERROR`; propagate.
- `REVISION_LOOP_LIMIT` from any Section-gate (5 revisions elapsed) →
  severity `WARN`; surface the current draft, recommend Confirm-as-is
  or Cancel.
- `DEVIATION_REASON_REJECTED` from step 18 (5 anti-pattern-check
  failures elapsed) → severity `WARN`; surface to human; the deviation
  is not recorded; the calling skill's hard-block remains in effect.
- `MODE_INVALID` from step 1 (caller passed an unrecognised `mode`
  argument) → severity `ERROR`; propagate.
- All other errors: propagate (default).
