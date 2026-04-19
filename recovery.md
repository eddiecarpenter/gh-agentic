# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #538                               |
| Branch              | feature/538-strip-v1-v2-vocabulary |
| Last commit         | 2e804f4                            |
| Total tasks         | 8                                  |
| Last updated        | 2026-04-19T03:51:25+00:00          |

## Completed Tasks

### #539 — Rename internal/initv2 to internal/init; strip V2 vocabulary from package
- **Implemented:** Renamed directory via `git mv`; `package initv2` → `package init`; callers import as `initpkg`.
- **Files changed:** internal/init/*.go, internal/cli/{init,repair}.go, internal/project/{init,init_test,guards,scope}.go, internal/scope/scope.go
- **Decisions:** Chose `initpkg` as the import alias (`init` collides with Go's init function).

### #540 — Rename internal/doctorv2 to internal/doctor; strip V2 vocabulary from package
- **Implemented:** Renamed directory via `git mv`; `package doctorv2` → `package doctor`; callers updated.
- **Files changed:** internal/doctor/*.go, internal/cli/check.go, internal/cli/repair.go
- **Decisions:** No alias needed.

### #541 — Strip -v2/--v2 vocabulary from mount templates, CLI strings, comments, and tests
- **Implemented:** `gh agentic --v2 mount` → `gh agentic mount` across templates, .tmpl file, and tests. Package doc-strings cleaned.
- **Files changed:** internal/mount/{templates,templates_test,firsttime_test,deps}.go, internal/mount/templates/AGENTS.md.tmpl, internal/auth/auth.go, internal/cli/auth.go
- **Decisions:** Left legitimate `v1.5.0+` migration references in `internal/tarball/tarball.go` untouched.

### #542 — Rewrite docs/ARCHITECTURE.md as a single coherent design
- **Implemented:** Collapsed v1/v2 command tables into a single Commands table; renamed "v2 Mount model" and "v2 Auth subcommands" headings; rewrote the package-structure tree to match the current codebase (internal/init/, internal/doctor/, and removed retired v1 entries); stripped `-v2` / `--v2` references from Layering rules and Key dependencies; removed the deprecation narrative. TUI_DESIGN.md listed without `(legacy)` label (task #544 handles that file).
- **Files changed:** docs/ARCHITECTURE.md
- **Decisions:** None — rewrite was bounded to ARCHITECTURE.md.

## Remaining Tasks

- [ ] #543 — Clean docs/PROJECT_BRIEF.md and README.md ← current
- [ ] #544 — Resolve docs/TUI_DESIGN.md
- [ ] #545 — Audit CATALOGUE.md and LOCALRULES.md
- [ ] #546 — Final verification
