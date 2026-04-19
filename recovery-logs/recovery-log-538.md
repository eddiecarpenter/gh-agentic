# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #538                               |
| Branch              | feature/538-strip-v1-v2-vocabulary |
| Last commit         | 3bc6f19                            |
| Total tasks         | 8                                  |
| Last updated        | 2026-04-19T03:56:23+00:00          |

## Completed Tasks

### #539 — Rename internal/initv2 to internal/init; strip V2 vocabulary from package
- **Implemented:** `git mv`; `package initv2` → `package init`; callers import as `initpkg`.
- **Files changed:** internal/init/*.go, internal/cli/{init,repair}.go, internal/project/{init,init_test,guards,scope}.go, internal/scope/scope.go
- **Decisions:** `initpkg` import alias (Go init function identifier collision).

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
- **Implemented:** Both files describe only the current single design.
- **Files changed:** docs/PROJECT_BRIEF.md, README.md

### #544 — Resolve docs/TUI_DESIGN.md — delete it or strip the legacy v1 framing
- **Implemented:** Deleted. Purely v1 prototype history. Updated docs/ARCHITECTURE.md (removed tree entry) and internal/ui/styles.go (two stale comments).
- **Files changed:** deleted docs/TUI_DESIGN.md; docs/ARCHITECTURE.md, internal/ui/styles.go

### #545 — Audit CATALOGUE.md and LOCALRULES.md for v1/v2 vocabulary
- **Implemented:** CATALOGUE.md required no changes — all remaining hits are `GitHub Project V2` GraphQL type, allow-listed. LOCALRULES.md's Commands table was replaced: the retired `gh agentic bootstrap` / `inception` / `sync` entries are gone, replaced with the current subcommand surface. The `github.com/cli/go-gh/v2` library path remains — allow-listed.
- **Files changed:** LOCALRULES.md

## Remaining Tasks

- [ ] #546 — Final verification ← current
