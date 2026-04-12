# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #418                               |
| Branch              | feature/418-v2-self-mounting-framework |
| Last commit         | 383df78                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-12T10:35:00Z               |

## Completed Tasks

### #419 — Add -v2 persistent flag routing and deprecated command blocking
- **Implemented:** Added -v2 persistent flag with v2 guard checks on v1 commands.
- **Files changed:** internal/cli/root.go, v2.go, v2_test.go, sync.go, bootstrap.go, inception.go, doctor.go
- **Decisions:** None

### #420 — Implement mount command — tag validation and framework download
- **Implemented:** Core mount logic: tag validation, framework download/extraction, .ai-version, .gitignore.
- **Files changed:** internal/mount/deps.go, mount.go, mount_test.go
- **Decisions:** None

### #421 — Implement mount command — first-time flow (no .ai-version)
- **Implemented:** First-time mount generating CLAUDE.md, AGENTS.md, workflows, .ai-version, .gitignore.
- **Files changed:** internal/mount/firsttime.go, firsttime_test.go, templates.go, internal/cli/mount.go, mount_test.go
- **Decisions:** None

### #422 — Implement mount command — version switch and remount flows
- **Implemented:** Version switch with confirmation and workflow tag update. Silent remount.
- **Files changed:** internal/mount/switch.go, switch_test.go, remount.go, remount_test.go
- **Decisions:** Regex-based workflow tag replacement

### #423 — Implement auth command (login, refresh, check)
- **Implemented:** Auth login/refresh/check with credential expiry parsing and upload.
- **Files changed:** internal/auth/auth.go, auth_test.go, internal/cli/auth.go, auth_test.go
- **Decisions:** None

## Remaining Tasks

- [ ] #424 — Implement v2 doctor command with grouped output
- [ ] #425 — Implement init command — interactive wizard
- [ ] #426 — Create reusable GitHub Actions workflow with mount step
- [ ] #427 — Add v2 bootstrap rule to AGENTS.md template
