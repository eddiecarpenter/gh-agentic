# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #418                               |
| Branch              | feature/418-v2-self-mounting-framework |
| Last commit         | 9cd4e44                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-12T10:25:00Z               |

## Completed Tasks

### #419 — Add -v2 persistent flag routing and deprecated command blocking
- **Implemented:** Added -v2 persistent flag with v2 guard checks on v1 commands.
- **Files changed:** internal/cli/root.go, internal/cli/v2.go, internal/cli/v2_test.go, internal/cli/sync.go, internal/cli/bootstrap.go, internal/cli/inception.go, internal/cli/doctor.go
- **Decisions:** None

### #420 — Implement mount command — tag validation and framework download
- **Implemented:** Core mount logic: tag validation, framework download/extraction, .ai-version read/write, .gitignore management.
- **Files changed:** internal/mount/deps.go, internal/mount/mount.go, internal/mount/mount_test.go
- **Decisions:** Framework prefixes: RULEBOOK.md, skills/, recipes/, standards/, concepts/

### #421 — Implement mount command — first-time flow (no .ai-version)
- **Implemented:** First-time mount flow generating all files. CLI mount command with flow routing.
- **Files changed:** internal/mount/firsttime.go, internal/mount/firsttime_test.go, internal/mount/templates.go, internal/cli/mount.go, internal/cli/mount_test.go
- **Decisions:** Preserve existing CLAUDE.md/AGENTS.md if present

### #422 — Implement mount command — version switch and remount flows
- **Implemented:** Version switch with confirmation prompt, workflow tag update via regex. Remount silently re-downloads at same version.
- **Files changed:** internal/mount/switch.go, internal/mount/switch_test.go, internal/mount/remount.go, internal/mount/remount_test.go
- **Decisions:** Workflow tag update uses regex replacement rather than full file regeneration

## Remaining Tasks

- [ ] #423 — Implement auth command (login, refresh, check)
- [ ] #424 — Implement v2 doctor command with grouped output
- [ ] #425 — Implement init command — interactive wizard
- [ ] #426 — Create reusable GitHub Actions workflow with mount step
- [ ] #427 — Add v2 bootstrap rule to AGENTS.md template
