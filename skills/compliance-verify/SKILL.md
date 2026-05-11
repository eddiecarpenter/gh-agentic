---
name: compliance-verify
description: Evaluates a Feature's implementation against its acceptance criteria, test quality, and language standards by reading the diff and Feature issue in isolation — no dev-session context — and produces a structured findings report posted as an issue comment. Applies compliance-verified on all-pass; applies development-in-progress on any fail or partial; escalates and halts on oscillation or 10-cycle cap. Use when the compliance-verify pipeline stage fires on a Feature labelled in-verification — the diff is evaluated independently so findings are uncontaminated by implementation-session bias.
category: Session
triggers: automated
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/verification-procedure.md
  - skills/apply-label/SKILL.md
  - skills/post-issue-comment/SKILL.md
emits-exit-block: true
exit-hands-to: "pipeline: in-review (all-pass) | pipeline: in-development (fail/partial, no oscillation, cycle < 10) | human: escalation (oscillation or 10-cycle cap)"
---

# Compliance Verify

## Goal

Take a Feature in `in-verification` state and evaluate its implementation
against the Feature's acceptance criteria, test coverage, test existence,
and language-standard best practices. Post the findings as a structured
report on the Feature issue. Route to `compliance-verified` (all pass),
`development-in-progress` (any fail or partial, no oscillation, cycle < 10),
or escalation halt (oscillation detected or 10-cycle cap reached).

The skill operates in isolation — it reads only the diff, the Feature issue,
and prior report comments. No dev-session conversation history is consulted.
This isolation is intentional: findings must be uncontaminated by
implementation-session context so the verification is independent.

## Output Artefacts

- **One findings report comment** on the Feature issue, starting with
  `<!-- compliance-verify-report:v1 -->`. Every run posts a report —
  even re-runs and escalation cycles — so the issue thread is a complete
  durable audit trail.
- **Label transitions** — one of:
  - All-pass: `compliance-verified` added, `in-verification` removed.
  - Any-fail: `development-in-progress` added, `in-verification` removed.
  - Oscillation escalation: `in-verification` removed; no new label added.
  - 10-cycle cap: `in-verification` removed; no new label added.

A return value at exit:
```
{ repo: <string>, feature: <int>,
  cycle: <int>, verdict: "pass" | "fail" | "escalated",
  findings_total: <int>, findings_fail: <int>, findings_partial: <int> }
```

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy applied to
  `INVALID_VERIFY_STATE`, `DIFF_EMPTY`, `COVERAGE_TOOL_UNAVAILABLE`,
  `OSCILLATION_DETECTED`, `CYCLE_CAP_REACHED`.
- `skills/definitions/verification-procedure.md` — change-pinning and
  re-run verification rules applied throughout.

## Dependencies

- `skills/apply-label/SKILL.md` — all label transitions in Section C.
- `skills/post-issue-comment/SKILL.md` — posting the findings report and
  escalation comments.

## Steps

**Resolving the active repo.** Resolve once at the start:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

Hold as `<active-repo>`.

---

### Section A — Setup

1. **Announce the session.** Print the banner verbatim before any tool call:

   ```
   ==========================================================
   === Compliance Verify — Started                        ===
   ==========================================================
   ```

2. **Read the Feature.** Query the issue:

   ```bash
   gh issue view <N> --repo <active-repo> \
     --json number,title,body,labels
   ```

   Hold as `<feature>`.

   On non-zero exit → raise `INVALID_VERIFY_STATE` (`ERROR`).

3. **Validate state.** Inspect `<feature>.labels`:

   - MUST contain `feature` and `in-verification`.
   - If `in-verification` is absent → raise `INVALID_VERIFY_STATE`
     (`ERROR`). The workflow should not have triggered without it.
   - If `compliance-verified` or `in-review` or `done` is present →
     raise `INVALID_VERIFY_STATE` (`ERROR`). The Feature has moved
     past verification; this run should not proceed.

4. **Extract the AC list.** Parse `<feature>.body` for the
   `## Acceptance Criteria` section. Extract every line matching
   `**Given** ... **When** ... **Then** ...` as an AC item indexed
   AC-1, AC-2, ..., AC-N in document order.

   If no `## Acceptance Criteria` section exists → log the omission,
   skip the AC evaluation in Section B step 7, and continue.
   Coverage, test-existence, and standards checks still run.

5. **Read cycle history.** Query prior compliance-verify-report comments:

   ```bash
   gh issue view <N> --repo <active-repo> --json comments \
     --jq '[.comments[] | select(.body | startswith("<!-- compliance-verify-report:v1 -->"))]'
   ```

   Count as `<cycle-count>`. Extract the most recent report's
   findings as `<prior-findings>` — a list of
   `{ key: "<ac-index|coverage|test-existence|standards-<rule>>",
     verdict: "pass" | "fail" | "partial" }`.

   If no prior reports → `<cycle-count> = 0`, `<prior-findings> = []`.

6. **Compute the diff.**

   ```bash
   git diff origin/main..HEAD
   ```

   Hold as `<diff>`.

   If `<diff>` is empty → post a findings comment with a single
   finding: `{ key: "diff-empty", verdict: "fail",
   detail: "No commits found on this branch ahead of main — nothing to verify." }`.
   Then raise `DIFF_EMPTY` (`WARN`) and exit with Output D (blocked).
   Do NOT apply any label transition — a human must investigate why
   the branch has no commits.

---

### Section B — Evaluation

The evaluation produces `<findings>` — an ordered list of finding objects:
```
{ key: <string>, verdict: "pass" | "fail" | "partial",
  detail: <string>, location: <file:line> | null }
```

7. **AC evaluation.** For each AC-K in the AC list:

   - Search `<diff>` for code changes that implement the observable
     behaviour described in the AC's **Then** clause.
   - A **pass** requires a positive code reference (file + line or
     hunk in the diff) that directly addresses the Then clause.
   - A **partial** applies when code changes exist but the evidence
     is incomplete (e.g. the happy path is covered but an error branch
     is missing, or a test asserts the behaviour but no production code
     implements it).
   - A **fail** applies when no code evidence for the Then clause
     is found in the diff.

   Record each finding as:
   ```
   { key: "AC-K", verdict: "pass|fail|partial",
     detail: "<one sentence>",
     location: "<file>:<hunk-start>" | null }
   ```

   Append all AC findings to `<findings>`.

8. **Test-existence check.** Inspect `<diff>` for hunks that add or
   modify production logic (non-test files: files not matching
   `*_test.go` or `testdata/`).

   For each such hunk, check whether the diff also contains
   corresponding test additions or modifications in a `*_test.go`
   file within the same package.

   - If all changed production logic has accompanying test changes →
     record `{ key: "test-existence", verdict: "pass", detail: "...", location: null }`.
   - If any changed production logic has NO accompanying test → record
     `{ key: "test-existence", verdict: "fail",
       detail: "New/changed logic in <file> has no corresponding test changes.",
       location: "<file>:<hunk-start>" }`.

   This check is distinct from the coverage check — it looks at
   whether tests were written at all, not whether coverage thresholds
   were met. Append to `<findings>`.

   **Skill-only files (no Go production code):** if `<diff>` contains
   only Markdown skill/recipe files and no `.go` files, skip this
   check and record:
   ```
   { key: "test-existence", verdict: "pass",
     detail: "No Go production code in diff — check not applicable.",
     location: null }
   ```

9. **Coverage check.** Run the Go test suite with coverage:

   ```bash
   go test -coverprofile=coverage.out ./... 2>&1
   go tool cover -func=coverage.out | grep "^total:"
   ```

   Parse the total coverage percentage from the `total:` line.

   - If total ≥ 80% → record `{ key: "coverage", verdict: "pass",
     detail: "Coverage: <pct>% (threshold: 80%)", location: null }`.
   - If total < 80% → record `{ key: "coverage", verdict: "fail",
     detail: "Coverage: <pct>% is below 80% threshold.", location: null }`.

   If `go` is unavailable or the command fails with a tool-not-found
   error → record `{ key: "coverage", verdict: "fail",
   detail: "go tool unavailable — coverage check could not run." }` and
   raise `COVERAGE_TOOL_UNAVAILABLE` (`WARN`). Log the reason; continue.

   **Skill-only files (no Go code):** if `<diff>` contains only
   Markdown skill/recipe files and no `.go` files, skip this check
   and record:
   ```
   { key: "coverage", verdict: "pass",
     detail: "No Go production code in diff — coverage check not applicable.",
     location: null }
   ```

   Append to `<findings>`.

10. **Standards check.** Read the language standard file:

    ```bash
    cat standards/go.md
    ```

    Locate the `## Coding Standards` section. For each named rule
    in that section (context propagation, nil safety, panics,
    interface design, struct initialisation, constants, time, financial
    values, sensitive data, concurrency):

    - Inspect `<diff>` for violations of that specific rule.
    - If no violation found → record `{ key: "standards-<rule>",
      verdict: "pass", detail: "No violations found.", location: null }`.
    - If a violation is found → record `{ key: "standards-<rule>",
      verdict: "fail",
      detail: "Violation of rule: <rule-name>. <one sentence describing the specific violation>.",
      location: "<file>:<line>" }`.

    **Skill-only files (no Go code):** if `<diff>` contains only
    Markdown skill/recipe files and no `.go` files, record a single
    finding: `{ key: "standards", verdict: "pass",
    detail: "No Go production code in diff — standards check not applicable.",
    location: null }` and skip the per-rule walk.

    Append all standards findings to `<findings>`.

---

### Section C — Report, oscillation, and exit

11. **Compute summary counts.**

    ```
    <findings-fail>    = count(f in <findings> where f.verdict == "fail")
    <findings-partial> = count(f in <findings> where f.verdict == "partial")
    <findings-total>   = len(<findings>)
    <any-nonpass>      = (<findings-fail> + <findings-partial>) > 0
    ```

12. **Oscillation detection.** If `<prior-findings>` is non-empty
    and `<any-nonpass>` is true:

    Compute `<oscillating>` = findings in `<findings>` where
    `(f.key, f.verdict)` also appears in `<prior-findings>` (the
    most recent prior cycle). The oscillation key is `(key, verdict)`;
    code references (location, detail) are NOT part of the key.

    If `<oscillating>` is non-empty → set `<oscillation-detected> = true`.
    Otherwise → `<oscillation-detected> = false`.

13. **Cycle-cap check.** If `<cycle-count>` ≥ 10 and `<any-nonpass>`
    is true and `<oscillation-detected>` is false →
    set `<cap-reached> = true`. Otherwise `<cap-reached> = false`.

14. **Build the findings report.** Format the structured report:

    ```markdown
    <!-- compliance-verify-report:v1 -->

    # Compliance Verify — Cycle <cycle-count + 1>

    **Feature:** #<N> — <title>
    **Verdict:** <ALL PASS | FAIL (<findings-fail> fail, <findings-partial> partial)>
    **Date:** <ISO-8601>

    ## Findings

    | # | Key | Verdict | Detail | Location |
    |---|-----|---------|--------|----------|
    | 1 | AC-1 | ✅ pass / ❌ fail / ⚠️ partial | <detail> | <location or —> |
    | 2 | AC-2 | ...                            | ...      | ...              |
    | … | test-existence | ...               | ...      | ...              |
    | … | coverage       | ...               | ...      | ...              |
    | … | standards-<rule> | ...             | ...      | ...              |

    ## Summary
    Total findings: <N> | Pass: <P> | Fail: <findings-fail> | Partial: <findings-partial>
    ```

15. **Build escalation comment (if needed).** When oscillation
    detected or cap reached, format:

    ```markdown
    <!-- compliance-verify-report:v1 -->

    # Compliance Verify — Escalation (Cycle <cycle-count + 1>)

    **Feature:** #<N>
    **Reason:** <"Oscillation detected" | "10-cycle cap reached">

    ## Oscillating violations
    [Only for oscillation case: table of (key, verdict) present in both
     current and prior cycle with cycle numbers]

    ## Action required
    Human review required. The automated verify loop cannot resolve
    these findings. Options:
    - Edit the Feature acceptance criteria if requirements changed
    - Implement the missing behaviour and re-trigger manually
    - Close the Feature if no longer relevant
    ```

16. **Post the report.**

    ```
    post-issue-comment(repo=<active-repo>, issue=<N>,
                       body=<findings report or escalation comment>)
    ```

    On failure → surface the error as `WARN` and continue to label
    transitions — the label transition is more critical than the comment.

17. **Exit routing.** Branch on the outcome:

    **A — Oscillation detected:**
    ```
    apply-label(repo=<active-repo>, issue=<N>,
                add=[], remove=["in-verification"])
    ```
    Emit Output D (escalation). Halt.

    **B — Cycle cap reached:**
    ```
    apply-label(repo=<active-repo>, issue=<N>,
                add=[], remove=["in-verification"])
    ```
    Emit Output D (escalation). Halt.

    **C — All findings pass:**
    ```
    apply-label(repo=<active-repo>, issue=<N>,
                add=["compliance-verified"], remove=["in-verification"])
    ```
    Emit Output A (completed). Halt.

    **D — Any fail or partial (no oscillation, cycle < 10):**
    ```
    apply-label(repo=<active-repo>, issue=<N>,
                add=["development-in-progress"], remove=["in-verification"])
    ```
    Emit Output B (fail cycle). Halt.

18. **Emit the exit block.** Match the actual outcome:

    **Output A — All pass:**
    ```
    === Compliance Verify — Completed ===

    Feature #<N> — cycle <cycle-count+1>
    Verdict: ALL PASS

    Findings: <total> | Pass: <pass> | Fail: 0 | Partial: 0

    Applied: compliance-verified
    Removed: in-verification

    Next: workflow opens the PR and applies in-review
    ```

    **Output B — Fail cycle (no oscillation, cycle < 10):**
    ```
    === Compliance Verify — Fail Cycle ===

    Feature #<N> — cycle <cycle-count+1>
    Verdict: FAIL

    Findings: <total> | Pass: <pass> | Fail: <fail> | Partial: <partial>

    Failed findings:
      - <key>: <detail>
      ...

    Applied: development-in-progress
    Removed: in-verification

    Next: dev-session implements the missing behaviour; re-triggers compliance-verify
    ```

    **Output C — Re-run no-op:**
    (Not applicable — `in-verification` guard in step 3 prevents this.)

    **Output D — Blocked (escalation or diff-empty):**
    ```
    === Compliance Verify — Escalated ===

    Feature #<N> — cycle <cycle-count+1>
    Reason: <oscillation detected | 10-cycle cap reached | diff empty>

    <oscillating violations or cap context>

    Removed: in-verification
    NOT applied: development-in-progress

    Next: human reviews the Feature and decides the path forward
    ```

19. **Terminate the session.** Per `emits-exit-block: true`, halt.

## Error Handling

- `INVALID_VERIFY_STATE` from steps 2–3 (Feature missing, wrong
  labels, or already past verification) → severity `ERROR`; propagate.
  Workflow bug — should not have triggered without `in-verification`.
- `DIFF_EMPTY` from step 6 (no commits ahead of main) → severity
  `WARN`; propagate. Post a finding comment before raising so the
  issue thread records the state.
- `COVERAGE_TOOL_UNAVAILABLE` from step 9 (go binary missing) →
  severity `WARN`; record a fail finding for coverage and continue.
  The finding will appear in the report; if it is the only failure it
  will trigger a fail cycle, and the operator must install the toolchain.
- `OSCILLATION_DETECTED` from step 12 → severity `WARN`; the skill
  handles this via the escalation path in step 17A.
- `CYCLE_CAP_REACHED` from step 13 → severity `WARN`; the skill
  handles this via the escalation path in step 17B.
- All other errors: propagate (default). The label transition in step
  17 should be attempted as a best-effort on every exit path — if the
  label edit fails, surface the failure clearly so a human can manually
  remove `in-verification`.
