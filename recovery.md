# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #538                               |
| Branch              | feature/538-strip-v1-v2-vocabulary |
| Last commit         | d048d9d                            |
| Total tasks         | 8                                  |
| Last updated        | 2026-04-19T03:49:33+00:00          |

## Completed Tasks

### #539 — Rename internal/initv2 to internal/init; strip V2 vocabulary from package
- **Implemented:** Renamed the package directory via `git mv`, changed every `package initv2` declaration to `package init`, updated callers to import `initpkg "github.com/eddiecarpenter/gh-agentic/internal/init"` and qualified identifiers `initpkg.X`. Cleaned v2 vocabulary in comments and the `gh agentic -v2 doctor` string.
- **Files changed:** internal/init/*.go (renamed), internal/cli/{init,repair}.go, internal/project/{init,init_test,guards,scope}.go, internal/scope/scope.go
- **Decisions:** Chose `initpkg` as the import alias (`init` collides semantically with Go's init function identifier).

### #540 — Rename internal/doctorv2 to internal/doctor; strip V2 vocabulary from package
- **Implemented:** Renamed via `git mv`, changed `package doctorv2` to `package doctor`, updated imports in `internal/cli/{check,repair}.go`, rewrote qualified identifiers. Cleaned v2 doc-string vocabulary. `ProjectV2` (GitHub GraphQL type) untouched per allow-list.
- **Files changed:** internal/doctor/*.go (renamed), internal/cli/check.go, internal/cli/repair.go
- **Decisions:** No alias needed — `doctor` does not collide with any Go identifier.

### #541 — Strip -v2/--v2 vocabulary from mount templates, CLI strings, comments, and tests
- **Implemented:** Replaced `gh agentic --v2 mount` with `gh agentic mount` in the embedded AGENTS.md template, the corresponding .tmpl file, and the tests that assert on it. Removed `for v2` / `v2 credential` language from package doc-strings in `internal/mount/deps.go` and `internal/auth/auth.go`. Updated the `--v2 auth` comment on `newAuthCmd`.
- **Files changed:** internal/mount/templates.go, internal/mount/templates/AGENTS.md.tmpl, internal/mount/templates_test.go, internal/mount/firsttime_test.go, internal/mount/deps.go, internal/auth/auth.go, internal/cli/auth.go
- **Decisions:** Left `v1.5.0+` historical migration references in `internal/tarball/tarball.go` untouched — these are semantic-version references to a specific older template layout, not v1/v2 framework vocabulary.

## Remaining Tasks

- [ ] #542 — Rewrite docs/ARCHITECTURE.md ← current
- [ ] #543 — Clean docs/PROJECT_BRIEF.md and README.md
- [ ] #544 — Resolve docs/TUI_DESIGN.md
- [ ] #545 — Audit CATALOGUE.md and LOCALRULES.md
- [ ] #546 — Final verification
