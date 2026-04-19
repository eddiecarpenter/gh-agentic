# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #570                               |
| Branch              | feature/570-refactor-assessment-skill |
| Last commit         | a8790ba                            |
| Total tasks         | 6                                  |
| Last updated        | 2026-04-19T06:58:00Z               |

## Completed Tasks

### #572 — Create skills/refactor-assessment.md canonical Reference skill
- **Implemented:** Authored `skills/refactor-assessment.md` — a Reference skill defining the canonical Reuse & Refactor Discipline: search procedure, three permitted outcomes (reuse as-is / reuse via refactor / do not reuse), opt-out variant, single-line `Reuse:` recording format, and the verbatim loader phrase consumer skills must invoke. Includes one worked example per outcome.
- **Files changed:** skills/refactor-assessment.md
- **Decisions:** Canonical outcome token set = `as-is|via-refactor|none|opt-out|n/a`. Canonical loader phrase: "Invoke skills/refactor-assessment.md before writing any new code." Recording line shape: `Reuse: <token> — <reason>` placed in commit trailer or design-artefact note. These are normative — tasks 2–5 use them verbatim.

## Remaining Tasks

- [ ] #573 — Integrate Refactor Assessment into skills/feature-design.md ← current
- [ ] #574 — Integrate Reuse & Refactor Check into skills/dev-session.md
- [ ] #575 — Integrate Reuse & Refactor Check into skills/issue-session.md
- [ ] #576 — Add universal Reuse & Refactor Discipline rule to RULEBOOK.md
- [ ] #577 — Verify frontmatter, rebuild CATALOGUE.md, confirm reference-skill classification
