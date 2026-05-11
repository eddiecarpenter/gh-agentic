---
name: compliance-verify
description: Verifies that the implementation on a feature branch passes both a static analysis gate (code standards, OWASP vulnerability checks, bug scan via language-native tools and SonarQube) and every acceptance criterion from the Feature issue. Posts a structured verdict, and either applies `compliance-verified` (all checks pass — workflow then opens the PR) or posts a `<!-- compliance-feedback:v1 -->` comment and swaps `in-verification` back to `in-development` (triggering a new dev session). Use when GitHub Actions fires on a feature issue labelled `in-verification`. Headless only.
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

1. **Static analysis** — code standards, OWASP-category vulnerability
   checks, and a bug scan using language-native tools and SonarQube
   (if configured). Any BLOCKER or CRITICAL finding is a hard failure.

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

    c. Set `exit_state = "cycle-cap"` and emit **Output E** (step 18).
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

### Section B — Static Analysis

Perform a code-quality and security gate before AC evaluation. This
section runs entirely against the checked-out feature branch. All
findings are recorded for inclusion in the compliance report (step 14)
and, if the overall verdict is FAIL, the feedback comment (step 16).

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
      - The coverage gate command and threshold
      - The SonarQube OWASP hotspot severity mapping

      If no `## Static Analysis` section exists in the standards file,
      log a `WARN` ("no static analysis rules defined for <stack>")
      and skip steps 9, 9b. Proceed to step 10 (SonarQube only).

   c. **SonarQube availability.** Check for both a project
      configuration file and a valid token:

      ```bash
      test -f sonar-project.properties && echo "config:yes" || echo "config:no"
      test -n "${SONAR_TOKEN:-}"        && echo "token:yes"  || echo "token:no"
      ```

      If both present → `<sonar-available>` = true.
      If either missing → `<sonar-available>` = false. Log a note;
      do NOT treat as an error.

   d. **Native tool presence.** For each tool listed in the standards
      `## Static Analysis` native-tools table, verify it is on PATH:

      ```bash
      which <tool-name> || echo "<tool-name>:absent"
      ```

      Record absent tools; skip their execution in step 9.

   Hold `<sa-toolset>` = `{ stack, sonar: bool, native: [tool-name, ...] }`.

9. **Run native static analysis.**

   Using the commands from `standards/<stack-lowercase>.md`
   `## Static Analysis` → "Native tools — commands" table, execute
   each available tool against the full module/package tree. Capture
   stdout and stderr.

   Apply the severity mapping from the standards "Native tools —
   severity mapping" table to classify each finding.

   Hold as `<native-findings>` — list of
   `{ tool, severity, category, file, line, message }`.

9b. **Run the coverage gate.**

    Using the command from `standards/<stack-lowercase>.md`
    `## Static Analysis` → "Coverage gate", execute the coverage
    measurement. Parse the total coverage percentage.

    Hold as `<coverage-pct>` (numeric).

    Apply the threshold and severity mapping from the standards
    "Coverage gate" table. If coverage is below the threshold, add
    a finding to `<native-findings>`:

    ```
    { tool: "<stack>-test-coverage",
      severity: <per standards table>,
      category: "coverage",
      file: "overall",
      line: "—",
      message: "Test coverage is <coverage-pct>% — minimum required
                is <threshold>%. <gap>% of statements are untested." }
    ```

    If the test suite itself fails (compilation error or test panic),
    record a CRITICAL finding per failing package and proceed —
    coverage is unmeasurable but the failure must be reported.

    **SonarQube cross-check** (only when `<sonar-available>` = true):
    After step 10 completes, compare `<coverage-pct>` against the
    `coverage` metric from `get_component_measures`. If they diverge
    by more than 5 percentage points, log a discrepancy warning in the
    report (informational only — native measurement is authoritative).

10. **Run SonarQube analysis** (skip entirely if `<sonar-available>` = false).

    a. **Trigger analysis.** Submit the branch to the SonarQube server:

       ```bash
       sonar-scanner \
         -Dsonar.branch.name="$BRANCH" \
         -Dsonar.token="$SONAR_TOKEN"
       ```

       Wait for the analysis task to complete. On scanner failure, log
       a `WARN`, set `<sonar-available>` = false, and continue with
       native findings only. Do NOT raise a hard error — a SonarQube
       outage must not block compliance verification.

    b. **Fetch quality gate status.**

       ```
       get_project_quality_gate_status(projectKey=<sonar-project-key>)
       ```

       If gate status = `ERROR` → record one BLOCKER finding:
       `{ tool: "sonarqube-gate", severity: "BLOCKER",
          message: "Quality gate failed — see SonarQube dashboard" }`.

    c. **Fetch security hotspots** (OWASP category coverage).

       ```
       search_security_hotspots(
         projectKey=<sonar-project-key>,
         status=TO_REVIEW,
         branch=<BRANCH>
       )
       ```

       Map each hotspot to a compliance severity using the
       "SonarQube — OWASP hotspot severity mapping" table from
       `standards/<stack-lowercase>.md` `## Static Analysis`.
       If the standards file has no such table, use MAJOR for all
       hotspots as a safe default.

    d. **Fetch bugs and vulnerabilities.**

       ```
       search_sonar_issues_in_projects(
         projectKeys=<sonar-project-key>,
         types=BUG,VULNERABILITY,
         severities=BLOCKER,CRITICAL,MAJOR,
         branch=<BRANCH>,
         resolved=false
       )
       ```

       Map each issue directly to the SonarQube severity returned.

    Merge all SonarQube findings into `<sonar-findings>` — same schema
    as `<native-findings>`. Deduplicate against `<native-findings>` on
    `{ file, line, message }` — keep the higher-severity entry.

11. **Compute the static-analysis verdict.**

    Merge `<native-findings>` and `<sonar-findings>` into
    `<sa-findings>`.

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

12. **Evaluate each AC.** For each criterion in `<acs>`, independently
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
    **Tools run:** go vet, golangci-lint, govulncheck, go-test-coverage, SonarQube  (list actual tools used)

    | Metric | Value | Threshold | Status |
    |---|---|---|---|
    | Test coverage | <coverage-pct>% | ≥ 80% | ✅ PASS  \|  ❌ FAIL (use one) |

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
    | 🔴 BLOCKER | sonarqube-gate | — | Quality gate failed |
    | 🟠 CRITICAL | go-test-coverage | overall | Test coverage is 61.2% — minimum required is 80% |
    | 🟠 CRITICAL | govulncheck | `go.mod` | CVE-2024-XXXX in golang.org/x/net < 0.23.0 |
    | 🟡 MAJOR | golangci-lint (errcheck) | `cmd/bar.go:88` | Error return value not checked |

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

    - **🔴 BLOCKER — sonarqube-gate:** Quality gate failed. Resolve all
      open SonarQube issues before re-submitting.
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

15. **On PASS — apply `compliance-verified`.**

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

    Emit **Output A** (step 18) and exit.

16. **On FAIL — post the feedback comment.**

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

17. **On FAIL — swap `in-verification` → `in-development`.**

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

18. **Emit the exit block.** Match the actual outcome:

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

19. **Terminate the session.** Per `emits-exit-block: true`, invoke
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
- **SonarQube scanner failure** (step 10a) → severity `WARN`. Log,
  set `<sonar-available>` = false, and continue with native-tool
  findings only. A SonarQube outage must not block compliance
  verification.
- All other errors: propagate (default).
