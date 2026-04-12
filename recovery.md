# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #418                               |
| Branch              | feature/418-v2-self-mounting-framework |
| Last commit         | 991d755                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-12T10:15:00Z               |

## Completed Tasks

### #419 — Add -v2 persistent flag routing and deprecated command blocking
- **Implemented:** Added -v2 persistent flag to root cobra command with v2 guard checks on v1 commands.
- **Files changed:** internal/cli/root.go, internal/cli/v2.go, internal/cli/v2_test.go, internal/cli/sync.go, internal/cli/bootstrap.go, internal/cli/inception.go, internal/cli/doctor.go
- **Decisions:** v2 stub commands require -v2 flag; doctor-v2 hidden to avoid conflict

### #420 — Implement mount command — tag validation and framework download
- **Implemented:** Created internal/mount package with core mount logic: tag validation, framework download/extraction, .ai-version read/write, .gitignore management.
- **Files changed:** internal/mount/deps.go, internal/mount/mount.go, internal/mount/mount_test.go
- **Decisions:** Framework prefixes: RULEBOOK.md, skills/, recipes/, standards/, concepts/. Existing .ai/ cleaned before mount.

### #421 — Implement mount command — first-time flow (no .ai-version)
- **Implemented:** First-time mount flow generating CLAUDE.md, AGENTS.md with bootstrap rule, wrapper workflows, .ai-version, .gitignore. CLI mount command with flow routing. Stub remount/switch for task #422.
- **Files changed:** internal/mount/firsttime.go, internal/mount/firsttime_test.go, internal/mount/templates.go, internal/mount/remount.go, internal/mount/switch.go, internal/cli/mount.go, internal/cli/mount_test.go
- **Decisions:** Preserve existing CLAUDE.md/AGENTS.md if present. Wrapper workflows reference reusable workflow via uses: tag.

## Remaining Tasks

- [ ] #422 — Implement mount command — version switch and remount flows
- [ ] #423 — Implement auth command (login, refresh, check)
- [ ] #424 — Implement v2 doctor command with grouped output
- [ ] #425 — Implement init command — interactive wizard
- [ ] #426 — Create reusable GitHub Actions workflow with mount step
- [ ] #427 — Add v2 bootstrap rule to AGENTS.md template
