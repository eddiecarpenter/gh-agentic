---
name: compliance-verify
description: Verifies that the implementation on a feature branch passes both a static analysis gate (native tools where available plus an AI-driven security and quality review of the diff) and every acceptance criterion from the Feature issue. Posts a structured verdict, and either applies `compliance-verified` (all checks pass — workflow then opens the PR) or posts a `<!-- compliance-feedback:v1 -->` comment and swaps `in-verification` back to `in-development` (triggering a new dev session). SonarQube deep analysis runs post-PR via CI — not during this stage. Use when GitHub Actions fires on a feature issue labelled `in-verification`. Headless only.
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

After the dev session has committed its work, run two independent
verification gates against the feature branch:

1. **Static analysis** — native tooling where available on the runner,
   plus an AI-driven review of the diff covering OWASP-category security
   issues, bug patterns, and code quality. Any BLOCKER or CRITICAL
   finding is a hard failure. SonarQube deep analysis runs post-PR via
   `sonarcloud.yml` — not during this stage.

2. **AC evaluation** — evaluate every acceptance criterion in the
   Feature issue against the implementation diff and test suite.

Post a structured compliance report as a comment on the Feature issue.
Then:

- **All checks pass** — apply the `compliance-verified` label. The
  surrounding workflow opens the PR and transitions the Feature to
  `in-review`. A human then reviews the PR.
- **Any check fails** — post the compliance-feedback comment (with the
  `<!-- compliance-feedback:v1 -->` marker) and swap the Feature
  from `in-verification` → `in-development` using `PIPELINE_PAT`
  (so the label event triggers a new dev session). The dev session
  will read the feedback comment and use it as additional context.

The skill is headless — invoked by workflow automation when the
`in-verification` label is applied.

## Output Artefacts

- **A compliance report comment** on the Feature issue, always —
  whether PASS or FAIL. Begins with `<!-- compliance-report:v1 -->`.
  Contains the static-analysis findings summary, per-AC verdict table,
  evidence, and the overall verdict.
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
  sa_verdict: "PASS" | "WARN" | "FAIL",
  acs_total: <int>, acs_passed: <int>, acs_failed: <int>,
  verdict: "PASS" | "FAIL" | "ESCALATED" | "BLOCKED",
  exit_state: "pass" | "fail" | "no-acs" | "cycle-cap" | "blocked" }
```

`exit_state`:
- `pass` — static analysis passed (or WARN-only) AND all ACs passed.
- `fail` — static analysis found BLOCKER/CRITICAL issues, OR one or
  more ACs failed; feedback comment posted; `in-development` applied.
- `no-acs` — Feature issue has no `## Acceptance Criteria` section.
  Static analysis still runs. If static analysis passes, treated as
  advisory pass; if static analysis fails, treated as `fail`.
- `cycle-cap` — the dev-session ↔ compliance-verify feedback loop has
  reached 3 iterations without all checks passing. Escalation comment
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
- `skills/post-issue-comment/SKILL.md` — used in step 14 to post the
  compliance report and (on FAIL) in step 16 to post the feedback
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
       diagnose why the checks cannot be satisfied automatically.

       ## Last feedback comment

       See the most recent `<!-- compliance-feedback:v1 -->` comment above for
       the specific issues that are not passing.

       ## Suggested actions

       - Review the acceptance criteria — are they testable and unambiguous?
       - Review the last compliance report for static-analysis findings.
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

    c. Set `exit_state = "cycle-cap"` and emit **Output E** (step 19).
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
   → set `exit_state = "no-acs"` and continue. Static analysis in
   Section B still runs in full. The AC evaluation in Section C
   produces a synthetic no-acs entry. The overall verdict in step 13
   is driven entirely by `<sa-verdict>` when no ACs exist.

---

### Section A.0 — Build & Test Gate

The gate runs **before** any other compliance check. It is the
first action after Section A setup completes and the diff +
acceptance-criteria list have been loaded.

The rule lives in `standards/<stack-lowercase>.md` →
`## Verification Gate (build + test)`. Load that section, execute
the listed commands verbatim, in order, from the repo root. The
same commands will have been run by the dev session as its
last step before exit; this is a fresh re-run that re-validates the
pushed branch is consistent with the build and test contract.

**Possible outcomes:**

1. **All commands exit zero** → record `gate=PASS` plus the
   captured command output summary into working memory. Continue
   to Section B. The compliance report's first section reflects
   the PASS.

2. **Any command exits non-zero** → record `gate=FAIL` plus the
   captured failure output. Skip Section B (static analysis) and
   Section C (AC evaluation) entirely — ACs cannot be PASS while
   the build is broken. Jump directly to Section D with
   `<sa-verdict>` and `<ac-verdict>` both set to "skipped — gate
   failed". The overall verdict is FAIL with the gate-failure
   output as the structured violation list.

3. **Toolchain unavailable** (e.g. `go` not on PATH, the standards
   file lists tools the runner does not have) → record
   `gate=BLOCKED`. Do NOT post a FAIL verdict — a BLOCKED verdict
   is qualitatively different. Apply the `needs-human-review`
   label (this is not part of the FAIL cycle; it is a runner
   environment failure the human must resolve). Post a
   `<!-- compliance-blocked:v1 -->` comment surfacing the missing
   tool and the standards-file requirement. Exit with Output D.
   Subsequent runs on the same Feature pick up only when the
   human has installed the toolchain and re-toggled the label.

4. **No standards file** for the active stack (no
   `standards/<stack>.md` exists, or the file has no
   `## Verification Gate` section) → raise
   `STACK_GATE_UNDEFINED` (`ERROR`). The compliance verifier
   cannot enforce a contract the framework has not defined.
   Surface the missing file to the human as a framework-level fix
   needed; exit with Output D.

The gate's outcome is the first line of the compliance report
(see step 14). When the report shows AC PASS but the gate ran
and failed, that is a protocol violation — the gate gates
everything.

**Compliance MUST NOT mark "build and tests pass" as PASS by code
inspection.** If the gate could not run, the verdict for that AC
is BLOCKED, recorded as such, and the cycle does not advance.

---

### Section B — Static Analysis

Perform a code-quality and security gate before AC evaluation. This
section runs entirely against the checked-out feature branch. All
findings are recorded for inclusion in the compliance report (step 14)
and, if the overall verdict is FAIL, the feedback comment (step 16).

**Pre-requisite:** Section A.0 has executed and `gate=PASS`. If
`gate=FAIL`, Section B is skipped — see Section A.0 outcome #2.

**Severity levels used throughout this section:**

| Level | Meaning | Verdict impact |
|---|---|---|
| BLOCKER | Must not ship — correctness, data loss, or security-critical | Causes `sa-verdict` = FAIL |
| CRITICAL | Must not ship — known vulnerability or severe bug | Causes `sa-verdict` = FAIL |
| MAJOR | Should fix soon — functional issue or OWASP concern | Causes `sa-verdict` = WARN |
| MINOR / INFO | Low-priority — style or advisory | Informational only |

Only BLOCKER and CRITICAL findings cause `<sa-verdict>` = `FAIL`.
MAJOR findings produce `<sa-verdict>` = `WARN` — they appear in the
report but do NOT block compliance. MINOR/INFO are included in the
collapsible findings table only.

8. **Detect available tooling.**

   a. **Project stack.** Read `LOCALRULES.md` → `## Stack` (or
      equivalent field). Resolve the stack name (e.g. `Go`, `Java`,
      `TypeScript`). Hold as `<stack>`.

   b. **Standards file.** Load `standards/<stack-lowercase>.md`.
      Read the `## Static Analysis` section. It defines:
      - The native tools to run and their commands
      - The severity mapping for each tool's output

      If no `## Static Analysis` section exists in the standards file,
      log a `WARN` ("no static analysis rules defined for <stack>")
      and skip step 9 (native tools). The AI code review in step 9b
      always runs regardless.

   c. **Native tool presence.** For each tool listed in the standards
      `## Static Analysis` native-tools table, verify it is on PATH:

      ```bash
      which <tool-name> || echo "<tool-name>:absent"
      ```

      Record absent tools; skip their execution in step 9.

   Hold `<sa-toolset>` = `{ stack, native: [tool-name, ...] }`.

9. **Run native static analysis** (opportunistic — skip gracefully if
   all tools are absent; the AI review in step 9b always runs).

   Using the commands from `standards/<stack-lowercase>.md`
   `## Static Analysis` → "Native tools — commands" table, execute
   each available tool against the full module/package tree. Capture
   stdout and stderr.

   Apply the severity mapping from the standards "Native tools —
   severity mapping" table to classify each finding.

   Hold as `<native-findings>` — list of
   `{ tool, severity, category, file, line, message }`.
   If no tools were available, `<native-findings>` = `[]`.

9b. **AI-driven code review.** Always runs — no tooling required.

    Using `<diff>` (captured in step 6) and the full content of each
    changed file, perform a structured security and quality review of
    the implementation changes. Read each changed file in full to
    understand context beyond the diff lines alone.

    Apply the following checklist to the changed code:

    | Category | What to look for | Default severity |
    |---|---|---|
    | Hardcoded secrets | API keys, passwords, tokens, credentials in code or config | BLOCKER |
    | Injection | SQL, command, path injection; unsanitised user input reaching dangerous sinks | CRITICAL |
    | Insecure crypto | MD5/SHA1 for security purposes, weak PRNG, ECB mode | CRITICAL |
    | Auth/authorisation | Missing auth checks, broken access control, privilege escalation | CRITICAL |
    | Error handling | Unchecked errors, swallowed exceptions/panics, silent failures | MAJOR |
    | Nil/null safety | Missing nil guards before dereference, unchecked type assertions | MAJOR |
    | Resource management | Unclosed files/connections/responses, missing `defer`/`finally` | MAJOR |
    | Input validation | Missing bounds checks, unvalidated external or user-supplied input | MAJOR |
    | Concurrency | Shared mutable state without locks, obvious data races | MAJOR |
    | Dead code | Unreachable branches, unused variables or imports introduced by the diff | MINOR |

    For each finding:
    - Cite the specific file, function, and line range from the diff
    - Explain concisely why it is a concern — not just the category label
    - Assign severity per the table; adjust one level when context
      clearly warrants it and document the reason

    Hold as `<ai-findings>` — same schema as `<native-findings>`:
    `{ tool: "ai-review", severity, category, file, line, message }`.

    **Grounding rule (verification-procedure):** only raise findings
    that are clearly present in the diff or the files it touches. Do
    not speculate about code paths not changed by this branch. When
    uncertain whether a finding exists at all, omit it. When uncertain
    between MAJOR and MINOR, use MAJOR — the reviewer can downgrade.

10. **SonarQube — advisory note.**

    SonarQube deep analysis (full SAST, dependency CVEs, coverage
    measurement) runs automatically via `sonarcloud.yml` when the PR
    is opened by this workflow. Results appear in the PR checks and
    decorate the diff for the human reviewer in Stage 6.

    No scanner action is required here. Record the following in the
    compliance report (step 14):

    > SonarQube analysis will run post-PR via CI (`sonarcloud.yml`).
    > Results available in PR checks for the Stage 6 reviewer.

11. **Compute the static-analysis verdict.**

    Merge `<native-findings>` and `<ai-findings>` into `<sa-findings>`.

    ```
    if any finding in <sa-findings> has severity BLOCKER or CRITICAL:
      <sa-verdict> = FAIL
    else if any finding has severity MAJOR:
      <sa-verdict> = WARN
    else:
      <sa-verdict> = PASS
    ```

    Log a one-line summary before proceeding:

    ```
    Static analysis: <sa-verdict> — <B> blockers, <C> critical, <M> major, <I> minor/info
    ```

---

### Section C — AC Evaluation

12. **AC quality pre-check.** Before evaluating individual ACs,
    assess the overall AC set for behaviour coverage.

    Scan all ACs in `<acs>`. Classify each as either:
    - **Behaviour AC** — describes what a user, caller, or external
      system observes when the feature runs: what appears in the
      terminal, what the API returns, what the UI shows, what event
      is emitted.
    - **Implementation AC** — describes internal code structure:
      a function exists with certain args, a test asserts a call
      signature, the build compiles, a config value is set.

    If the Feature delivers user-facing behaviour (CLI command, API
    endpoint, UI interaction, or any change a user can directly
    invoke) AND all ACs are implementation-only with none describing
    observable behaviour, add the following to `<ai-findings>`:

    ```
    { tool: "ac-quality", severity: "MAJOR", category: "AC coverage",
      message: "All ACs are implementation-detail checks — no
      user-observable behaviour AC is present. The feature appears
      to be user-facing but no AC describes what the user sees or
      experiences. This means compliance can pass while the
      user-visible behaviour is wrong or absent." }
    ```

    This finding flows into `<sa-findings>` and contributes to
    `<sa-verdict>` as a MAJOR — visible in the report but does not
    by itself cause `sa-verdict = FAIL`. Record whether a behaviour
    AC was found; use it when writing the compliance-feedback comment
    if the overall verdict is FAIL.

    **Evaluate each AC.** For each criterion in `<acs>`, independently
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
      evidence: "Feature issue body has no ## Acceptance Criteria section. AC compliance verification is advisory only." }
    ```
    Proceed to the report step.

---

### Section D — Report and transition

13. **Compute the overall verdict.**

    ```
    ac-verdict:
      PASS  — all AC verdicts are PASS (or PASS (no-acs warning))
      FAIL  — any AC verdict is PARTIAL or FAIL

    overall-verdict:
      PASS  — ac-verdict = PASS  AND  sa-verdict ∈ {PASS, WARN}
      FAIL  — ac-verdict = FAIL  OR   sa-verdict = FAIL
    ```

    Note: `<sa-verdict>` = `WARN` does NOT cause an overall FAIL.
    WARN findings appear in the report for developer awareness but do
    not block the PR.

    Hold as `<verdict>`.

14. **Post the compliance report comment.** Always, regardless of
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
    **Overall verdict:** PASS ✅  |  FAIL ❌  (use one)

    ---

    ## Static Analysis

    **Result:** PASS ✅  |  WARN ⚠️  |  FAIL ❌  (use one)
    **Tools run:** AI code review (always); native tools run: go vet, golangci-lint (list actual tools used, or "none — AI review only")

    > SonarQube analysis will run post-PR via CI (`sonarcloud.yml`). Results available in PR checks.

    | Severity | Count |
    |---|---|
    | 🔴 BLOCKER | <n> |
    | 🟠 CRITICAL | <n> |
    | 🟡 MAJOR | <n> |
    | 🔵 MINOR / INFO | <n> |

    <!-- Only render the findings table when BLOCKER/CRITICAL/MAJOR findings exist -->
    <details>
    <summary>Findings requiring attention (<B+C+M> issues)</summary>

    | Severity | Tool | Location | Message |
    |---|---|---|---|
    | 🔴 BLOCKER | ai-review | `internal/auth/token.go:42` | Hardcoded API token — move to env var or secret store |
    | 🟠 CRITICAL | ai-review | `cmd/serve.go:88` | User-supplied path passed to os.Open without sanitisation — path traversal risk |
    | 🟠 CRITICAL | govulncheck | `go.mod` | CVE-2024-XXXX in golang.org/x/net < 0.23.0 |
    | 🟡 MAJOR | go vet | `pkg/handler.go:33` | Error return value not checked |

    </details>

    ---

    ## Acceptance Criteria

    | AC | Status | Evidence |
    |---|---|---|
    | AC-1 | ✅ PASS | `pkg/foo.go:42` — `funcBar` implements the stated behaviour; `TestFuncBar` asserts the outcome |
    | AC-2 | ❌ FAIL | No implementation found for X; no test asserts Y |
    | AC-3 | ⚠️ PARTIAL | Implementation in `pkg/baz.go` covers the success path; error path is unhandled; no test for error case |

    ---

    ## Required Fixes
    <!-- only present when overall verdict is FAIL -->

    ### Static Analysis Issues
    <!-- only present when sa-verdict = FAIL -->

    - **🔴 BLOCKER — ai-review:** `internal/auth/token.go:42` — remove
      the hardcoded token; load it from an environment variable instead.
    - **🟠 CRITICAL — govulncheck:** `go.mod` — upgrade `golang.org/x/net`
      to ≥ 0.23.0 to resolve CVE-2024-XXXX.

    ### Acceptance Criteria Issues
    <!-- only present when any AC is FAIL or PARTIAL -->

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

15. **On PASS — compose and post the PR body.**

    Using the feature title, `<diff-stat>`, `<evaluations>`, and
    `<sa-findings>`, compose a PR body and post it as a
    `<!-- pr-body:v1 -->` comment on the feature issue. The workflow
    reads this comment when opening the PR.

    The body must follow this structure exactly:

    ```markdown
    <!-- pr-body:v1 -->

    ## Summary

    <2–3 sentences describing what was implemented: what problem was
    solved, what changed, and the approach taken. Derived from the
    feature title, body, and the diff. Be specific — name the key
    function or file changed.>

    ## Changes

    <bullet list — max 5 bullets — of the key files and functions
    modified. Format: `path/to/file.go` — what changed.>

    ## Acceptance Criteria

    | AC | Status |
    |---|---|
    | <AC-1 text, truncated to ~70 chars if long> | ✅ PASS |
    | <AC-2 text> | ✅ PASS |
    <!-- one row per AC from <evaluations> — PASS/PARTIAL/FAIL -->

    ## Static Analysis

    <sa-verdict emoji> <sa-verdict> — AI code review: <B> blockers, <C> critical, <M> major (<I> informational)

    > SonarQube deep analysis will run on this PR via CI (`sonarcloud.yml`).

    Closes #<N>

    🤖 Generated with [gh-agentic](https://github.com/eddiecarpenter/gh-agentic)
    ```

    Post via `post-issue-comment`. Verify posted:

    ```bash
    gh issue view <N> --repo <active-repo> --json comments \
      --jq '[.comments[] | select(.body | startswith("<!-- pr-body:v1 -->"))] | length'
    ```

    Should be ≥ 1. On failure → log a `WARN` and continue — the
    workflow falls back to a minimal body. Do NOT raise a hard error;
    a missing PR body must not block the compliance-verified label.

16. **On PASS — apply `compliance-verified`.**

    ```bash
    gh issue edit <N> --repo <active-repo> \
      --add-label "compliance-verified"
    ```

    Use `GH_TOKEN` from the environment (the workflow sets this to
    `PIPELINE_PAT` — required so the label event can trigger the
    `pr-review-session` workflow if needed). Verify the label is
    present after the edit.

    The surrounding workflow's `Open PR if compliance-verified` step
    reads the `<!-- pr-body:v1 -->` comment and opens the PR with it.

    Emit **Output A** (step 19) and exit.

17. **On FAIL — post the feedback comment.**

    Post a second comment containing the actionable feedback:

    ```markdown
    <!-- compliance-feedback:v1 -->

    # Compliance Feedback — Feature #<N>

    The compliance verification run found the following issues not
    resolved. The dev session will re-enter with this feedback as
    context.

    ## Static Analysis Issues Requiring Fixes
    <!-- only present when sa-verdict = FAIL -->

    - **🔴 BLOCKER — <tool>:** `<file>:<line>` — <message>
      - Fix needed: <concrete description of the change required>
    - **🟠 CRITICAL — <tool>:** `<file>:<line>` — <message>
      - Fix needed: <concrete description>

    <!-- repeated for each BLOCKER or CRITICAL finding -->

    ## Acceptance Criteria Requiring Work
    <!-- only present when any AC is FAIL or PARTIAL -->

    - **AC-<idx>:** <one-sentence summary of what's missing>
      - Evidence: <specific file/function/test reference>
      - Fix needed: <concrete description>

    <!-- repeated for each FAIL or PARTIAL AC -->

    The feature branch already contains the work for passing checks.
    The dev session should implement ONLY the fixes listed above —
    do not re-implement passing criteria or re-run passing tools.
    ```

    Verify posted:
    ```bash
    gh issue view <N> --repo <active-repo> --json comments \
      --jq '[.comments[] | select(.body | startswith("<!-- compliance-feedback:v1 -->"))] | length'
    ```

18. **On FAIL — swap `in-verification` → `in-development`.**

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

### Section E — Closeout

19. **Emit the exit block.** Match the actual outcome:

    **Output A — PASS:**
    ```
    === Compliance Verify — PASS ===

    Feature #<N> satisfies all checks.

    Results:
      - Static analysis: <sa-verdict> (PASS or WARN — no blockers/criticals)
      - <M> of <M> ACs passed
      - compliance-verified label applied
      - Report posted on issue #<N>

    Next: workflow opens PR and applies in-review
    ```

    **Output B — FAIL:**
    ```
    === Compliance Verify — FAIL ===

    Feature #<N> does not yet satisfy all checks.

    Results:
      - Static analysis: <sa-verdict> — <B> blockers, <C> critical, <M> major
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
    AC compliance verification was advisory only.

    Results:
      - Static analysis: <sa-verdict>
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

    Next: human must intervene — review last compliance report and feedback
    comment, then manually reset the pipeline or close the Feature.
    ```

20. **Terminate the session.** Per `emits-exit-block: true`, invoke
    the host runtime's session-close API if available; otherwise
    halt.

## Error Handling

- `INVALID_VERIFY_STATE` from steps 2–3 (Feature missing, wrong
  labels, empty diff) → severity `ERROR`; propagate. The pipeline
  invoked this skill against a Feature not ready for verification.
- `BRANCH_MISSING` from step 4 (no remote feature branch) → severity
  `ERROR`; propagate. The feature branch must exist before
  `in-verification` is applied.
- `REPORT_FAILED` from step 14 (compliance report comment did not
  post) → severity `ERROR`; propagate. Without the report, the
  state is ambiguous — do not apply `compliance-verified` or swap
  labels when the audit trail cannot be written.
- `CYCLE_CAP_EXCEEDED` from step 3b (feedback-count ≥ 3) → severity
  `WARN` (not `ERROR`); this is an expected pipeline state, not an
  environment failure. The escalation comment and label transition
  are applied and the skill exits cleanly with `exit_state =
  "cycle-cap"`. Do not propagate as an uncaught exception.
- All other errors: propagate (default).
