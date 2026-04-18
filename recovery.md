# Recovery State

| Field               | Value                                           |
|---------------------|-------------------------------------------------|
| Feature issue       | #483                                            |
| Branch              | feature/483-skill-creation-ask-user             |
| Last commit         | bda6e15                                         |
| Total tasks         | 6                                               |
| Last updated        | 2026-04-18T09:25:00Z                            |

## Completed Tasks

### #484 — Create skills/ask-user.md — canonical Reference skill for user-interaction prompts
- **Implemented:** New Reference skill defining harness-neutral interaction shapes (confirm/revise, multi-choice, yes/no/later, name-collision) with option constraints, fallback phrasing, and invocation rules. No tool or harness names anywhere in the body.
- **Files changed:** skills/ask-user.md
- **Decisions:** None

### #485 — Extend skills/skill-categories.md with per-category Skeleton subsections
- **Implemented:** Added "Category Skeletons" section between "Consumers of this File" and the terminal "Rules" section. Six skeletons — Session, Recovery, Bootstrap, Operation, Information, Reference — each with the minimal frontmatter block plus the body headings required for that category. Session/Recovery skeletons include an exit-block placeholder; Operation includes a feedback block; others do not.
- **Files changed:** skills/skill-categories.md
- **Decisions:** None

## Remaining Tasks

- [ ] #486 — Create skills/skill-creation.md — reactive + proactive skill creator (Operation) ← current
- [ ] #487 — Update skills/feature-scoping.md to invoke skills/ask-user.md at every confirmation/selection moment
- [ ] #488 — Add proactive skill-suggestion rule to RULEBOOK.md (≤ ~2 lines)
- [ ] #489 — End-to-end verification — reactive, proactive, placement, collision, session-init self-heal, harness behaviour
