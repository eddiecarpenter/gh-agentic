# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #418                               |
| Branch              | feature/418-v2-self-mounting-framework |
| Last commit         | b9620b5                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-12T10:55:00Z               |

## Completed Tasks

### #419 — Add -v2 persistent flag routing and deprecated command blocking
- **Implemented:** -v2 flag, v2 guard checks, stub commands
- **Files changed:** internal/cli/root.go, v2.go, v2_test.go, sync.go, bootstrap.go, inception.go, doctor.go
- **Decisions:** None

### #420 — Implement mount command — tag validation and framework download
- **Implemented:** Core mount: tag validation, download/extraction, .ai-version, .gitignore
- **Files changed:** internal/mount/deps.go, mount.go, mount_test.go
- **Decisions:** None

### #421 — Implement mount command — first-time flow (no .ai-version)
- **Implemented:** First-time mount with CLAUDE.md, AGENTS.md, workflows, templates
- **Files changed:** internal/mount/firsttime.go, firsttime_test.go, templates.go, internal/cli/mount.go, mount_test.go
- **Decisions:** None

### #422 — Implement mount command — version switch and remount flows
- **Implemented:** Switch with confirmation, remount silent, workflow tag update
- **Files changed:** internal/mount/switch.go, switch_test.go, remount.go, remount_test.go
- **Decisions:** None

### #423 — Implement auth command (login, refresh, check)
- **Implemented:** Auth login/refresh/check with credential expiry parsing
- **Files changed:** internal/auth/auth.go, auth_test.go, internal/cli/auth.go, auth_test.go
- **Decisions:** None

### #424 — Implement v2 doctor command with grouped output
- **Implemented:** Grouped health checks with ✓/⚠/✗ output
- **Files changed:** internal/doctorv2/checks.go, checks_test.go, groups.go, internal/cli/doctorv2.go, doctorv2_test.go
- **Decisions:** None

### #425 — Implement init command — interactive wizard
- **Implemented:** Init wizard with CollectConfig, mount, configure secrets/variables/collaborator
- **Files changed:** internal/initv2/wizard.go, wizard_test.go, internal/cli/init.go, init_test.go
- **Decisions:** None

## Remaining Tasks

- [ ] #426 — Create reusable GitHub Actions workflow with mount step
- [ ] #427 — Add v2 bootstrap rule to AGENTS.md template
