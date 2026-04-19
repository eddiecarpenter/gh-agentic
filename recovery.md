# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #570                               |
| Branch              | feature/570-refactor-assessment-skill |
| Last commit         | d1973bb                            |
| Total tasks         | 6                                  |
| Last updated        | 2026-04-19T07:01:00Z               |

## Completed Tasks

### #572 — Create skills/refactor-assessment.md canonical Reference skill
- **Implemented:** Authored `skills/refactor-assessment.md` — a Reference skill defining the canonical Reuse & Refactor Discipline: search procedure, three permitted outcomes, opt-out variant, single-line `Reuse:` recording format, and the verbatim loader phrase consumer skills must invoke. Includes one worked example per outcome.
- **Files changed:** skills/refactor-assessment.md
- **Decisions:** Canonical outcome token set = `as-is|via-refactor|none|opt-out|n/a`. Loader phrase: "Invoke skills/refactor-assessment.md before writing any new code." Recording line shape: `Reuse: <token> — <reason>`.

### #573 — Integrate Refactor Assessment into skills/feature-design.md
- **Implemented:** Added `refactor-assessment` to `loads`. Inserted step 4 (Refactor Assessment) between codebase analysis and task creation. Extended exit block with refactor-outcome line. Added rule blocking task emission until assessment is recorded.
- **Files changed:** skills/feature-design.md
- **Decisions:** Refactor tasks placed first in task ordering when outcome is reuse-via-refactor.

### #574 — Integrate Reuse & Refactor Check into skills/dev-session.md
- **Implemented:** Added `refactor-assessment` to `loads`. Added a per-task Reuse & Refactor Check sub-step that runs before any new symbol is written; session halts if the check is skipped. Extended task-commit and unit-commit formats with the canonical `Reuse:` trailer. Extended Current Task `Notes` field guidance. Added rule forbidding new-symbol commits without the reuse trailer.
- **Files changed:** skills/dev-session.md
- **Decisions:** None beyond those already set by task 572.

### #575 — Integrate Reuse & Refactor Check into skills/issue-session.md
- **Implemented:** Added `refactor-assessment` to `loads`. Bug route performs the check before the fix; bug-fix commit body carries the canonical `Reuse:` trailer (with `n/a` for pure local fixes). Scope Check section notes that a `reuse via refactor` outcome exceeding safe scope routes to `needs-human`. Added rules: mandatory trailer on bug-fix commits; out-of-scope reuse routes to needs-human. Question route unchanged.
- **Files changed:** skills/issue-session.md
- **Decisions:** None beyond those already set by task 572.

## Remaining Tasks

- [ ] #576 — Add universal Reuse & Refactor Discipline rule to RULEBOOK.md ← current
- [ ] #577 — Verify frontmatter, rebuild CATALOGUE.md, confirm reference-skill classification
