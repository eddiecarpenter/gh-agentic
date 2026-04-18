# Recovery State

| Field               | Value                                           |
|---------------------|-------------------------------------------------|
| Feature issue       | #483                                            |
| Branch              | feature/483-skill-creation-ask-user             |
| Last commit         | 455e01b                                         |
| Total tasks         | 6                                               |
| Last updated        | 2026-04-18T09:35:00Z                            |

## Completed Tasks

### #484 — Create skills/ask-user.md — canonical Reference skill for user-interaction prompts
- **Implemented:** New Reference skill defining harness-neutral interaction shapes (confirm/revise, multi-choice, yes/no/later, name-collision) with option constraints, fallback phrasing, and invocation rules. No tool or harness names anywhere in the body.
- **Files changed:** skills/ask-user.md
- **Decisions:** None

### #485 — Extend skills/skill-categories.md with per-category Skeleton subsections
- **Implemented:** Added "Category Skeletons" section with six skeletons (Session, Recovery, Bootstrap, Operation, Information, Reference). Session/Recovery include exit-block placeholders; Operation includes a feedback block; others do not.
- **Files changed:** skills/skill-categories.md
- **Decisions:** None

### #486 — Create skills/skill-creation.md — reactive + proactive skill creator (Operation)
- **Implemented:** New Operation skill with reactive and proactive procedures. Delegates every confirmation/classification/disambiguation/collision to ask-user. Includes rubric, placement logic (framework vs local by origin remote), name-collision handling, feedback block variants. `emits-exit-block: false` to align with Operation-category validator rules; feedback block is described inline (not the session exit block).
- **Files changed:** skills/skill-creation.md
- **Decisions:** Operation category → `emits-exit-block: false`, `exit-hands-to: null` per schema validator rules. The "feedback block" referenced in the feature UX-3 is described inline and emitted on completion, but is not the session-ending exit block from skills/session-exit.md.

## Remaining Tasks

- [ ] #487 — Update skills/feature-scoping.md to invoke skills/ask-user.md at every confirmation/selection moment ← current
- [ ] #488 — Add proactive skill-suggestion rule to RULEBOOK.md (≤ ~2 lines)
- [ ] #489 — End-to-end verification — reactive, proactive, placement, collision, session-init self-heal, harness behaviour
