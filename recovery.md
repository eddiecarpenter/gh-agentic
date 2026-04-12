# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #418                               |
| Branch              | feature/418-v2-self-mounting-framework |
| Last commit         | 9be8ea7                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-12T09:58:00Z               |

## Completed Tasks

### #419 — Add -v2 persistent flag routing and deprecated command blocking
- **Implemented:** Added -v2 persistent flag to root cobra command with v2 guard checks on v1 commands (sync, verify, bootstrap, inception). Registered v2 stub commands (mount, init, auth, doctor-v2).
- **Files changed:** internal/cli/root.go, internal/cli/v2.go, internal/cli/v2_test.go, internal/cli/sync.go, internal/cli/bootstrap.go, internal/cli/inception.go, internal/cli/doctor.go
- **Decisions:** v2 stub commands require -v2 flag to execute; doctor-v2 is hidden to avoid conflict with existing doctor command

## Remaining Tasks

- [ ] #420 — Implement mount command — tag validation and framework download
- [ ] #421 — Implement mount command — first-time flow (no .ai-version)
- [ ] #422 — Implement mount command — version switch and remount flows
- [ ] #423 — Implement auth command (login, refresh, check)
- [ ] #424 — Implement v2 doctor command with grouped output
- [ ] #425 — Implement init command — interactive wizard
- [ ] #426 — Create reusable GitHub Actions workflow with mount step
- [ ] #427 — Add v2 bootstrap rule to AGENTS.md template
