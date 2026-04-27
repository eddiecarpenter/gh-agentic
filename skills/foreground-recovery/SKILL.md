---
name: foreground-recovery
description: Diagnoses and recovers stuck pipeline state interactively — stale concurrency beacons (design-in-progress / development-in-progress / issue-in-progress), label-vs-status mismatches, partial design or dev artefacts on a Feature, orphan feature branches, and backwards-transitioned issues. Walks the human through what's wrong, proposes a remediation, and applies it only after explicit confirmation. Use when the human suspects the pipeline is stuck on a specific Requirement / Feature / issue, or wants to scan the whole project for known stuck-state patterns. Use even when the caller doesn't say "foreground-recovery" — phrases like "this Feature is stuck", "recover from a partial state", "the design-in-progress label is stuck", "fix the broken pipeline state on #42" should trigger this skill.
triggers: human-interactive
user-invocable: true
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/step-skip-rule.md
  - skills/prompt-user/SKILL.md
  - skills/gh-agentic/SKILL.md
  - skills/apply-label/SKILL.md
  - skills/set-issue-status/SKILL.md
  - skills/post-issue-comment/SKILL.md
emits-exit-block: true
exit-hands-to: "human: re-run the appropriate phase skill against the recovered issue, or pick the next stuck item from the scan output"
---

# Foreground Recovery

## Goal

When something is stuck in the pipeline — a beacon label that
won't release, a Feature whose label and project status disagree,
a partial design that didn't reach the trigger step, an orphan
branch — diagnose the issue interactively and apply a targeted
remediation. Every mutation is gated by explicit human consent;
the skill prefers reading and explaining over acting.

The skill is interactive and runs in the foreground. It is the
rescue tool the partial-state cases in the headless skills
intentionally surface rather than try to auto-recover from.

## Output Artefacts

- **Diagnosis output** — for each issue inspected, a structured
  report listing detected anomalies with their classifying code
  (e.g. `STUCK_BEACON`, `LABEL_STATUS_MISMATCH`).
- **Remediation actions, applied** — only when the human confirmed
  each one. Each action is one of:
  - Remove a stale beacon label.
  - Realign label and project status (via `apply-label` or
    `set-issue-status`).
  - Close an orphan Task or Feature.
  - Delete a local feature branch (after extra confirmation).
  - Post a recovery comment on the issue describing what was
    changed and why.
- A return value at exit summarising the run:
  ```
  { repo: <string>, mode: "targeted" | "scan",
    inspected: <int>, anomalies_found: <int>,
    remediations_applied: <int>,
    items_punted: <int> }
  ```

`items_punted` is the count of anomalies the human declined to fix
in this session (left as-is for later attention).

The skill's three valid terminal outputs:

**A. Recovered.** At least one anomaly was diagnosed and remediated.

**B. Diagnosed only.** Anomalies found but the human chose not to
apply any remediation (informational run).

**C. Clean.** No anomalies detected on the inspected target(s).
Nothing to do.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy for
  `INVALID_TARGET`, `REMEDIATION_FAILED`, `USER_CANCELLED`.
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement.
  Mode-gated steps (Targeted vs Scan) follow the carve-out.

This skill does NOT load `commit-discipline.md` because it does
not produce code commits. The `apply-label` and `set-issue-status`
primitives carry their own atomicity rules.

## Dependencies

- `skills/prompt-user/SKILL.md` — used at every remediation gate
  (the human approves each action, never a batch).
- `skills/gh-agentic/SKILL.md` — used in step 3 to read the target
  issue's full state and (in scan mode) to enumerate
  Requirements/Features in the project.
- `skills/apply-label/SKILL.md` — used in remediations that swap
  labels.
- `skills/set-issue-status/SKILL.md` — used in remediations that
  realign project status.
- `skills/post-issue-comment/SKILL.md` — used to post the recovery
  comment on the issue when remediations are applied.

## Steps

The **step-skip rule** applies. Mode-gated carve-out: Scan-only
steps (4 in particular) are not run when in Targeted mode and
vice versa — by design.

**Resolving the active repo.** Resolve once via:

```bash
gh repo view --json nameWithOwner -q .nameWithOwner
```

and reuse as `<active-repo>`.

**Read-only first, write only on confirm.** This skill never
applies a label change, status update, branch delete, or issue
close without a fresh `prompt-user` confirmation that names the
exact mutation. Human-in-the-loop is the entire point — bulk
"approve all" is not offered.

---

### Section A — Setup and mode pick

1. **Announce the session.** Print the banner verbatim before any
   tool call:

   ```
   ==========================================================
   === Foreground Recovery — Started                          ===
   ==========================================================
   ```

   Resolve the active repo per the rule above and hold as
   `<active-repo>`.

   **Branch-safety check.** Some remediations in this skill touch
   the local working tree (deleting branches, possibly closing
   issues that auto-update branches via gh hooks). The agent MUST
   refuse to operate from `main` to prevent any chance of
   accidental main-branch mutations.

   ```bash
   git branch --show-current
   ```

   - Result `main` (or `master`) → exit cleanly with a clear
     remediation message:
     ```
     This skill refuses to run on main. Switch to a branch first
     (e.g. `git checkout -b chore/recovery-<timestamp>`), then
     re-invoke /foreground-recovery.
     ```
     No prompt, no scan, no mutations.
   - Anything else → continue. Hold the branch name as `<branch>`.

2. **Pick mode.** Ask the human:

   ```
   prompt-user(
     question: "How do you want to recover?",
     header: "Recovery mode",
     options: [
       {label: "Targeted — I know which issue is stuck",
        description: "Give a number; the skill diagnoses just that one."},
       {label: "Scan — find stuck items across the project",
        description: "Walk every Requirement/Feature/issue and list anomalies."},
       {label: "Cancel",
        description: "Exit without doing anything."}
     ]
   )
   ```

   - Targeted → step 3.
   - Scan → step 4.
   - Cancel → Output C variant (no inspection done). Exit.

3. **Targeted: get the issue number. (targeted only)** Ask via
   plain conversation: "Which issue number is stuck?" Wait for the
   reply. Parse to an int; on parse failure, ask once more, then
   raise `INVALID_TARGET` (`WARN`) and exit.

   Hold the parsed number as `<N>`. Continue to Section B with
   the single-element list `<targets> = [<N>]`.

4. **Scan: enumerate candidates. (scan only)** Query the project
   for all open Requirements and Features:

   ```bash
   gh agentic status requirements --raw
   gh agentic status features --raw
   ```

   The two TSV outputs combined yield `<targets>` — the full list
   of issue numbers. Continue to Section B.

   For scan mode, surface to the human: "Inspecting <count>
   open issues..." before the loop begins.

---

### Section B — Diagnose

5. **For each `<N>` in `<targets>`:** read the full state and
   classify anomalies.

   Read state:
   ```bash
   gh agentic status feature <N> --raw
   ```
   (or `requirement` if the issue is a Requirement; the `--raw`
   output indicates type via the labels). Capture as `<state>`.

   Read the branch state, if applicable. For a Feature, the
   expected branch is `feature/<N>-*`:
   ```bash
   git ls-remote --heads "https://github.com/<active-repo>" \
     "feature/<N>-*"
   ```

6. **Classify anomalies.** Run each detector in order; record any
   that fire.

   **Detectors:**

   - **`STUCK_BEACON`** — one of `design-in-progress`,
     `development-in-progress`, `issue-in-progress` is set, and
     the corresponding session is not actually running. The skill
     cannot detect "not running" perfectly; treat any beacon that
     has been set for more than the obvious window (the human
     judges) as stuck. The agent surfaces the timestamp of the
     label and asks the human to confirm it's stale.

   - **`LABEL_STATUS_MISMATCH`** — the label set indicates one
     stage but the project Status field says another. Examples:
     - Label `in-design` + status `Backlog`.
     - Label `done` + status `In Review`.
     - Label `in-development` + status `Designed`.

     Compute the canonical stage from labels (highest-precedence
     lifecycle label wins; `done` > `in-review` > `in-development`
     > `designed` > `in-design`/`interactive-design` > `scoping`
     > `ready-to-implement` > `backlog`) and compare to the
     status. Mismatch is anomalous.

   - **`PARTIAL_DESIGN`** (Feature only) — the Feature is at
     `in-design` or `interactive-design` and has SOME but not
     ALL of: rationale comment (`<!-- design-plan:v1 -->`),
     feature branch, child Task issues. Either everything is
     present (then the trigger should have fired) or nothing is
     (then design hasn't run yet); a partial set means the run
     died part-way.

   - **`PARTIAL_DEV`** (Feature only) — the Feature is at
     `in-development`, the feature branch exists with commits,
     but some child Tasks are still open AND the
     `development-in-progress` beacon is NOT set. (Beacon set
     means a session is or was running; absent + open tasks +
     commits means a dev session ended without finishing.)

   - **`ORPHAN_BRANCH`** (Feature only) — the feature branch
     exists locally or remotely, but the Feature is closed or
     `cancelled` or has been done for > 30 days. Branch
     leftovers from completed work that never got cleaned up.

   - **`BACKWARDS_TRANSITION`** — the Feature was previously at
     a later stage and is now at an earlier one without a
     corresponding human action recorded. (Heuristic: check
     issue events for label-removal patterns that don't match
     the canonical pipeline transitions.)

   - **`UNEXPECTED_STAGE`** — the Feature carries a stage label
     not in the canonical set, OR carries multiple mutually-
     exclusive lifecycle labels (e.g., both `in-design` AND
     `in-development`). This is corruption that needs human
     judgement.

   - **`STALE_NEEDS_SCOPING`** — an issue has `needs-scoping`
     applied for > 14 days with no follow-up activity. Surface
     for the human to triage (close, capture as a Requirement,
     or extend).

   Record the detections as `<anomalies>` — a list per `<N>`.

7. **If `<anomalies>` is empty for the entire targets list** —
   Output C. Surface "No anomalies detected." and exit cleanly.

---

### Section C — Remediation walk

8. **For each issue with anomalies, render the report:**

   ```
   === Issue #<N>: <title> ===

   Anomalies detected:
     - <CODE_1>: <one-line description>
     - <CODE_2>: <one-line description>

   Current state:
     labels: <list>
     status: <status>
     branch: <branch state>
     tasks:  <open/total>
     comments: <count, with rationale-comment marker?>
   ```

   Present each anomaly with its proposed remediation. The
   skill does NOT bundle remediations — each is its own
   `prompt-user`.

9. **Per-anomaly remediation gate.** For each anomaly in order:

   Render the proposed action verbatim (the exact tool call(s)
   that would run), then:

   ```
   prompt-user(
     question: "Apply this remediation?",
     header: "<CODE> on #<N>",
     options: [
       {label: "Yes, apply",
        description: "Run the action shown above."},
       {label: "Skip — leave as-is",
        description: "I'll handle this one manually or later."},
       {label: "Cancel session",
        description: "Stop the recovery walk; exit."}
     ]
   )
   ```

   - Yes → execute the action (step 10). On success, move to
     the next anomaly. On failure → raise `REMEDIATION_FAILED`
     (`WARN`); surface and continue with next anomaly.
   - Skip → record as punted; continue.
   - Cancel → exit with whatever has been applied so far
     (Output A or B depending on count).

10. **Standard remediations.** Each anomaly maps to a canonical
    remediation. Apply only the one the human confirmed.

    | Anomaly | Remediation |
    |---|---|
    | `STUCK_BEACON` | Remove the beacon label via `apply-label(remove=[<beacon>])`. |
    | `LABEL_STATUS_MISMATCH` | Surface both candidate fixes — "align status to label" (run `set-issue-status` to match the canonical-stage from labels) or "align label to status" (run `apply-label` to swap to the label matching the status). Ask the human which is correct; apply the chosen one. |
    | `PARTIAL_DESIGN` | Two paths: (a) close the partial artefacts (close orphan Tasks, delete branch, revert Feature label to `backlog`); or (b) leave artefacts in place and revert just the trigger label so the human can re-invoke `feature-design`. The agent surfaces both; the human picks. |
    | `PARTIAL_DEV` | Same shape as PARTIAL_DESIGN: revert vs leave. Closes any child task that has a corresponding commit on the branch as the close criterion. |
    | `ORPHAN_BRANCH` | Confirm by name; delete the local branch via `git branch -D` (extra-confirm prompt). Do NOT delete the remote branch — that requires a separate explicit human action. |
    | `BACKWARDS_TRANSITION` | Surface the event log; ask the human whether to re-apply the missing label transition (re-set to the previous higher stage) or accept the current state as a deliberate revert. |
    | `UNEXPECTED_STAGE` | No automated remediation. Surface the corruption clearly; recommend manual `gh issue edit` to the canonical state. |
    | `STALE_NEEDS_SCOPING` | Surface options: close the issue, run `requirements-session` to capture as a Requirement, or extend by removing `needs-scoping`. The human picks. |

11. **Post the recovery comment.** When at least one remediation
    was applied to an issue, post a brief comment on that issue
    summarising what was done:

    ```
    Foreground recovery applied:
      - <CODE_1>: <action taken>
      - <CODE_2>: <action taken>

    Re-run the appropriate phase skill (or pick up manually) when
    ready.
    ```

    Via `post-issue-comment`. On failure → `REMEDIATION_FAILED`
    (`WARN`); the recovery itself succeeded, only the comment
    didn't. Surface in the exit block.

---

### Section D — Closeout

12. **Emit the exit block.** Match the actual outcome:

    **Output A — Recovered:**
    ```
    === Foreground Recovery — Completed ===

    Inspected: <count> issue(s) (<mode>)
    Anomalies found: <count>
    Remediations applied: <count>
    Punted: <count>

    Per-issue summary:
      - #<N1>: <CODE>: applied | <CODE>: punted
      - #<N2>: ...

    Next: re-run the appropriate phase skill on each remediated
          issue when ready, or run /foreground-recovery again to
          handle the punted items.
    ```

    **Output B — Diagnosed only:**
    ```
    === Foreground Recovery — Diagnosed (no remediations applied) ===

    Inspected: <count> issue(s)
    Anomalies found: <count>

    Per-issue summary:
      - #<N1>: <CODE_list>
      - #<N2>: <CODE_list>

    Next: address each anomaly manually, or re-invoke
          /foreground-recovery to apply remediations interactively.
    ```

    **Output C — Clean:**
    ```
    === Foreground Recovery — Clean ===

    Inspected: <count> issue(s)
    Anomalies found: 0

    The pipeline state is consistent. Nothing to do.
    ```

13. **Terminate the session.** Per `emits-exit-block: true`,
    invoke the host runtime's session-close API if available;
    otherwise halt.

## Verification

Per `skills/definitions/verification-procedure.md` "Section format".
Skill-specific commands:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/foreground-recovery/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/foreground-recovery/SKILL.md
```

Pass criteria: both commands exit 0.
## Error Handling

- `INVALID_TARGET` from step 3 (target issue number unparseable
  or doesn't exist) → severity `WARN`. Exit cleanly with a clear
  diagnosis. The human can re-invoke with a corrected number.
- `REMEDIATION_FAILED` from step 10 (`apply-label`,
  `set-issue-status`, branch delete, or `gh issue close` raised)
  → severity `WARN`. Surface; continue with the next anomaly. The
  exit block reports which remediations succeeded vs failed.
- `USER_CANCELLED` (any cancel point) → severity `WARN`. End
  cleanly with whatever was applied so far.
- All other errors: propagate (default).

**Destructive-action rule.** Three remediations are extra-cautious
because they cannot be cheaply undone:

- **Local branch delete** (`ORPHAN_BRANCH`): the per-anomaly
  prompt is followed by a SECOND `prompt-user` that names the
  branch verbatim and asks for confirmation. Skip equals abandon
  the action.
- **Closing an issue** (`STALE_NEEDS_SCOPING`, `PARTIAL_DESIGN`
  partial close): same shape — second prompt that names the
  issue.
- **Backwards-transition re-application** (`BACKWARDS_TRANSITION`):
  same — second prompt clarifying the label/status that will be
  re-applied and what the current state is.

These second-confirmation gates apply on top of the standard
per-anomaly gate; "Yes, apply" on the first prompt opens the
extra-confirm prompt rather than executing immediately.
