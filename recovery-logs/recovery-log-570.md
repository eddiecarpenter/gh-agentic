# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #570                               |
| Branch              | feature/570-refactor-assessment-skill |
| Last commit         | d31eb02                            |
| Total tasks         | 6                                  |
| Last updated        | 2026-04-19T07:02:00Z               |

## Completed Tasks

### #572 — Create skills/refactor-assessment.md canonical Reference skill
- **Implemented:** Authored `skills/refactor-assessment.md` — a Reference skill defining the canonical Reuse & Refactor Discipline.
- **Files changed:** skills/refactor-assessment.md
- **Decisions:** Outcome tokens `as-is|via-refactor|none|opt-out|n/a`; loader phrase and `Reuse:` trailer format are canonical.

### #573 — Integrate Refactor Assessment into skills/feature-design.md
- **Implemented:** Added `refactor-assessment` to `loads`. New step 4 (Refactor Assessment) between codebase analysis and task creation. Extended exit block. Added rule blocking task emission until assessment is recorded.
- **Files changed:** skills/feature-design.md
- **Decisions:** Refactor tasks placed first in task ordering when outcome is reuse-via-refactor.

### #574 — Integrate Reuse & Refactor Check into skills/dev-session.md
- **Implemented:** Added `refactor-assessment` to `loads`. Per-task check before any new symbol; session halts if skipped. Extended commit format with `Reuse:` trailer. Extended Current Task `Notes` guidance. Added rule.
- **Files changed:** skills/dev-session.md
- **Decisions:** None beyond those set by task 572.

### #575 — Integrate Reuse & Refactor Check into skills/issue-session.md
- **Implemented:** Added `refactor-assessment` to `loads`. Bug route performs the check; commit body carries `Reuse:` trailer. Out-of-scope reuse routes to `needs-human`. Added rules.
- **Files changed:** skills/issue-session.md
- **Decisions:** None beyond those set by task 572.

### #576 — Add universal Reuse & Refactor Discipline rule to RULEBOOK.md
- **Implemented:** Added a "Reuse & Refactor Discipline" section (between Sensitive Operations and Framework Files — Read Only) naming the three outcomes, the opt-out, and cross-referencing skills/refactor-assessment.md. No duplication of the procedure.
- **Files changed:** RULEBOOK.md
- **Decisions:** RULEBOOK section is intentionally short — per the token-cost-first principle, detail lives in the skill.

## Remaining Tasks

- [ ] #577 — Verify frontmatter, rebuild CATALOGUE.md, confirm reference-skill classification ← current
