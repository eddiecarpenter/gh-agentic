---
name: requirements-session
description: Captures a new business need as a Requirement GitHub issue through a conversational, agent-led interview — listening, challenging vague or solution-framed input, and confirming the result with the human before creating the issue. Use when a human wants to record a new business need, idea, or enhancement request as a Requirement. Use even when the user doesn't explicitly say "requirements session" — phrases like "I want to capture a new requirement", "let's record this idea", "add a new business need", "capture a feature request" should trigger this skill.
triggers: human-interactive
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/verification-procedure.md
  - skills/prompt-user/SKILL.md
  - skills/gh-agentic/SKILL.md
emits-exit-block: true
exit-hands-to: human — Requirement issue created; run Feature Scoping when ready to scope it
---

# Requirements Session

## Goal

Capture one discrete business need as a `requirement`-labelled GitHub
issue through a conversational interview, with the human confirming
the captured content before the issue is created.

## Output Artefacts

- A new GitHub issue created in the active repo, with:
  - `requirement` label applied.
  - `backlog` label applied (when the human confirms the capture is
    final), or `draft` label applied (when the human flags the
    requirement as still-being-refined).
  - Body matching the structure described in step 4 (User Story,
    Business Need, Success Criteria, Notes).
- A confirmation-gate transcript showing the human's explicit
  acceptance of the captured content before the issue was created.
  Recorded in conversation, not as a file.
- A hand-off message naming the issue number, URL, and the next
  recommended phase (Feature Scoping).

No file artefacts. The issue is the durable record.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy applied
  to `USER_CANCELLED`, `ISSUE_CREATION_FAILED`, `DUPLICATE_DETECTED`.
- `skills/definitions/verification-procedure.md` — the change-pinning
  rule the Verification section follows (issue presence is verified
  by querying GitHub, not by the agent's claim).

## Dependencies

- `skills/prompt-user/SKILL.md` — used in step 2 (interview),
  step 5 (confirmation gate), and step 7 (handle-revision loop) to
  ask the human structured questions.
- `skills/gh-agentic/SKILL.md` — used in step 1 to query existing
  requirements (so the agent can spot duplicates) and in step 6 to
  query the resulting issue back via `gh agentic status requirement
  <N> --raw` for the verification check.

## Steps

1. **Survey existing requirements.** Before asking the human
   anything, query the active repo's open requirements so the agent
   has context to spot duplicates or closely-related needs:

   ```bash
   gh agentic status requirements --raw
   ```

   Hold the parsed list (number, title, stage) in working memory.
   This is the agent's context for step 2; do not surface it to the
   user yet.

2. **Open the interview.** Surface a short framing message and ask
   the human for their initial description of the business need:

   > "I'll help you capture a new requirement. Tell me about the
   > business need — what's the problem, who has it, and what would
   > 'solved' look like?"

   Wait for the human's reply. Their response is the raw input the
   rest of this skill structures.

3. **Challenge vagueness or solution-framing.** Read the human's
   reply against two failure modes:

   - **Vague** — "users want better reporting" tells you nothing
     about what report, for whom, or what behaviour. Push back: ask
     for a concrete scenario, a specific role, or a measurable
     outcome.
   - **Solution-framed** — "add a CSV export button to the
     dashboard" describes a solution, not a need. Push back: what
     is the user trying to accomplish that the export would enable?
     Capture the *need*, not the *fix*.

   Loop on prompt-user until the captured input has both: (a) a
   specific user/role, (b) a concrete problem or outcome (not a
   feature). The agent's job here is friction, not transcription —
   bad requirements compound downstream.

   **Duplicate check:** if the emerging need overlaps an existing
   open requirement (from step 1's list), surface the duplicate
   to the human and ask whether to (i) treat the new input as a
   refinement of the existing requirement, (ii) capture it as
   distinct, or (iii) cancel.

4. **Draft the issue body.** Build the body from the captured
   content using this template:

   ```markdown
   ## User Story

   **As a** <role>
   **I want** <capability>
   **So that** <outcome>

   ## Business Need

   <2–4 sentences explaining the underlying problem and why it
   matters. Phrased as a problem, not a solution.>

   ## Success Criteria

   - <observable outcome 1>
   - <observable outcome 2>
   - <observable outcome 3>

   ## Notes

   <optional — context, constraints, related work, open questions>
   ```

   Hold the drafted body in working memory; do NOT create the
   issue yet.

5. **Confirmation gate.** Surface a structured summary to the human
   via `prompt-user`:

   ```
   prompt-user(
     question: "Does this capture your requirement correctly?",
     header: "Confirm requirement",
     options: [
       {label: "Yes — create the issue (backlog)",
        description: "Final capture; ready for scoping."},
       {label: "Yes, but mark as draft (still refining)",
        description: "Apply 'draft' instead of 'backlog'."},
       {label: "Revise — let me change something",
        description: "Loop back to interview with the change."},
       {label: "Cancel — don't capture this",
        description: "End the session without creating an issue."}
     ]
   )
   ```

   The summary surfaced alongside the question must include the
   complete drafted body from step 4 — no omissions. The human
   needs to see exactly what would be filed.

   Branch on the answer:
   - **Yes (backlog)** → continue to step 6 with labels
     `requirement,backlog`.
   - **Yes, but mark as draft** → continue to step 6 with labels
     `requirement,draft`.
   - **Revise** → continue to step 7.
   - **Cancel** → raise `USER_CANCELLED` (`WARN`), surface a brief
     "no requirement captured" message, end the skill cleanly.

6. **Create the issue.** Invoke:

   ```bash
   gh issue create \
     --repo "<active-repo>" \
     --title "<one-line title derived from the User Story>" \
     --label "<labels from step 5>" \
     --body-file <temp-file with the body from step 4>
   ```

   Capture the resulting issue URL. **Verification gate:**
   immediately query the issue back via gh-agentic to confirm it
   landed:

   ```bash
   gh agentic status requirement <N> --raw
   ```

   The query must return the issue with the expected stage
   (`backlog` or `draft`) and matching title. If the query fails
   or returns inconsistent data, raise `ISSUE_CREATION_FAILED`
   (`ERROR`).

7. **Revision loop (only if step 5 returned "Revise").** Ask the
   human what to change via `prompt-user`. Adjust the relevant
   section of the drafted body (User Story, Business Need, Success
   Criteria, or Notes) based on the answer. Loop back to step 5
   with the revised body. Cap at 5 iterations; on the 5th, raise
   `REVISION_LOOP_LIMIT` (`WARN`) and surface the current draft
   plus a recommendation to either accept-as-draft or cancel.

8. **Hand off.** Surface a short message to the human containing:

   - The issue number and URL.
   - The labels that were applied.
   - The recommended next phase: *"When you're ready to scope this
     into Feature(s), run Feature Scoping (Stage 2)."*
   - Note that an `## Acceptance Criteria` section was deliberately
     not included in this issue — that's the Feature Scoping
     phase's deliverable, not this one's.

   Then emit the session exit block per the framework's
   `emits-exit-block: true` contract.

## Verification

Run the framework checks against this skill:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/requirements-session/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/requirements-session/SKILL.md
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
  the `GROUND_TRUTH` entry for `requirements-session` (e.g.
  *"capture a new requirement"* triggers; *"check the build"*
  doesn't).

## Error Handling

- `USER_CANCELLED` from step 5 (human chose Cancel) → severity
  `WARN`. End the skill cleanly with a brief no-requirement-captured
  message. Not an error of this skill — the human deliberately
  declined.
- `ISSUE_CREATION_FAILED` from step 6 (gh CLI failed, or
  verification query returned inconsistent data) → severity
  `ERROR`; propagate. Surface the gh stderr to the human;
  recommend re-running the skill once the underlying issue is
  resolved.
- `DUPLICATE_DETECTED` from step 3 (the human chose to treat the
  new input as a refinement of an existing requirement) → not an
  error; redirect: hand off with a pointer to the existing issue
  and recommend running the agent's update-issue flow on the
  existing requirement instead. Currently no update-issue skill
  exists; surface as a manual action for the human.
- `REVISION_LOOP_LIMIT` from step 7 (5 iterations elapsed) →
  severity `WARN`; surface the current draft, recommend accept-as-
  draft or cancel, end the skill.
- All other errors: propagate (default).
