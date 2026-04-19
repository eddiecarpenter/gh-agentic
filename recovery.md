# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #570                               |
| Branch              | feature/570-refactor-assessment-skill |
| Last commit         | f90aa4b                            |
| Total tasks         | 6                                  |
| Last updated        | 2026-04-19T06:59:00Z               |

## Completed Tasks

### #572 — Create skills/refactor-assessment.md canonical Reference skill
- **Implemented:** Authored `skills/refactor-assessment.md` — a Reference skill defining the canonical Reuse & Refactor Discipline: search procedure, three permitted outcomes (reuse as-is / reuse via refactor / do not reuse), opt-out variant, single-line `Reuse:` recording format, and the verbatim loader phrase consumer skills must invoke. Includes one worked example per outcome.
- **Files changed:** skills/refactor-assessment.md
- **Decisions:** Canonical outcome token set = `as-is|via-refactor|none|opt-out|n/a`. Canonical loader phrase: "Invoke skills/refactor-assessment.md before writing any new code." Recording line shape: `Reuse: <token> — <reason>` placed in commit trailer or design-artefact note. These are normative — tasks 2–5 use them verbatim.

### #573 — Integrate Refactor Assessment into skills/feature-design.md
- **Implemented:** Added `refactor-assessment` to `loads`. Inserted a new step 4 (Refactor Assessment) between codebase analysis and task creation. Extended the exit block's `Produced` section with a refactor-outcome line. Added a rule blocking task emission until the assessment is recorded. Renumbered existing steps 4–9 to 5–10.
- **Files changed:** skills/feature-design.md
- **Decisions:** Refactor tasks are placed **first** in the emitted task ordering when the outcome is reuse-via-refactor, ahead of feature tasks that depend on them.

## Remaining Tasks

- [ ] #574 — Integrate Reuse & Refactor Check into skills/dev-session.md ← current
- [ ] #575 — Integrate Reuse & Refactor Check into skills/issue-session.md
- [ ] #576 — Add universal Reuse & Refactor Discipline rule to RULEBOOK.md
- [ ] #577 — Verify frontmatter, rebuild CATALOGUE.md, confirm reference-skill classification
