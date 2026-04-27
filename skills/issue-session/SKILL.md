---
name: issue-session
description: Handles a GitHub issue that has been assigned to the agent (label `assigned-to-agent`) — either by answering it as a question (reply + close), implementing it as a small fix (branch + commit + push + PR), or redirecting it as out-of-scope (apply `needs-scoping` and surface a pointer to the pipeline). Headless only. Use when GitHub Actions fires on an `assigned-to-agent` label being applied to a non-pipeline issue (i.e. an issue that is not a Requirement, Feature, or Task). Use even when the caller doesn't say "issue session" — phrases like "address the assigned issue", "handle issue #42", "respond to the agent-assigned issue" should trigger this skill.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/step-skip-rule.md
  - skills/definitions/commit-discipline.md
  - skills/gh-agentic/SKILL.md
  - skills/apply-label/SKILL.md
  - skills/post-issue-comment/SKILL.md
emits-exit-block: true
exit-hands-to: "human: review the answer, the merged PR, or the redirect comment; respond on the issue thread if more is needed"
---

# Issue Session

## Goal

Take a non-pipeline GitHub issue that has been labelled
`assigned-to-agent` and act on it appropriately:

- **Question** — the issue is asking something. The agent posts a
  reply with the answer, closes the issue, removes the label.
- **Fix** — the issue describes a concrete, small bug or change
  the agent can plausibly handle without going through the full
  Requirements → Scoping → Design pipeline. The agent creates an
  `issue/<N>-<slug>` branch, implements the change under the
  universal commit discipline, opens a PR, and removes the label.
  The PR's `Closes #<N>` body line auto-closes the issue on merge.
- **Out-of-scope** — the issue is too large, too ambiguous, or
  too design-heavy to handle headlessly. The agent posts a
  redirect comment, applies `needs-scoping`, and removes
  `assigned-to-agent`. The human captures it as a Requirement and
  runs it through the pipeline.

The skill is invoked headlessly by GitHub Actions when the
`assigned-to-agent` label is applied. It is NEVER invoked on
Requirements, Features, or Tasks — those have their own
pipeline-driven sessions.

## Output Artefacts

The skill has four valid terminal outputs — exactly one fires:

**A. Answered.** The issue was a question. A reply was posted; the
issue is closed; `assigned-to-agent` was removed. No code change.

**B. Fixed.** The issue described a small change. A branch
`issue/<N>-<slug>` was created, the change was implemented under
the commit discipline (one commit, pushed), and a PR was opened
with body `Closes #<N>`. The label was removed from the issue
(but not the PR — the PR carries no label by default). The issue
remains open until the PR merges.

**C. Redirected.** The issue is out-of-scope for headless work. A
comment was posted explaining why and pointing to the pipeline.
`needs-scoping` was applied; `assigned-to-agent` was removed. The
issue stays open.

**D. Blocked.** The agent attempted to fix but could not finish
(test/build refused to pass, dependency missing, scope grew
mid-implementation). The agent posts a comment explaining the
block, removes `assigned-to-agent`, applies `needs-scoping`. Any
partial work on the local branch is NOT pushed; nothing the human
needs to clean up. The issue stays open.

A return value at exit:
```
{ repo: <string>, issue: <int>,
  classification: "question" | "fix" | "out-of-scope",
  branch: <string|null>, pr: <int|null>,
  exit_state: "answered" | "fixed" | "redirected" | "blocked" }
```

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy for
  `INVALID_ISSUE_STATE`, `BRANCH_OPERATION_FAILED`,
  `PR_CREATION_FAILED`, `FIX_BLOCKED`.
- `skills/definitions/verification-procedure.md` — change-pinning
  rule (the comment was actually posted; the PR was actually
  opened; the label was actually removed).
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement.
- `skills/definitions/commit-discipline.md` — universal discipline
  for the fix-path commit. The `<context tag>` is `(closes #<N>)`.

## Dependencies

- `skills/gh-agentic/SKILL.md` — used in step 1 to resolve the
  active repo.
- `skills/apply-label/SKILL.md` — used to claim and release the
  `issue-in-progress` beacon (steps 4, 14), to apply
  `needs-scoping` on out-of-scope and blocked paths, and to remove
  `assigned-to-agent` on every exit.
- `skills/post-issue-comment/SKILL.md` — used to post the answer
  (step 7), the redirect comment (step 9), the blocked comment
  (step 13), and the resolution-pointer comment (step 12).

## Steps

The **step-skip rule** applies. Mode-gated carve-out: the
question path skips fix-path steps and vice versa — by design,
not a violation.

**Resolving the active repo.** Resolve once via:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

and reuse as `<active-repo>`.

**Concurrency guard.** Step 4 claims `issue-in-progress` on the
issue. Every exit path (success, blocked, error) MUST best-effort
release it before exit. Same pattern as `feature-design` /
`dev-session`.

**Identifying self.** Resolve the agent's GitHub identity once at
session start (used to filter self-comments when reading prior
context):

```bash
gh api user --jq .login
```

Hold as `<self>`.

---

### Section A — Setup and triage

1. **Announce the session.** Print the banner verbatim before any
   tool call:

   ```
   ==========================================================
   === Issue Session — Started                                ===
   ==========================================================
   ```

   Resolve `<active-repo>` and `<self>` per the rules above. The
   triggering event provides the issue number `<N>`.

2. **Read the issue.** Query its full state:

   ```bash
   gh issue view <N> --repo <active-repo> --json \
     number,title,body,state,labels,comments
   ```

   Capture as `<issue>`.

   - `state == OPEN` MUST be true. If not → raise
     `INVALID_ISSUE_STATE` (`ERROR`); exit. The issue was closed
     between the label-apply event and this skill running.
   - The label set MUST contain `assigned-to-agent`. If not →
     raise `INVALID_ISSUE_STATE`; exit. The label was removed
     before this skill ran.
   - The label set MUST NOT contain any of `requirement`,
     `feature`, `task`. If any are present → raise
     `INVALID_ISSUE_STATE` (`ERROR`); the issue is a pipeline
     artefact and should be handled by its phase skill, not this
     one. The agent should not have been assigned.

3. **Detect re-run state (fail softly).** If `issue-in-progress`
   is already on the issue, another invocation is in flight (or
   crashed). Exit cleanly with a one-line note: "Another issue
   session is active on this issue; this run is a no-op." Do NOT
   remove the label.

4. **Claim the slot.** Apply `issue-in-progress`:

   ```
   apply-label(repo=<active-repo>, issue=<N>,
               add=["issue-in-progress"], remove=[])
   ```

   On failure → raise `INVALID_ISSUE_STATE` (`ERROR`); exit.

5. **Classify the issue.** Read the title, body, and existing
   reviewer/human comments (filter out `<self>`-authored ones —
   if any prior agent run posted notes). Decide one of:

   - **`question`** — the issue asks something. It seeks information,
     explanation, or clarification. No code change required to
     resolve. Examples: "How does X work?", "Why is the build
     failing on Y?", "Where is Z documented?", "What's the
     expected behaviour of W?"
   - **`fix`** — the issue describes a concrete, scoped change
     the agent can plausibly handle without further design. The
     change must be:
     - Localised (a small number of files, a clearly-bounded
       behaviour change).
     - Self-contained (no contract changes; no new external
       dependencies; no API/schema design decisions).
     - Spec-able from the issue body alone (acceptance is
       implicit in the description; no walking-through required).
     Examples: "Fix typo in docs/X.md", "The error message in
     Y.go uses 'recieve' instead of 'receive'", "Add a missing
     nil-check in handleRequest()", "Extend test coverage for the
     foo() failure case".
   - **`out-of-scope`** — the issue is too large, ambiguous, or
     design-heavy. Anything that touches more than a few files,
     changes a public API, introduces a new dependency, or
     requires architectural judgement. Default toward this when
     in doubt. Examples: "Migrate from X to Y", "Add user
     authentication", "Rewrite the renderer", "Improve
     performance" (without specific targets).

   Hold the result as `<class>`. Surface the classification +
   reasoning in the response stream so the run log captures the
   call.

---

### Section B — Question path (only when class == question)

6. **Compose the answer. (question only)** Anchor the answer in
   verifiable facts: cite specific files, functions, lines, or
   prior issue/PR references. No fabrication — if the answer
   requires information you don't have, say so explicitly and
   reclassify as `out-of-scope` (jump to step 9).

7. **Post the answer. (question only)**

   ```
   post-issue-comment(repo=<active-repo>, issue=<N>,
                      body=<answer-content>)
   ```

   Verify the comment exists by re-querying the issue's comments.

8. **Close the issue. (question only)** The agent's answer is the
   resolution; close with a default comment marker:

   ```bash
   gh issue close <N> --repo <active-repo> \
     --comment "Closing — question answered above. Re-open or post a follow-up if more is needed."
   ```

   Then jump to Section E (closeout).

---

### Section C — Out-of-scope path (only when class == out-of-scope)

9. **Compose and post the redirect. (out-of-scope only)** A short
   comment explaining why headless action isn't appropriate and
   pointing the human at the pipeline:

   ```
   This issue is out of scope for a headless agent session. Reason:
   <one-paragraph explanation — what makes it too big / ambiguous /
   design-heavy>.

   To take this through the pipeline:
     1. Capture it as a Requirement (`/requirements-session`).
     2. Scope into Features (`/requirement-scoping`).
     3. The pipeline takes over from there.

   I'll remove `assigned-to-agent` and apply `needs-scoping` so
   it's visible on the triage view. The issue stays open until
   you decide how to proceed.
   ```

   Post via `post-issue-comment`; verify.

10. **Apply `needs-scoping`, remove `assigned-to-agent`. (out-of-scope only)**

    ```
    apply-label(repo=<active-repo>, issue=<N>,
                add=["needs-scoping"],
                remove=["assigned-to-agent"])
    ```

    Then jump to Section E.

---

### Section D — Fix path (only when class == fix)

11. **Create the issue branch. (fix only)** Compute the branch
    name: `issue/<N>-<slug>`, where slug is the lowercase
    issue-title with non-alphanumeric runs collapsed to `-`,
    leading/trailing `-` stripped, capped at 30 chars.

    ```bash
    git fetch origin main
    BRANCH="issue/<N>-<slug>"
    git checkout -B "$BRANCH" origin/main
    ```

    On failure → raise `BRANCH_OPERATION_FAILED` (`ERROR`); exit
    with Output D (blocked). Post a comment per step 13.

12. **Implement and commit. (fix only)** Apply
    `skills/definitions/commit-discipline.md` — reuse outcome,
    tests pass, build pass, single commit, push.

    Commit subject template:
    ```
    fix: <one-line description of the change> (closes #<N>)

    <optional body>

    Reuse: <outcome>
    ```

    On any commit-discipline failure (test stuck, build stuck,
    scope-grows-during-implementation realisation that this is
    actually out-of-scope) → raise `FIX_BLOCKED` (`ERROR`). Move
    to step 13 (blocked-comment + label flip + exit Output D).
    Do NOT push a half-done commit; the local branch is dropped.

    On success → push:
    ```bash
    git push origin "$BRANCH"
    ```

    Then open the PR:
    ```bash
    gh pr create \
      --repo "<active-repo>" \
      --title "fix: <one-line description>" \
      --body "Closes #<N>" \
      --base main \
      --head "$BRANCH"
    ```

    Capture the PR number `<P>`. Verify the PR exists.

    On `gh pr create` failure → raise `PR_CREATION_FAILED`
    (`ERROR`). The branch is pushed; the human can open the PR
    manually. Surface this in the exit block.

    Post a brief resolution comment on the issue:
    ```
    Submitted as PR #<P> on branch `issue/<N>-<slug>`.
    Closing on merge.
    ```

    Then jump to Section E.

13. **Blocked-comment for fix-path failures. (fix only —
    failure branch)** When `BRANCH_OPERATION_FAILED` or
    `FIX_BLOCKED` fires:

    Post a comment explaining the block:
    ```
    I attempted to handle this as a fix but could not complete it.
    Reason: <test/build/dependency/scope reason>.

    Recommended path forward:
      - Treat this as an out-of-scope item: capture as a
        Requirement and run it through the pipeline.
      - Or take it manually if the diagnosis above suggests an
        environment / quick-fix issue I can't see headlessly.

    Removing `assigned-to-agent` and applying `needs-scoping`.
    ```

    Then apply the label flip (same call as step 10), and jump to
    Section E with `exit_state = blocked`.

---

### Section E — Closeout

14. **Release the slot.** Remove `issue-in-progress`:

    ```
    apply-label(repo=<active-repo>, issue=<N>,
                add=[], remove=["issue-in-progress"])
    ```

    Failures here are surfaced as `WARN` and do not block exit.

    For the question path: also ensure `assigned-to-agent` is
    removed. (For fix and out-of-scope it was already removed in
    step 12 / step 10 / step 13.)

    For the question path:
    ```
    apply-label(repo=<active-repo>, issue=<N>,
                add=[], remove=["assigned-to-agent"])
    ```

    Note: the apply-label calls in steps 10 / 13 already remove
    `assigned-to-agent`, so duplicating here is a no-op for those
    paths.

15. **Emit the exit block.** Match the actual outcome:

    **Output A — Answered:**
    ```
    === Issue Session — Answered ===

    Produced:
      - Reply on #<N>
      - Issue closed

    Next: human reviewer reads the answer; reopens or posts a
          follow-up if more is needed.
    ```

    **Output B — Fixed:**
    ```
    === Issue Session — Fixed ===

    Produced:
      - Branch: issue/<N>-<slug>
      - 1 commit, pushed
      - PR #<P> opened (Closes #<N>)
      - Resolution-pointer comment on #<N>

    Next: human reviewer reviews PR #<P>; merge closes #<N>
          automatically.
    ```

    **Output C — Redirected:**
    ```
    === Issue Session — Redirected ===

    Produced:
      - Redirect comment on #<N>
      - needs-scoping applied; assigned-to-agent removed

    Next: human captures #<N> as a Requirement (/requirements-session)
          and runs it through the pipeline.
    ```

    **Output D — Blocked:**
    ```
    === Issue Session — Blocked ===

    Attempted as: fix
    Stopped at: <test/build/dependency/scope reason>

    Produced:
      - Blocked-explanation comment on #<N>
      - needs-scoping applied; assigned-to-agent removed

    Local branch was NOT pushed; nothing to clean up remotely.

    Next: human triages — either re-scope as a Requirement or
          take over the implementation manually.
    ```

16. **Terminate the session.** Per `emits-exit-block: true`,
    invoke the host runtime's session-close API if available;
    otherwise halt.

## Verification

Run the framework checks against this skill:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/issue-session/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/issue-session/SKILL.md
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
  the `GROUND_TRUTH` entry for `issue-session`.

## Error Handling

**Slot-release rule (universal).** Every error path AND every
graceful exit AFTER step 4 (the slot was claimed) MUST attempt to
remove `issue-in-progress` before exit, on a best-effort basis.
If the removal itself fails, surface as a `WARN` and exit anyway.

- `INVALID_ISSUE_STATE` from steps 2–4 (issue closed, label
  missing, issue is a pipeline artefact, slot-claim failed) →
  severity `ERROR`; propagate. Caller / workflow bug — invoked
  on an inappropriate issue.
- `BRANCH_OPERATION_FAILED` from step 11 (branch checkout failed)
  → severity `ERROR`. Treat as `FIX_BLOCKED` for output purposes:
  step 13 posts the blocked-comment, applies `needs-scoping`,
  removes `assigned-to-agent`, exits Output D.
- `PR_CREATION_FAILED` from step 12 (`gh pr create` failed after
  successful push) → severity `ERROR`; propagate. The branch is
  on the remote; the human can open the PR manually. Surface in
  the exit block; mark `exit_state = blocked` even though the
  fix itself landed.
- `FIX_BLOCKED` from step 12 (test stuck, build stuck, scope grew
  mid-fix) → severity `WARN`; propagate. Step 13 handles the
  comment + label flip; exit Output D.
- All other errors: propagate (default).
