# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #468                               |
| Branch              | feature/468-skill-taxonomy-catalogue-exit-protocol |
| Last commit         | (pending)                          |
| Total tasks         | 11                                 |
| Last updated        | 2026-04-18T02:50:00Z               |

## Completed Tasks

### #469 — Verify Anthropic Claude Skills canonical frontmatter spec
- **Implemented:** Verified Anthropic's current Claude Skills spec via live docs fetch (platform.claude.com overview + best-practices, code.claude.com). Confirmed `name` and `description` remain the only canonical required fields — no drift since January 2026 scoping. Posted findings with field rules and handover guidance for Task #470 as a comment on #468.
- **Files changed:** No code changes. Comment posted on Feature #468.
- **Decisions:** Keep the scoped schema as-is. Do not adopt Claude Code-specific extensions (`allowed-tools`, `when_to_use`, etc.) — framework-specific fields (`category`, `triggers`, `loads`, `emits-exit-block`, `exit-hands-to`) live alongside the canonical two without overloading them.

## Remaining Tasks

- [ ] #470 — Create skills/skill-categories.md defining the six-category taxonomy and frontmatter schema ← current
- [ ] #471 — Create skills/session-exit.md with canonical exit block templates
- [ ] #472 — Update RULEBOOK.md with minimal taxonomy pointer and session-termination rule
- [ ] #473 — Add YAML frontmatter to every existing skill and classify each into a category
- [ ] #474 — Roll out the universal exit block across all session-ending skills
- [ ] #475 — Create skills/build-catalogue.md defining the CATALOGUE.md regeneration procedure
- [ ] #476 — Generate initial CATALOGUE.md from the now-classified skills
- [ ] #477 — Extend gh agentic check with frontmatter validation and catalogue status reporting
- [ ] #478 — Update session-init.md with self-healing catalogue detection and lazy skill loading
- [ ] #479 — End-to-end verification: check passes, catalogue self-heals, exit block emits
