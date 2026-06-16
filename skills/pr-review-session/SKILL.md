---
name: pr-review-session
description: Processes PR reviewer feedback by reading unaddressed reviewer comments on a Feature's PR — both inline review comments and general PR comments — and acting on each: pure questions get a direct reply, change requests get implemented (with the commit-discipline applied) and committed in a single batched commit then pushed. Approvals do NOT trigger this skill. Use when GitHub Actions fires on a review submitted with state changes_requested or commented, on a new issue comment on a PR, or on a new pull-request review comment. Use even when the caller doesn't say "pr review" — phrases like "address PR feedback", "respond to the review on #42", "implement the requested changes" should trigger this skill.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/cp-execution-context.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/step-skip-rule.md
  - skills/definitions/commit-discipline.md
  - skills/gh-agentic/SKILL.md
  - skills/post-issue-comment/SKILL.md
emits-exit-block: true
exit-hands-to: "human: reviewer revisits the PR; on approval the workflow's pr-merged handler closes the Feature and (if all sibling Features are closed) the parent Requirement"
---

# PR Review Session

## Goal

When a reviewer leaves comments or requests changes on the Feature's
PR, walk every unaddressed comment and act on it: answer questions
in-thread; implement change requests as a single batched commit on
the feature branch following the universal commit discipline. Push
the commit if any code changed; do nothing further on the
PR-lifecycle side. Approvals never reach this skill — the workflow's
pr-merged handler is the path forward from there.

The skill is headless. It is invoked by GitHub Actions on three
event types:

- **PR review submitted** with `state ∈ {changes_requested, commented}`.
  (Reviews with `state ∈ {approved, dismissed}` never trigger.)
- **New issue comment** posted on a PR (top-level conversation).
- **New pull-request review comment** (inline comment on a file/line).

Each invocation may receive one event but must process all
unaddressed comments visible at the time — re-fires and
out-of-order webhook delivery don't break correctness.

## Output Artefacts

- **Reply comments** to each addressed reviewer comment. For inline
  review comments, replied via the inline thread. For top-level PR
  comments and review-summary comments, replied as new top-level PR
  comments. Each reply addresses one source comment.
- **At most one batched commit** on the feature branch when one or
  more change requests were implemented. Subject:
  ```
  fix: address review feedback — PR #<N>
  ```
  Body lists the addressed comments (one bullet per comment, with
  excerpt or comment id). `Reuse:` trailer per the commit discipline.
  Pushed to origin.
- **No label transitions.** This skill does not touch lifecycle
  labels. The Feature stays at `in-review` (applied earlier by the
  pipeline workflow). The PR is the source of truth for review
  state.
- **No re-request review.** If the reviewer has not approved, that's
  the reviewer's call to revisit — the agent does not nudge them.

A return value at exit:
```
{ repo: <string>, pr: <int>, branch: <string>,
  questions_answered: <int>, change_requests_implemented: <int>,
  commit_sha: <string|null>,
  exit_state: "completed" | "no-op" | "blocked" }
```

`exit_state`:
- `completed` — at least one comment was addressed (reply or commit).
- `no-op` — entered with zero unaddressed non-self comments. Nothing
  to do; clean exit.
- `blocked` — change request could not be implemented (test/build
  refused to pass, ambiguous instruction, missing dependency); reply
  posted explaining the block; PR stays at `changes_requested`.

The skill's four valid terminal outputs:

**A. Replies + commit.** Mix of questions answered and change
requests implemented. Single batched commit pushed.

**B. Replies only.** All addressed comments were questions; no code
changed; no commit.

**C. No-op.** All visible non-self comments already have agent
replies; nothing unaddressed. Clean exit.

**D. Blocked.** A change request could not be implemented. Surface
the diagnosis as a reply on that comment; mark exit_state blocked;
exit. Other comments in the same batch that succeeded are still
addressed.

## Definitions

- `skills/definitions/cp-execution-context.md` — the control-plane
  execution context (#873): cwd is the project code
  (`$AGENTIC_PROJECT_DIR`) where the PR branch lives; the PR and its
  review threads live in the project repo, while docs and the rulebook
  live on the control plane (`$AGENTIC_CP_ROOT`). Read docs from
  `$AGENTIC_CP_ROOT/docs`; commit and push fixes only inside the
  project; never write to the control-plane checkout.
- `skills/definitions/error-handling.md` — severity taxonomy for
  `INVALID_REVIEW_STATE`, `BRANCH_MISSING`, `REPLY_FAILED`,
  `COMMIT_DISCIPLINE_FAILED`, `CHANGE_BLOCKED`.
- `skills/definitions/verification-procedure.md` — change-pinning
  rule (every reply and commit is verified by re-querying GitHub /
  re-reading git log).
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement.
- `skills/definitions/commit-discipline.md` — universal discipline for
  the batched commit (reuse outcome, tests pass, build pass, format,
  push). The `<context tag>` for this skill is `— PR #<N>`.

## Dependencies

- `skills/gh-agentic/SKILL.md` — used in step 1 to resolve the active
  repo (the PR is on the active repo's feature branch).
- `skills/post-issue-comment/SKILL.md` — used in step 7 (replies to
  questions) and step 12 (in-line replies on inline comments) and
  step 11 (the diagnostic reply on blocked change-requests).

## Steps

The **step-skip rule** applies. The natural-skip carve-out: comments
that have a self-reply (idempotency) are skipped per their state, not
silently — see step 5 for the discovery rule.

**Resolving the active repo.** Resolve once via:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

and reuse as `<active-repo>`.

**Identifying self.** Resolve the agent's own GitHub identity once at
session start so self-comments are filtered out and self-replies are
detectable:

```bash
gh api user --jq .login
```

Hold as `<self>`. The result is the App's bot login (e.g.
`agentic-bot[bot]` or similar). All comparisons against comment
authors use this login.

---

### Section A — Setup

1. **Announce the session.** Print the banner verbatim before any
   tool call:

   ```
   ==========================================================
   === PR Review Session — Started                            ===
   ==========================================================
   ```

   Resolve `<active-repo>` and `<self>` per the rules above.

2. **Read the PR.** The triggering event provides the PR number `<N>`.
   Query the PR's full state:

   ```bash
   gh pr view <N> --repo <active-repo> --json \
     number,title,state,baseRefName,headRefName,labels
   ```

   - PR state MUST be `OPEN`. If not → raise `INVALID_REVIEW_STATE`
     (`ERROR`).
   - PR head branch MUST start with `feature/`. If not → raise
     `INVALID_REVIEW_STATE`.

   Capture `<head-branch>` (the feature branch).

3. **Find the Feature issue.** Read the PR body and extract the
   referenced Feature issue number from the `Closes #<feature>`
   line:

   ```bash
   PR_BODY=$(gh pr view <N> --repo <active-repo> --json body --jq .body)
   FEATURE_NUMBER=$(echo "$PR_BODY" | grep -oE 'Closes #[0-9]+' | head -1 | grep -oE '[0-9]+')
   ```

   - `FEATURE_NUMBER` empty → raise `INVALID_REVIEW_STATE` (`ERROR`);
     exit. The PR is not properly linked to a Feature, which means
     the agent cannot trace replies back to the design rationale or
     the Feature's acceptance criteria. PR review on an unlinked PR
     is out of scope for this skill.
   - Otherwise → hold as `<feature>` for use in reply formatting and
     for cross-checking against the design rationale when answering
     questions.

   The Feature issue is metadata for this skill — referenced in
   reply comments and used to retrieve the design rationale (the
   `<!-- design-plan:v1 -->` comment), not mutated.

4. **Check out the head branch.** Mirror `dev-session` step 6:

   ```bash
   git fetch origin
   git checkout -B "<head-branch>" "origin/<head-branch>"
   ```

   On failure → raise `BRANCH_MISSING` (`ERROR`). The PR exists,
   the branch should too.

---

### Section B — Comment discovery

5. **Gather all comments on the PR.** Two sources:

   - **Issue (top-level) comments:**
     ```bash
     gh pr view <N> --repo <active-repo> --json comments \
       --jq '.comments'
     ```
   - **Review comments (inline + summary):**
     ```bash
     gh api repos/<active-repo>/pulls/<N>/reviews
     gh api repos/<active-repo>/pulls/<N>/comments
     ```

   Merge into a single ordered list `<all-comments>` keyed by
   creation timestamp. Each entry: `{ id, type, author, body,
   in_reply_to_id, path, line, created_at }`.

   - `type ∈ {issue_comment, review_summary, review_comment_inline}`.
   - `in_reply_to_id` is set for inline replies; null for top-level.

6. **Filter to unaddressed reviewer comments.** Apply two filters:

   - **Drop self-authored comments** (`author.login == <self>`).
     The remaining list is reviewer-authored.
   - **Drop comments that already have a self-reply.** A
     comment `C` is "addressed" if there exists a comment `R` with
     `R.author.login == <self>` and either:
     - `R.in_reply_to_id == C.id` (inline thread reply), OR
     - `R.created_at > C.created_at` AND `R` is a top-level comment
       AND `R.body` references `C` (e.g. cites the comment id, file
       path, or quotes the text). The agent uses judgement here —
       the goal is to avoid double-processing, not to be paranoid.

   The remaining list is `<unaddressed>`.

   - **`<unaddressed>` empty** → emit Output C and exit cleanly.
     Do NOT proceed to Section C.

7. **Categorise each unaddressed comment** as one of:

   - **`question`** — the comment asks something, expresses
     uncertainty, or seeks clarification. Examples: "Why this
     approach?", "Is this thread-safe?", "What about edge case X?"
   - **`change_request`** — the comment asks for a code change.
     Examples: "Rename this", "Add error handling for X", "This
     should return Y instead of Z", an inline comment on a code
     line saying "use X here".

   Categorisation is the agent's call. When ambiguous, lean
   `change_request` — implementing what the reviewer asked is the
   default behaviour (the reviewer is the boss).

   Hold the categorised list as `<work>`.

---

### Section C — Reply to questions

8. **For each `question` in `<work>`:** generate a reply that
   directly answers what was asked. Anchor the answer in the code
   (cite specific files/functions) or the rationale comment on the
   Feature issue (cite by issue and quote-excerpt). No fabrication —
   if the answer requires information you don't have, say so.

9. **Post the reply.** Use the appropriate channel:

   - **Inline review comment** (has `in_reply_to_id` is null but
     `path` and `line` are set) → reply via the inline thread:
     ```bash
     gh api repos/<active-repo>/pulls/<N>/comments \
       -F body=<reply-body> \
       -F in_reply_to=<comment.id> \
       --method POST
     ```
   - **Issue comment / review summary** → reply as a top-level
     PR comment:
     ```
     post-issue-comment(repo=<active-repo>, issue=<N>,
                        body=<reply-body-with-quote-of-source>)
     ```
     Begin the body with `> @<reviewer-login> said:` followed by
     the quoted excerpt so the thread context is preserved.

   On failure → raise `REPLY_FAILED` (`WARN`); continue with the
   next comment. A failed reply does not block the rest of the
   batch.

   Verify each reply by re-querying the comments and checking the
   new reply is present.

---

### Section D — Implement change requests

10. **Plan the change set.** For each `change_request` in `<work>`:
    - Read the comment in full, including code context (`path`,
      `line` for inline comments).
    - Decide what code change satisfies the request.
    - If genuinely unimplementable as written (the request
      contradicts a constraint, missing context, ambiguous to the
      point of multiple plausible interpretations) → mark this
      comment for blocked-reply (step 11) instead of implementing.

    The output of this step is two lists:
    - `<to-implement>` — change requests with a concrete plan.
    - `<to-block>` — change requests that need diagnosis-reply.

11. **Diagnostic reply for blocked items.** For each entry in
    `<to-block>`, post a reply explaining why the change cannot be
    made headlessly. Use the same channel rules as step 9. The
    reply MUST:

    - Acknowledge the request.
    - State specifically what blocks implementation (constraint,
      missing detail, ambiguity, environment issue).
    - Suggest the smallest piece of clarification that would unblock.
    - Not implement *part* of the change. Either the change is
      implementable or it isn't — partial implementations of unclear
      requests cause more confusion.

    Mark the session as `exit_state: blocked` if `<to-block>` is
    non-empty. Output D applies. Continue with `<to-implement>`
    even when some are blocked — addressed comments are still
    addressed.

12. **Implement `<to-implement>` and commit.** Apply
    `skills/definitions/commit-discipline.md` — reuse outcome,
    tests pass, build pass, batched commit, push.

    - Make the code changes for every entry in `<to-implement>`.
    - Add/update tests as needed (per the discipline).
    - Run tests; fix until passing.
    - Run the build; fix until passing.
    - Commit with the canonical format:

      ```
      fix: address review feedback — PR #<N>

      - <comment-id or excerpt>: <one-line description of the change>
      - <comment-id or excerpt>: <one-line description>
      ...

      Reuse: <outcome>
      ```

    - Push.

    On any commit-discipline failure (test stuck, build stuck,
    unable to satisfy the request even though it seemed plannable)
    → raise `COMMIT_DISCIPLINE_FAILED` (`ERROR`). Move all
    `<to-implement>` entries into `<to-block>`, post diagnostic
    replies (step 11), exit with Output D.

13. **Reply to each implemented change request.** For each entry
    in `<to-implement>` that was successfully committed, post a
    reply citing the commit:

    ```
    Addressed in <commit-sha-short>: <one-line description>.
    ```

    Inline comments → inline reply. Top-level → top-level reply
    quoting the source. Same channel rules as step 9.

    Failure of any individual reply → `REPLY_FAILED` (`WARN`);
    continue.

---

### Section E — Closeout

14. **Emit the exit block.** Match the actual outcome:

    **Output A — Replies + commit:**
    ```
    === PR Review Session — Completed ===

    Addressed:
      - Questions answered: <count>
      - Change requests implemented: <count> (in commit <sha>)
      - Pushed to origin/<head-branch>

    Blocked: none

    Next: human reviewer revisits the PR
    ```

    **Output B — Replies only:**
    ```
    === PR Review Session — Completed (no code changes) ===

    Addressed:
      - Questions answered: <count>
      - Change requests implemented: 0

    Blocked: none

    Next: human reviewer revisits the PR
    ```

    **Output C — No-op:**
    ```
    === PR Review Session — No-op ===

    Entered with zero unaddressed reviewer comments. All visible
    comments already have agent replies.

    Next: nothing
    ```

    **Output D — Blocked:**
    ```
    === PR Review Session — Blocked ===

    Addressed before block:
      - Questions answered: <count>
      - Change requests implemented: <count> (commit <sha or "none">)

    Blocked on:
      - <comment-id or excerpt>: <reason>
      - ...

    Diagnostic replies posted on the PR for each blocked item.

    Next: human reviewer addresses the blocked items (clarifies
          the request, takes over the implementation, or dismisses)
    ```

15. **Terminate the session.** Per `emits-exit-block: true`, invoke
    the host runtime's session-close API if available; otherwise
    halt.

## Error Handling

- `INVALID_REVIEW_STATE` from steps 2–3 (PR not open, head branch
  not a feature branch, no Closes line) → severity `ERROR`;
  propagate. Caller / workflow bug — invoked the skill against a
  PR that should not be reviewed by the agent.
- `BRANCH_MISSING` from step 4 (head branch missing locally /
  remotely) → severity `ERROR`; propagate.
- `REPLY_FAILED` from step 9, 11, or 13 → severity `WARN`; continue.
  A failed reply on one comment does not block processing of others.
  Surface in the exit block which replies failed.
- `COMMIT_DISCIPLINE_FAILED` from step 12 → severity `ERROR`. Per
  step 12's rule, all unimplemented `<to-implement>` items roll
  into `<to-block>` and get diagnostic replies. Exit with Output D
  (blocked).
- `CHANGE_BLOCKED` is not a separately-raised code — it is the
  state captured in the `<to-block>` list and surfaced in the exit
  block. The skill never exits in error solely because a change
  was un-implementable; it exits in `blocked` state having posted
  diagnostic replies.
- All other errors: propagate (default).
