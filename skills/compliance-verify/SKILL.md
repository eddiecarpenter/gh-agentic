---
name: compliance-verify
description: Verifies that the implementation on a feature branch satisfies all acceptance criteria from the Feature issue. Evaluates each AC against the actual diff, posts a structured verdict, and either applies `compliance-verified` (all ACs pass — workflow then opens the PR) or posts a `<!-- compliance-feedback:v1 -->` comment and swaps `in-verification` back to `in-development` (triggering a new dev session). Use when GitHub Actions fires on a feature issue labelled `in-verification`. Headless only.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/step-skip-rule.md
  - skills/gh-agentic/SKILL.md
  - skills/post-issue-comment/SKILL.md
emits-exit-block: true
exit-hands-to: "automation: on PASS — workflow opens PR and applies in-review; on FAIL — workflow swapped in-verification→in-development, triggering a new dev session with the compliance feedback comment as context"
---

# Compliance Verify

## Goal

After the dev session has committed its work, evaluate the feature
branch implementation against every acceptance criterion (AC) in the
Feature issue. Post a structured compliance report as a comment on
the Feature issue. Then:

- **All ACs pass** — apply the `compliance-verified` label. The
  surrounding workflow opens the PR and transitions the Feature to
  `in-review`. A human then reviews the PR.
- **Any AC fails** — post the compliance-feedback comment (with the
  `<!-- compliance-feedback:v1 -->` marker) and swap the Feature
  from `in-verification` → `in-development` using `PIPELINE_PAT`
  (so the label event triggers a new dev session). The dev session
  will read the feedback comment and use it as additional context.

The skill is headless — invoked by workflow automation when the
`in-verification` label is applied.

## Output Artefacts

- **A compliance report comment** on the Feature issue, always —
  whether PASS or FAIL. Subject begins with `<!-- compliance-report:v1 -->`.
  Contains the per-AC verdict table, evidence, and the overall
  verdict.
- **On PASS only:** the `compliance-verified` label applied to the
  Feature issue (via `PIPELINE_PAT`). The workflow's post-recipe
  step checks for this label and opens the PR.
- **On FAIL only:** a compliance-feedback comment with marker
  `<!-- compliance-feedback:v1 -->` and the label transition
  `in-verification` → `in-development` (via `PIPELINE_PAT`). This
  triggers a new dev-session run.

A return value at exit:
```
{ repo: <string>, feature: <int>, branch: <string>,
  acs_total: <int>, acs_passed: <int>, acs_failed: <int>,
  verdict: "PASS" | "FAIL" | "ESCALATED" | "BLOCKED",
  exit_state: "pass" | "fail" | "no-acs" | "cycle-cap" | "blocked" }
```

`exit_state`:
- `pass` — all ACs evaluated and all passed.
- `fail` — one or more ACs failed; feedback comment posted;
  `in-development` label applied.
- `no-acs` — Feature issue has no `## Acceptance Criteria` section.
  Cannot evaluate. Treated as `pass` with a warning so the Feature
  is not stuck forever by a missing AC block; the PR is opened and
  a warning is included in the report.
- `cycle-cap` — the dev-session ↔ compliance-verify feedback loop has
  reached 3 iterations without all ACs passing. Escalation comment
  posted; `needs-human-review` applied; `in-verification` removed.
  No further automated cycling. Human intervention required.
- `blocked` — the skill could not complete verification (environment
  error, branch missing, etc.); exits with an error; Feature stays
  at `in-verification` for human intervention.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy for
  `INVALID_VERIFY_STATE`, `BRANCH_MISSING`, `REPORT_FAILED`.
- `skills/definitions/verification-procedure.md` — evidence must be
  grounded: cite specific files, functions, test names, and commit
  SHAs; no fabrication.
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement.

## Dependencies

- `skills/gh-agentic/SKILL.md` — used in step 2 to read the Feature.
- `skills/post-issue-comment/SKILL.md` — used in step 8 to post the
  compliance report and (on FAIL) in step 11 to post the feedback
  comment.

---

## Steps

The **step-skip rule** applies throughout.

**Resolving the active repo.** Resolve once via:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

and reuse as `<active-repo>`.

---

### Section A — Setup

1. **Announce the session.** Print the banner verbatim before any
   tool call:

   ```
   ==========================================================
   === Compliance Verify — Started                            ===
   ==========================================================
   ```

   Resolve `<active-repo>` per the rule above.

2. **Read the Feature.** Query the full state:

   ```bash
   gh agentic status feature <N> --raw
   ```

   Capture title, body, labels, branch metadata. Hold as `<feature>`.

   On non-zero exit → raise `INVALID_VERIFY_STATE` (`ERROR`).

3. **Validate state.** Inspect `<feature>.labels`:

   - MUST contain `feature` and `in-verification`.
   - MUST NOT contain `in-review`, `compliance-verified`, or `done`.
     If present → raise `INVALID_VERIFY_STATE`. The Feature is
     already past compliance verification; this run is a no-op or a
     workflow bug.

3b. **Check the feedback cycle count.** Count how many
    `<!-- compliance-feedback:v1 -->` comments already exist on the
    Feature issue:

    ```bash
    gh issue view <N> --repo <active-repo> --json comments \
      --jq '[.comments[] | select(.body | startswith("<!-- compliance-feedback:v1 -->"))] | length'
    ```

    Hold as `<feedback-count>`.

    **Cap:** if `<feedback-count>` ≥ 3, the dev-session and
    compliance-verify cycle has exceeded its permitted retries. Do
    NOT continue verification. Instead:

    a. Post an escalation comment via `post-issue-comment`:

       ```markdown
       <!-- compliance-escalation:v1 -->

       # Compliance Verification — Cycle Cap Reached

       Feature #<N> has failed compliance verification **<feedback-count> times**
       and has reached the maximum automated retry limit (3 cycles).

       Automated cycling has been halted. Human intervention is required to
       diagnose why the acceptance criteria cannot be satisfied automatically.

       ## Failed ACs (from last feedback comment)

       See the most recent `<!-- compliance-feedback:v1 -->` comment above for
       the specific ACs that are not passing.

       ## Suggested actions

       - Review the acceptance criteria — are they testable and unambiguous?
       - Review the last compliance report for evidence of what the agent did implement.
       - Consider splitting the AC, relaxing the phrasing, or implementing the fix manually.
       - Once resolved, manually apply `in-development` to restart the pipeline,
         or apply `compliance-verified` if the implementation is already acceptable.
       ```

    b. Apply `needs-human-review` label and remove `in-verification`:

       ```bash
       gh issue edit <N> --repo <active-repo> \
         --add-label "needs-human-review" \
         --remove-label "in-verification"
       ```

    c. Set `exit_state = "cycle-cap"` and emit **Output E** (step 14).
       Terminate immediately — do not proceed to step 4.

4. **Check out the feature branch.**

   ```bash
   git fetch origin
   BRANCH="feature/<N>-<slug>"   # resolved from remote listing
   git checkout -B "$BRANCH" "origin/$BRANCH"
   ```

   On failure → raise `BRANCH_MISSING` (`ERROR`).

5. **Read the design plan.** Query the Feature's comments for the
   one starting with `<!-- design-plan:v1 -->`:

   ```bash
   gh issue view <N> --repo <active-repo> --json comments \
     --jq '[.comments[] | select(.body | startswith("<!-- design-plan:v1 -->"))]
            | .[0].body'
   ```

   Hold as `<design-plan>`. If missing, log a warning and proceed
   without it — the ACs in the Feature body are the primary source
   of truth.

6. **Get the implementation diff.** Produce the unified diff between
   the feature branch and `origin/main`:

   ```bash
   git diff origin/main...HEAD --stat
   git diff origin/main...HEAD
   ```

   Hold `<diff-stat>` and `<diff>`. If `<diff>` is empty (no commits
   beyond main), the feature has no implementation — raise
   `INVALID_VERIFY_STATE` (`ERROR`). Nothing to verify.

7. **Extract acceptance criteria.** Parse the Feature issue body for
   the `## Acceptance Criteria` section. Treat each bullet or
   numbered item starting with `AC-` (or unlabelled bullets if the
   AC block exists but items are not prefixed) as one criterion.

   Hold as `<acs>` — ordered list of `{ index, text }`.

   If `<acs>` is empty (no `## Acceptance Criteria` section found)
   → set `exit_state = "no-acs"` and proceed to step 8 with an
   empty evaluation table (see step 8's `no-acs` handling).

---

### Section B — Evaluation

8. **Evaluate each AC.** For each criterion in `<acs>`, independently
   assess whether the implementation satisfies it:

   **Method:**
   - Read the AC text carefully.
   - Search the diff for code changes relevant to the AC's stated
     outcome. Look for: new functions, modified logic, new tests,
     changed configuration.
   - Search the test suite for tests that assert the AC's observable
     behaviour (not just "tests exist" but "tests assert this
     specific outcome").
   - Assign a verdict: `PASS`, `PARTIAL`, or `FAIL`.

   **Verdict definitions:**
   - `PASS` — the AC is fully satisfied: the implementation change
     is present AND at least one test asserts the stated outcome.
   - `PARTIAL` — the implementation change appears to be there but
     test coverage is missing or the implementation only covers
     part of the AC's stated condition.
   - `FAIL` — no implementation change relevant to the AC, or the
     change exists but is incorrect, or a test for the AC fails.

   **Evidence requirement (verification-procedure rule):** For each
   verdict, record specific evidence: file path(s), function/struct
   name(s), test name(s), line ranges. No generic statements like
   "tests exist" — cite the actual test name and what it asserts.

   Hold as `<evaluations>` — list of
   `{ ac_index, ac_text, verdict, evidence }`.

   **`no-acs` case:** If step 7 found no ACs, produce a single
   synthetic evaluation:
   ```
   { ac_index: "—", ac_text: "No acceptance criteria defined",
     verdict: "PASS (no-acs warning)",
     evidence: "Feature issue body has no ## Acceptance Criteria section. Compliance verification is advisory only." }
   ```
   Proceed to the report step.

---

### Section C — Report and transition

9. **Compute the overall verdict.**

   - All verdicts are `PASS` (or `PASS (no-acs warning)`) →
     overall verdict = `PASS`.
   - Any verdict is `PARTIAL` or `FAIL` → overall verdict = `FAIL`.

   Hold as `<verdict>`.

10. **Post the compliance report comment.** Always, regardless of
    verdict. Use `post-issue-comment`:

    ```
    post-issue-comment(
      repo=<active-repo>,
      issue=<N>,
      body=<report>
    )
    ```

    Report format:

    ```markdown
    <!-- compliance-report:v1 -->

    # Compliance Verification Report — Feature #<N>

    **Branch:** `feature/<N>-<slug>`
    **Verdict:** PASS ✅  |  FAIL ❌  (use one)

    ## Acceptance Criteria Results

    | AC | Status | Evidence |
    |---|---|---|
    | AC-1 | ✅ PASS | `pkg/foo.go:42` — `funcBar` implements the stated behaviour; `TestFuncBar` asserts the outcome |
    | AC-2 | ❌ FAIL | No implementation found for X; no test asserts Y |
    | AC-3 | ⚠️ PARTIAL | Implementation in `pkg/baz.go` covers the success path; error path is unhandled; no test for error case |

    ## Required Fixes
    <!-- only present when verdict is FAIL -->

    For each failed or partial AC, describe concisely what the dev
    session needs to add or fix. Be specific: file, function,
    behaviour.

    - **AC-2:** Add `funcX` in `pkg/foo.go` that handles Y. Add
      `TestFuncX_HandlesY` asserting [observable outcome].
    - **AC-3:** Extend `funcBaz` to handle the error path; add
      `TestFuncBaz_ErrorCase`.
    ```

    Verify the comment was posted:
    ```bash
    gh issue view <N> --repo <active-repo> --json comments \
      --jq '[.comments[] | select(.body | startswith("<!-- compliance-report:v1 -->"))] | length'
    ```
    Should be ≥ 1. On failure → raise `REPORT_FAILED` (`ERROR`).

11. **On PASS — apply `compliance-verified`.**

    ```bash
    gh issue edit <N> --repo <active-repo> \
      --add-label "compliance-verified"
    ```

    Use `GH_TOKEN` from the environment (the workflow sets this to
    `PIPELINE_PAT` — required so the label event can trigger the
    `pr-review-session` workflow if needed). Verify the label is
    present after the edit.

    The surrounding workflow's `Open PR if compliance-verified` step
    checks for this label and opens the PR.

    Emit **Output A** (step 14) and exit.

12. **On FAIL — post the feedback comment.**

    Post a second comment containing the actionable feedback:

    ```markdown
    <!-- compliance-feedback:v1 -->

    # Compliance Feedback — Feature #<N>

    The compliance verification run found the following ACs not
    fully satisfied. The dev session will re-enter with this
    feedback as context.

    ## ACs Requiring Work

    - **AC-<idx>:** <one-sentence summary of what's missing>
      - Evidence: <specific file/function/test reference>
      - Fix needed: <concrete description>

    <!-- repeated for each FAIL or PARTIAL AC -->

    The feature branch already contains the work for passing ACs.
    The dev session should implement ONLY the fixes listed above —
    do not re-implement passing criteria.
    ```

    Verify posted:
    ```bash
    gh issue view <N> --repo <active-repo> --json comments \
      --jq '[.comments[] | select(.body | startswith("<!-- compliance-feedback:v1 -->"))] | length'
    ```

13. **On FAIL — swap `in-verification` → `in-development`.**

    ```bash
    gh issue edit <N> --repo <active-repo> \
      --remove-label "in-verification" \
      --add-label "in-development"
    ```

    This MUST use `PIPELINE_PAT` (set as `GH_TOKEN` by the workflow)
    so the `in-development` label event triggers the dev-session
    workflow. `github.token` events cannot trigger other workflow
    runs (GitHub platform restriction).

    Verify the label transition:
    ```bash
    gh issue view <N> --repo <active-repo> --json labels \
      --jq '[.labels[].name]'
    ```
    Must contain `in-development`; must not contain `in-verification`.

---

### Section D — Closeout

14. **Emit the exit block.** Match the actual outcome:

    **Output A — PASS:**
    ```
    === Compliance Verify — PASS ===

    Feature #<N> satisfies all <M> acceptance criteria.

    Results:
      - <M> of <M> ACs passed
      - compliance-verified label applied
      - Report posted on issue #<N>

    Next: workflow opens PR and applies in-review
    ```

    **Output B — FAIL:**
    ```
    === Compliance Verify — FAIL ===

    Feature #<N> does not yet satisfy all acceptance criteria.

    Results:
      - <P> of <M> ACs passed
      - <F> ACs failed or partial: AC-<x>, AC-<y>, ...
      - Feedback comment posted on issue #<N>
      - Label swapped: in-verification → in-development

    Next: dev session will re-enter with compliance feedback as context
    ```

    **Output C — No ACs (advisory PASS):**
    ```
    === Compliance Verify — PASS (no-acs warning) ===

    Feature #<N> has no ## Acceptance Criteria section.
    Compliance verification was advisory only.

    Results:
      - compliance-verified label applied (no-acs path)
      - Report posted with advisory warning on issue #<N>

    Next: workflow opens PR and applies in-review
    Warning: add an ## Acceptance Criteria section to future Features.
    ```

    **Output D — Blocked:**
    ```
    === Compliance Verify — Blocked ===

    Verification could not complete.

    Error: <BRANCH_MISSING | REPORT_FAILED | INVALID_VERIFY_STATE>
    Reason: <specific diagnosis>

    Feature #<N> stays at in-verification. Human intervention needed.
    ```

    **Output E — Cycle Cap:**
    ```
    === Compliance Verify — Cycle Cap Reached ===

    Feature #<N> has failed compliance verification <feedback-count> times.
    Automated retry limit (3 cycles) exceeded.

    Results:
      - <feedback-count> compliance-feedback comments found on issue #<N>
      - Escalation comment posted on issue #<N>
      - Label applied: needs-human-review
      - Label removed: in-verification

    Next: human must intervene — review ACs, last compliance report, and
    last feedback comment, then manually reset the pipeline or close the Feature.
    ```

15. **Terminate the session.** Per `emits-exit-block: true`, invoke
    the host runtime's session-close API if available; otherwise
    halt.

## Error Handling

- `INVALID_VERIFY_STATE` from steps 2–3 (Feature missing, wrong
  labels, empty diff) → severity `ERROR`; propagate. The pipeline
  invoked this skill against a Feature not ready for verification.
- `BRANCH_MISSING` from step 4 (no remote feature branch) → severity
  `ERROR`; propagate. The feature branch must exist before
  `in-verification` is applied.
- `REPORT_FAILED` from step 10 (compliance report comment did not
  post) → severity `ERROR`; propagate. Without the report, the
  state is ambiguous — do not apply `compliance-verified` or swap
  labels when the audit trail cannot be written.
- `CYCLE_CAP_EXCEEDED` from step 3b (feedback-count ≥ 3) → severity
  `WARN` (not `ERROR`); this is an expected pipeline state, not an
  environment failure. The escalation comment and label transition
  are applied and the skill exits cleanly with `exit_state =
  "cycle-cap"`. Do not propagate as an uncaught exception.
- All other errors: propagate (default).
