# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #538                               |
| Branch              | feature/538-strip-v1-v2-vocabulary |
| Last commit         | aefef05                            |
| Total tasks         | 8                                  |
| Last updated        | 2026-04-19T03:46:43+00:00          |

## Completed Tasks

### #539 — Rename internal/initv2 to internal/init; strip V2 vocabulary from package
- **Implemented:** Renamed the package directory via `git mv`, changed every `package initv2` declaration to `package init`, updated callers (`internal/cli/init.go`, `internal/cli/repair.go`, `internal/project/init.go`, `internal/project/init_test.go`) to import `initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"` and qualified identifiers `initpkg.X`. Cleaned up v2 vocabulary in comments and the `gh agentic -v2 doctor` string in `wizard.go`.
- **Files changed:** internal/init/*.go (renamed from initv2/), internal/cli/init.go, internal/cli/repair.go, internal/project/init.go, internal/project/init_test.go, internal/project/guards.go, internal/project/scope.go, internal/scope/scope.go
- **Decisions:** Chose `initpkg` as the import alias (task permitted it — `init` collides semantically with Go's reserved init function identifier). All subsequent tasks that reference the renamed package must use the same alias.

### #540 — Rename internal/doctorv2 to internal/doctor; strip V2 vocabulary from package
- **Implemented:** Renamed the package directory via `git mv`, changed every `package doctorv2` declaration to `package doctor`, updated import paths in `internal/cli/check.go` and `internal/cli/repair.go`, rewrote qualified identifiers from `doctorv2.X` to `doctor.X`. Cleaned v2 doc-string vocabulary on the package-level comment in `groups.go` and the `checkFramework` comment in `checks.go`. `ProjectV2` (GitHub GraphQL type) was left untouched per allow-list.
- **Files changed:** internal/doctor/*.go (renamed from doctorv2/), internal/cli/check.go, internal/cli/repair.go
- **Decisions:** No alias needed — `doctor` does not collide with any Go identifier.

## Remaining Tasks

- [ ] #541 — Strip -v2/--v2 from mount templates, CLI strings, tests ← current
- [ ] #542 — Rewrite docs/ARCHITECTURE.md
- [ ] #543 — Clean docs/PROJECT_BRIEF.md and README.md
- [ ] #544 — Resolve docs/TUI_DESIGN.md
- [ ] #545 — Audit CATALOGUE.md and LOCALRULES.md
- [ ] #546 — Final verification
