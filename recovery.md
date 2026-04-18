# Recovery State

| Field               | Value                                           |
|---------------------|-------------------------------------------------|
| Feature issue       | #483                                            |
| Branch              | feature/483-skill-creation-ask-user             |
| Last commit         | 5eab5db                                         |
| Total tasks         | 6                                               |
| Last updated        | 2026-04-18T09:20:00Z                            |

## Completed Tasks

### #484 — Create skills/ask-user.md — canonical Reference skill for user-interaction prompts
- **Implemented:** New Reference skill defining harness-neutral interaction shapes (confirm/revise, multi-choice, yes/no/later, name-collision) with option constraints, fallback phrasing, and invocation rules. No tool or harness names anywhere in the body.
- **Files changed:** skills/ask-user.md
- **Decisions:** None

## Remaining Tasks

- [ ] #485 — Extend skills/skill-categories.md with per-category Skeleton subsections ← current
- [ ] #486 — Create skills/skill-creation.md — reactive + proactive skill creator (Operation)
- [ ] #487 — Update skills/feature-scoping.md to invoke skills/ask-user.md at every confirmation/selection moment
- [ ] #488 — Add proactive skill-suggestion rule to RULEBOOK.md (≤ ~2 lines)
- [ ] #489 — End-to-end verification — reactive, proactive, placement, collision, session-init self-heal, harness behaviour
