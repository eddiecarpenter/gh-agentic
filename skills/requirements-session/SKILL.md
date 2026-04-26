---
name: requirements-session
description: Captures a new business need as a Requirement GitHub issue through a conversational, agent-led interview — listening, challenging vague or solution-framed input, and confirming the result with the human before creating the issue. Use when a human wants to record a new business need, idea, or enhancement request as a Requirement. Use even when the user doesn't explicitly say "requirements session" — phrases like "I want to capture a new requirement", "let's record this idea", "add a new business need", "capture a feature request" should trigger this skill.
triggers: human-interactive
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/step-skip-rule.md
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

The skill has two valid terminal outputs — exactly one fires per
invocation:

**A. New requirement captured (the primary path).**

- A new GitHub issue created in the active repo, with:
  - `requirement` label applied.
  - `backlog` label applied (when the human confirms the capture is
    final), or `draft` label applied (when the human flags the
    requirement as still-being-refined).
  - Body matching the structure described in step 5 (User Story,
    Business Need, Success Criteria, Notes).
- A confirmation-gate transcript showing the human's explicit
  acceptance of the captured content before the issue was created.
  Recorded in conversation, not as a file.
- A hand-off message naming the issue number, URL, and the next
  recommended phase (Feature Scoping).

**B. Hand-off to an existing requirement (step 2 pick-existing exit).**

- No new issue is created.
- A hand-off message naming the existing issue number and URL the
  human picked, and a note that the framework does not yet have an
  in-place update-requirement flow — the human edits the issue
  directly on GitHub.

No file artefacts in either case. The GitHub issue (new or existing)
is the durable record.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy applied
  to `USER_CANCELLED`, `ISSUE_CREATION_FAILED`, `REVISION_LOOP_LIMIT`.
- `skills/definitions/verification-procedure.md` — the change-pinning
  rule the Verification section follows (issue presence is verified
  by querying GitHub, not by the agent's claim).
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement
  rule that prevents silent skipping of any step in the Steps section.

## Dependencies

- `skills/prompt-user/SKILL.md` — used in step 3 (interview),
  step 6 (confirmation gate), and step 8 (handle-revision loop) to
  ask the human structured questions.
- `skills/gh-agentic/SKILL.md` — used in step 2 to query existing
  requirements (so the agent can spot duplicates) and in step 7 to
  query the resulting issue back via `gh agentic status requirement
  <N> --raw` for the verification check.

## Steps

The **step-skip rule** from `skills/definitions/step-skip-rule.md`
applies to every step below: no step may be skipped without the agent
first emitting, in its response stream, which step is being skipped
and the concrete reason why.

**Resolving the active repo.** Anywhere a step refers to the
"active repo", resolve it once in step 1 and reuse the value. The
canonical command is:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

This returns `owner/repo` (e.g. `eddiecarpenter/gh-agentic`) for the
git remote of the current working directory — the same repo `gh
issue create` would target by default, but explicitly captured so
later steps can pass it as `--repo <active-repo>` rather than
relying on implicit CWD detection (which is fragile in worktrees).
Never hard-code the repo.

**Required labels precondition.** Step 7 applies the labels
`requirement` and either `backlog` or `draft`. All three must exist
on the active repo before issue creation; if any is missing,
`gh issue create --label <missing>` will fail. The agent does not
create labels — that is repo-setup work owned by the framework's
bootstrap (`gh agentic check` / `repair`). If a label is missing at
step 7, propagate the failure via `ISSUE_CREATION_FAILED` and
recommend the human run `gh agentic repair` before re-trying.

**Conditional-step carve-out.** Steps marked as conditional in their
opening line ("only if …") are not skipped when their precondition
does not fire — they are simply *not applicable*. The skip rule does
not apply, and the agent need not emit a skip-justification. In this
skill: step 2's Stage B (only if Stage A returned "Continue with
existing") and step 8 (only if step 6 returned "Revise") are
conditional. Every other step is mandatory.

1. **Announce the session.** The first user-visible output of this
   skill is a mode-entry banner so the human knows Requirements
   collection has started. Print the banner verbatim before any
   tool call or other output:

   ```
   ==========================================================
   === Requirements Session — Started                     ===
   ==========================================================
   You are now in Requirements collection mode.
   I'll listen, challenge vague or solution-framed input,
   and capture each business need as a GitHub Requirement
   issue once you confirm.
   ==========================================================
   ```

   Then, immediately after the banner, resolve the active repo per
   the "Resolving the active repo" rule above and hold it in working
   memory as `<active-repo>` for use in step 7. If the command fails
   (e.g. CWD is not a git repo with a GitHub remote), surface the
   error and end the skill cleanly — the rest of the flow assumes a
   resolvable repo.

   **Foundation SA precondition (hard gate).** Verify that
   `docs/ARCHITECTURE.md` exists in the active repo:

   ```bash
   test -f docs/ARCHITECTURE.md
   ```

   If the file does not exist, the skill MUST hard-fail. Surface to
   the human:

   ```
   Cannot capture a Requirement: docs/ARCHITECTURE.md is missing.

   Foundation Solution Architecture is a precondition for capturing
   Requirements — without an architectural baseline, Requirements
   have nothing to anchor against and tend to be vague or
   contradictory. Run the solution-architecture skill to bootstrap
   docs/ARCHITECTURE.md, then re-invoke this skill.
   ```

   Then exit cleanly. Do not proceed to step 2.

   This is a single point of enforcement at the entry of the pipeline
   — downstream phases (scoping, feature design, etc.) trust this
   precondition rather than re-checking it. The check exists here to
   prevent any Requirement entering the pipeline without a Foundation
   SA in place.

   This step runs once per session. On subsequent capture loops
   (when the human chose "capture another" in step 9), skip the
   banner, the repo resolution, *and* the precondition check — all
   three are already done — and resume from step 2.

2. **Survey existing requirements and offer to continue one.** Query
   the active repo's open requirements for context (used in step 4
   for informational duplicate articulation, and below to offer
   pick-up of in-flight backlog items):

   ```bash
   gh agentic status requirements --raw
   ```

   The output is TSV with header row and these columns:
   `number  stage  title  blocked_by  owning_repo`. There is no
   summary or body field in this output — only the title is safe to
   render to the human. Hold the parsed rows in working memory; do
   not surface the full list automatically.

   **Conditional continuation prompt.** Filter the parsed list to
   entries whose stage is `backlog`. Then branch:

   - **No backlog entries** → continue silently to step 3. Do not
     prompt; there is nothing to pick up.
   - **One or more backlog entries** → run a two-stage prompt
     (necessary because `prompt-user` supports at most ~4 options
     per call, and the backlog list is unbounded).

     **Stage A — kind-of-work prompt.** Invoke `prompt-user`:

     ```
     prompt-user(
       question: "Capture a new requirement, or continue with an existing one?",
       header: "Existing backlog detected",
       options: [
         {label: "New requirement",
          description: "Start a fresh interview."},
         {label: "Continue with existing requirement",
          description: "Pick from the backlog list."}
       ]
     )
     ```

     Branch on the answer:
     - **New requirement** → continue to step 3.
     - **Continue with existing requirement** → proceed to Stage B.

     **Stage B — pick-which (plain conversation, no `prompt-user`).**
     Only if Stage A returned "Continue with existing requirement".
     Render the backlog list to the human as a plain conversation
     message — one line per entry, **number and title only**. Do not
     fabricate summaries: the TSV from step 2 has no summary field,
     so anything beyond `number` and `title` would be invention.

     ```
     Backlog requirements:

       #<N>  <title>
       #<M>  <title>
       …

     Enter the number of the requirement you'd like to continue
     with (or reply "cancel" to exit).
     ```

     Do NOT use `prompt-user` here — the list is unbounded and the
     `prompt-user` 4-option limit makes it the wrong primitive. A
     free-text conversational reply is the right interaction.

     Wait for the human's reply. The valid set is exactly the issue
     numbers rendered above (the *backlog-filtered* list, not the
     full open-requirements list). Parse the reply:
     - **A number matching a rendered `#<N>`** → surface the issue's
       URL to the human (`https://github.com/<active-repo>/issues/<N>`)
       and end the skill cleanly with a note that the framework
       does not yet have an in-place update-requirement flow; the
       human can edit the issue directly on GitHub. Do not continue
       to step 3.
     - **A number not in the rendered list** → tell the human the
       number isn't in the backlog list and re-render. Counts as
       one attempt against the cap below.
     - **"cancel" (or any clear cancellation)** → raise
       `USER_CANCELLED` (`WARN`), end cleanly.
     - **Anything else** → ask once for clarification ("Please reply
       with a requirement number from the list above, or 'cancel'.")
       and wait again. Counts as one attempt.

     **Attempt cap.** Stop after 3 invalid replies (any combination
     of out-of-range numbers and unparseable input). On the 3rd, end
     the skill cleanly with `USER_CANCELLED` (`WARN`) and a one-line
     note that the human can re-invoke the skill once they've
     decided.

   Re-run this entire step at the start of every capture loop so the
   new capture sees any just-created issue and the human gets a fresh
   choice each time.

3. **Open the interview.** Ask the human for their initial
   description of the business need:

   > "Tell me about the business need — what's the problem, who has
   > it, and what would 'solved' look like?"

   Wait for the human's reply. Their response is the raw input the
   rest of this skill structures.

   **Agent-facing contract for the rest of this skill:** the agent's
   job is friction, not transcription. Helpful-mode is off. The
   default is to challenge, ask back, and refuse to proceed on vague
   or solution-framed input — not to draft a tidy issue from
   whatever was said. A short, sharp interview that produces one
   well-formed requirement beats a long, agreeable one that produces
   a vague issue.

4. **Challenge vagueness or solution-framing.** Read the human's
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
   feature).

   **Quoted-evidence exit gate.** Before exiting this loop, the agent
   must emit — in its response stream — a short block of the form:

   ```
   Role (quoted from human): "<exact words from the human>"
   Outcome (quoted from human): "<exact words from the human>"
   ```

   If the agent cannot fill either quote with words the human
   actually said (paraphrase or invention is not sufficient), the
   loop is not complete: ask another question. This gate is
   non-negotiable — it is the mechanism that prevents the agent
   from rationalising a half-formed input as "good enough".

   **Duplicate articulation (mandatory).** Before drafting the body
   in step 5, the agent must restate — in its response stream —
   which existing open requirements (from step 2's list) it
   considered as potential duplicates and why each is or is not a
   duplicate. Format:

   ```
   Duplicate check:
     - #<N> "<title>" — <not-a-dup | overlap, treat as refinement | overlap, distinct>
     - #<N> "<title>" — <…>
     - (none) if step 2 returned no open requirements
   ```

   If any item is flagged as `overlap`, the next action depends on
   what the human told the agent in step 2:

   - **The human picked "New requirement" in step 2** (or step 2's
     prompt did not fire because no backlog entries existed) → treat
     overlap findings as **informational only**. Surface the
     candidates to the human in plain conversation ("I noticed this
     looks similar to #N — flagging it for your awareness; continuing
     with a new capture as you requested.") and continue to step 5.
     Do not re-prompt; the human already chose the path.
   - **Step 2's prompt fired but the agent had no list to consider**
     (edge case — should not happen) → treat as informational, same
     as above.

   In other words: step 2 is the human's explicit pick-up gate; step
   4's articulation is the agent's audit, not a second veto. The
   agent's job here is transparency about what it considered, not to
   second-guess the human's already-stated intent. No `prompt-user`
   interaction is required in this step — overlap candidates are
   surfaced as plain conversation only.

5. **Draft the issue body.** Build the body from the captured
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

   **Anti-fabrication clause (non-negotiable).** Every clause in the
   User Story (`role`, `capability`, `outcome`), every sentence of
   the Business Need, and every bullet of Success Criteria must be
   traceable to something the human actually said in the interview.
   Paraphrasing for grammar is fine; *inventing* a role, outcome,
   constraint, or success criterion the human did not state is not.
   If the agent finds itself filling a gap with a plausible-sounding
   guess, it must stop and ask the human instead. This is the
   single most likely failure mode of a requirements interview —
   the agent helpfully fabricates content. Don't.

   **Notes-section discipline.** The Notes section is for genuine
   side-context (related links, constraints the human raised,
   explicit open questions). It is NOT a dumping ground for content
   the agent could not push back on. If a sentence belongs in
   Business Need or Success Criteria but the agent is tempted to
   stash it in Notes to avoid challenging the human, that is a
   signal to go back to step 4 and challenge.

   Hold the drafted body in working memory; do NOT create the
   issue yet.

6. **Confirmation gate.** First, render the complete drafted body
   from step 5 to the human as plain conversation output —
   verbatim, no omissions, in a fenced markdown block prefaced by
   "Here is the issue body that would be filed:". This is a
   conversation message, not a `prompt-user` argument: `prompt-user`
   options are short labels and cannot carry a full issue body. The
   human reads the body in the conversation, then answers the
   structured question.

   Then, immediately after the body, invoke `prompt-user`:

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

   Branch on the answer:
   - **Yes (backlog)** → continue to step 7 with labels
     `requirement,backlog`.
   - **Yes, but mark as draft** → continue to step 7 with labels
     `requirement,draft`.
   - **Revise** → continue to step 8.
   - **Cancel** → raise `USER_CANCELLED` (`WARN`), surface a brief
     "no requirement captured" message, end the skill cleanly.

7. **Create the issue.** Invoke (with `<active-repo>` resolved per the
   "Resolving the active repo" rule above, and labels per the
   "Required labels precondition"):

   ```bash
   gh issue create \
     --repo "<active-repo>" \
     --title "<title>" \
     --label "<labels from step 6>" \
     --body-file <temp-file with the body from step 5>
   ```

   **Title derivation.** `<title>` is a noun-phrase summary of the
   outcome the human wants, ≤70 characters, traceable to the User
   Story's `So that <outcome>` line. It is NOT the User Story
   verbatim, NOT a verb-imperative ("Add CSV export"), and NOT
   prefixed with `feat:` / `fix:` etc. Examples of well-formed
   titles: *"Backlog visibility for product managers"*,
   *"Reproducible local dev environment"*. If no concise noun
   phrase fits within 70 chars, ask the human directly for a title
   rather than truncating.

   **Verification gate.** Immediately after `gh issue create`
   succeeds, capture the new issue number `<N>` and URL from the
   command's output, then query the issue back:

   ```bash
   gh agentic status requirement <N> --raw
   ```

   The output is `key: value` lines (one per line) followed by
   `---` and the issue body. The agent must check three lines from
   the metadata block:

   - `number: <N>` matches the just-created issue.
   - `stage: <expected>` where `<expected>` is `backlog` or `draft`
     per step 6's branch.
   - `title: <expected>` matches the title passed to
     `gh issue create`.

   If any of those don't match, or the command fails, raise
   `ISSUE_CREATION_FAILED` (`ERROR`) with the gh-agentic stderr
   surfaced to the human and a recommendation to run
   `gh agentic repair`.

8. **Revision loop (only if step 6 returned "Revise").** Ask the
   human which section to revise via `prompt-user`:

   ```
   prompt-user(
     question: "Which section would you like to revise?",
     header: "Revise requirement",
     options: [
       {label: "User Story",
        description: "Role / capability / outcome lines."},
       {label: "Business Need",
        description: "The 2–4 sentence problem statement."},
       {label: "Success Criteria",
        description: "The observable-outcome bullets."},
       {label: "Notes",
        description: "Side-context, links, open questions."}
     ]
   )
   ```

   After the section is picked, ask the human what to change in
   that section via plain conversation (free-text — `prompt-user`
   options can't carry edit-instructions). Adjust only the picked
   section; leave every other section verbatim.

   **Per-revision diff (mandatory).** Before looping back to step 6,
   the agent must emit — in its response stream — a per-field diff
   showing only the changed fields:

   ```
   Revision N — changes:
     <field name>:
       was: <previous value>
       now: <new value>
     <unchanged fields are omitted>
   ```

   This prevents silent rewriting of fields the human had already
   accepted. If a field appears in the diff, the agent intentionally
   changed it; if it doesn't, the agent commits to leaving it as-is.

   Cap at 5 iterations; on the 5th, raise `REVISION_LOOP_LIMIT`
   (`WARN`) and surface the current draft plus a recommendation to
   either accept-as-draft or cancel.

9. **Continuation prompt.** After the issue is verified, ask the
   human as an open question via `prompt-user` whether they want to
   capture another requirement or end the session:

   ```
   prompt-user(
     question: "Capture another requirement, or end the session?",
     header: "Requirement #<N> created — what next?",
     options: [
       {label: "Capture another requirement",
        description: "Loop back and start a fresh interview."},
       {label: "End the session (run /clear)",
        description: "Hand off and exit Requirements mode."}
     ]
   )
   ```

   Branch on the answer:
   - **Capture another** → loop back to step 2 (re-run the survey
     query so the new capture sees the just-created issue). The
     banner and active-repo resolution from step 1 are not redone —
     they are already in working memory.
   - **End the session** → continue to step 10.

   Note: when the loop fires, the requirement just created in this
   session will reappear in step 2's backlog list (it now has stage
   `backlog`). That is expected and harmless — the human can pick it
   to exit, pick another, or pick "New requirement" to continue
   capturing fresh ones.

   Never assume the human is done after one capture — always ask.

10. **Hand off.** Surface a short message to the human containing:

   - The issue number(s) and URL(s) created in this session.
   - The labels that were applied.
   - The recommended next phase: *"When you're ready to scope these
     into Feature(s), run Feature Scoping (Stage 2)."*
   - Note that an `## Acceptance Criteria` section was deliberately
     not included in these issues — that's the Feature Scoping
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

**Behavioural-ceiling note.** Passing the framework checks is a
necessary but not sufficient signal of quality. The checks verify
structure (frontmatter, sections, trigger phrasings) and reference
integrity — they do NOT verify that the agent honours the
anti-fabrication clause, quoted-evidence gate, or per-revision
diff at runtime. Those disciplines are self-policed by the agent.
Treat "passes verification" as "the skill is well-formed", not "the
skill produces correct requirements".

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

- `USER_CANCELLED` (raised from any user-cancel choice — step 2
  Stage B "cancel" reply, step 2 Stage B attempt-cap exhaustion,
  step 6 "Cancel" option) → severity `WARN`. End the skill cleanly
  with a brief no-requirement-captured message. Not an error of
  this skill — the human deliberately declined or ran out of valid
  attempts.
- `ISSUE_CREATION_FAILED` from step 7 (gh CLI failed, label
  missing, or verification query returned inconsistent metadata) →
  severity `ERROR`; propagate. Surface the gh / gh-agentic stderr
  to the human; recommend running `gh agentic repair` and
  re-invoking the skill once the underlying issue is resolved.
- `REVISION_LOOP_LIMIT` from step 8 (5 iterations elapsed) →
  severity `WARN`; surface the current draft, recommend
  accept-as-draft or cancel, end the skill.
- All other errors: propagate (default).
