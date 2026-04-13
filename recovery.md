# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #429                               |
| Branch              | feature/429-v2-cli-commands        |
| Last commit         | 18f34d2                            |
| Total tasks         | 2                                  |
| Last updated        | 2026-04-13T00:00:00Z               |

## Completed Tasks

### #439 — feat: implement init CollectConfig interactive form
- **Implemented:** Interactive huh form for `gh agentic -v2 init` that collects all InitConfig fields with auto-detection of repo and owner type. Wired real CollectConfigInteractive into production newInitCmd().
- **Files changed:** internal/cli/init.go, internal/initv2/form.go, internal/initv2/form_test.go
- **Decisions:** None

### #440 — feat: add deprecation notices to v1 commands
- **Implemented:** Deprecation warnings for sync, bootstrap, inception (to stderr) and doctor --update-credentials (to output writer). Uses consistent format via printDeprecationNotice helper with ui.RenderWarning styling. Does not block execution.
- **Files changed:** internal/cli/v2.go, internal/cli/sync.go, internal/cli/bootstrap.go, internal/cli/inception.go, internal/cli/doctor.go, internal/cli/v2_test.go, internal/cli/sync_test.go, internal/cli/bootstrap_test.go, internal/cli/inception_test.go, internal/cli/doctor_test.go
- **Decisions:** None

## Remaining Tasks

(none — all tasks complete)
