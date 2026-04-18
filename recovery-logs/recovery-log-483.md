# Recovery State

| Field               | Value                                           |
|---------------------|-------------------------------------------------|
| Feature issue       | #483                                            |
| Branch              | feature/483-skill-creation-ask-user             |
| Last commit         | bf94c06                                         |
| Total tasks         | 6                                               |
| Last updated        | 2026-04-18T09:50:00Z                            |

## Completed Tasks

### #484 — Create skills/ask-user.md
- **Implemented:** New Reference skill defining harness-neutral interaction shapes.
- **Files changed:** skills/ask-user.md

### #485 — Extend skills/skill-categories.md with per-category Skeleton subsections
- **Implemented:** Added "Category Skeletons" section with six skeletons.
- **Files changed:** skills/skill-categories.md

### #486 — Create skills/skill-creation.md
- **Implemented:** New Operation skill with reactive and proactive procedures.
- **Files changed:** skills/skill-creation.md

### #487 — Update skills/feature-scoping.md to invoke skills/ask-user.md
- **Implemented:** All confirmation/selection moments now delegate to `skills/ask-user.md`; `ask-user` added to loads; new Rules entry makes the delegation explicit.
- **Files changed:** skills/feature-scoping.md

### #488 — Add proactive skill-suggestion rule to RULEBOOK.md
- **Implemented:** Added a 3-line bullet under Working Principles pointing at `skills/skill-creation.md` in proactive-suggestion mode. No thresholds, rubric, placement, or collision detail leaks into RULEBOOK.
- **Files changed:** RULEBOOK.md
- **Decisions:** Rule placed under the existing Working Principles section adjacent to the pipeline-label exit rule, since both govern session-time agent behaviour.

## Remaining Tasks

- [ ] #489 — End-to-end verification — reactive, proactive, placement, collision, session-init self-heal, harness behaviour ← current
