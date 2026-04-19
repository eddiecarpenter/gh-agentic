# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #538                               |
| Branch              | feature/538-strip-v1-v2-vocabulary |
| Last commit         | 941a039                            |
| Total tasks         | 8                                  |
| Last updated        | 2026-04-19T03:44:04+00:00          |

## Completed Tasks

### #539 — Rename internal/initv2 to internal/init; strip V2 vocabulary from package
- **Implemented:** Renamed the package directory via `git mv`, changed every `package initv2` declaration to `package init`, updated callers (`internal/cli/init.go`, `internal/cli/repair.go`, `internal/project/init.go`, `internal/project/init_test.go`) to import `initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"` and qualified identifiers `initpkg.X`. Cleaned up v2 vocabulary in comments and the `gh agentic -v2 doctor` string in `wizard.go`.
- **Files changed:** internal/init/*.go (renamed from initv2/), internal/cli/init.go, internal/cli/repair.go, internal/project/init.go, internal/project/init_test.go, internal/project/guards.go, internal/project/scope.go, internal/scope/scope.go
- **Decisions:** Chose `initpkg` as the import alias (task permitted it — `init` collides semantically with Go's reserved init function identifier). All subsequent tasks that reference the renamed package must use the same alias.

## Remaining Tasks

- [ ] #540 — Rename internal/doctorv2 → internal/doctor ← current
- [ ] #541 — Strip -v2/--v2 from mount templates, CLI strings, tests
- [ ] #542 — Rewrite docs/ARCHITECTURE.md
- [ ] #543 — Clean docs/PROJECT_BRIEF.md and README.md
- [ ] #544 — Resolve docs/TUI_DESIGN.md
- [ ] #545 — Audit CATALOGUE.md and LOCALRULES.md
- [ ] #546 — Final verification
