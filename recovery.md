# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #430                               |
| Branch              | feature/430-update-v2-docs         |
| Last commit         | 0687902                            |
| Total tasks         | 4                                  |
| Last updated        | 2026-04-13T00:44:00Z               |

## Completed Tasks

### #442 — Update docs/ARCHITECTURE.md for v2 mount model and self-relative paths
- **Implemented:** Rewrote ARCHITECTURE.md to document v2 package structure, mount model (.ai-version, .ai/ gitignore, tarball fetch, reusable workflows), self-relative paths, credential management, and v2/v1 commands table with deprecation notes. Removed REPOS.md references.
- **Files changed:** docs/ARCHITECTURE.md
- **Decisions:** None

### #443 — Update docs/PROJECT_BRIEF.md for v2 goals and design decisions
- **Implemented:** Rewrote PROJECT_BRIEF.md to reflect merged repo model (gh-agentic as both CLI and framework source), v2 commands table with v1 deprecation notes, v2 init wizard flow replacing bootstrap/inception, updated separation of concerns. Removed REPOS.md references.
- **Files changed:** docs/PROJECT_BRIEF.md
- **Decisions:** None

## Remaining Tasks

- [ ] #444 — Update README.md with v2 getting started guide ← current
- [ ] #445 — Archive or remove TUI_DESIGN.md and clean up legacy doc references
