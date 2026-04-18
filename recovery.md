# Recovery State

| Field               | Value                                           |
|---------------------|-------------------------------------------------|
| Feature issue       | #483                                            |
| Branch              | feature/483-skill-creation-ask-user             |
| Last commit         | cf5c7a4                                         |
| Total tasks         | 6                                               |
| Last updated        | 2026-04-18T09:45:00Z                            |

## Completed Tasks

### #484 — Create skills/ask-user.md
- **Implemented:** New Reference skill defining harness-neutral interaction shapes.
- **Files changed:** skills/ask-user.md
- **Decisions:** None

### #485 — Extend skills/skill-categories.md with per-category Skeleton subsections
- **Implemented:** Added "Category Skeletons" section with six skeletons.
- **Files changed:** skills/skill-categories.md
- **Decisions:** None

### #486 — Create skills/skill-creation.md
- **Implemented:** New Operation skill with reactive and proactive procedures, classification rubric, placement logic, name-collision handling, and feedback-block variants.
- **Files changed:** skills/skill-creation.md
- **Decisions:** Operation category → `emits-exit-block: false`, `exit-hands-to: null` per schema; the "feedback block" is described inline, not a session exit block.

### #487 — Update skills/feature-scoping.md to invoke skills/ask-user.md
- **Implemented:** Every artefact confirmation (raw idea, problem, feature+user story, MVP, parallel/serial, AC, UX, deployment strategy, parking lot) and the explicit trigger confirmation now delegate to `skills/ask-user.md`. Deployment-strategy downstream steps (mode, flag name, reason for no-switch) also delegate. Added `ask-user` to loads. Added a Rules entry making the delegation explicit.
- **Files changed:** skills/feature-scoping.md
- **Decisions:** Left the pre-existing "Open Goose and select the Feature Scoping (Stage 2) recipe" line untouched — task 487 AC requires not *introducing* specific tool/harness names; it does not ask to remove pre-existing mentions, which is out of scope.

## Remaining Tasks

- [ ] #488 — Add proactive skill-suggestion rule to RULEBOOK.md (≤ ~2 lines) ← current
- [ ] #489 — End-to-end verification — reactive, proactive, placement, collision, session-init self-heal, harness behaviour
