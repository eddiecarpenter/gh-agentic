# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #538                               |
| Branch              | feature/538-strip-v1-v2-vocabulary |
| Last commit         | 8295cf9                            |
| Total tasks         | 8                                  |
| Last updated        | 2026-04-19T03:53:09+00:00          |

## Completed Tasks

### #539 — Rename internal/initv2 to internal/init; strip V2 vocabulary from package
- **Implemented:** `git mv`; `package initv2` → `package init`; callers import as `initpkg`.
- **Files changed:** internal/init/*.go, internal/cli/{init,repair}.go, internal/project/{init,init_test,guards,scope}.go, internal/scope/scope.go
- **Decisions:** `initpkg` import alias (init function identifier collision).

### #540 — Rename internal/doctorv2 to internal/doctor; strip V2 vocabulary from package
- **Implemented:** `git mv`; `package doctorv2` → `package doctor`.
- **Files changed:** internal/doctor/*.go, internal/cli/check.go, internal/cli/repair.go

### #541 — Strip -v2/--v2 vocabulary from mount templates, CLI strings, comments, and tests
- **Implemented:** `gh agentic --v2 mount` → `gh agentic mount`; package doc-strings cleaned.
- **Files changed:** internal/mount/*, internal/auth/auth.go, internal/cli/auth.go
- **Decisions:** Left legitimate `v1.5.0+` migration references in `internal/tarball/tarball.go`.

### #542 — Rewrite docs/ARCHITECTURE.md as a single coherent design
- **Implemented:** Single Commands table; renamed "v2 Mount model" → "Mount model"; rewrote package-structure tree; removed deprecation narrative.
- **Files changed:** docs/ARCHITECTURE.md

### #543 — Clean docs/PROJECT_BRIEF.md and README.md of v1/v2 framing
- **Implemented:** Both files now describe only the current single design. Collapsed v1/v2 command tables into a single Commands list in each file. Replaced `gh agentic -v2 init` / `-v2 doctor-v2` strings with their plain form. Removed the README Legacy notice, the v1-deprecated section, and the 'Getting started (v2)' heading. Dropped 'In v2' framing and 'required for v2' wording from PROJECT_BRIEF.md. Verified `grep -nE '(v1|v2|V1|V2) command|deprecated|-v2|--v2|doctor-v2'` returns no hits.
- **Files changed:** docs/PROJECT_BRIEF.md, README.md

## Remaining Tasks

- [ ] #544 — Resolve docs/TUI_DESIGN.md ← current
- [ ] #545 — Audit CATALOGUE.md and LOCALRULES.md
- [ ] #546 — Final verification
