# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #571                               |
| Branch              | feature/571-centralised-project-context |
| Last commit         | 8d97d9c                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-19T11:50:00Z               |

## Completed Tasks

### #579 — Audit and publish call-site inventory
- **Implemented:** `docs/refactor-audit-571.md`.

### #580 — Introduce unified project.Context resolver
- **Implemented:** `project.Resolve` + `project.Context` in `internal/project/`. Wrappers preserved.

### #581 — Migrate mount command to project.Resolve
- **Implemented:** mount.go routes through the resolver.

### #582 — Migrate check and repair commands to project.Resolve
- **Implemented:** check.go and repair.go use a single `project.Resolve` call.

### #583 — Migrate status, info, auth, upgrade, init commands to project.Resolve
- **Implemented:** status sub-commands, info, auth, init, project pickers, and doctor all route through the resolver.

### #584 — Enforce boundary: CI check for AGENTIC_* reads
- **Implemented:** `internal/project/boundary_test.go` scanner fails CI on forbidden identifier references outside `internal/project/`.

### #585 — Remove .ai-version file and AGENTIC_TOPOLOGY gate
- **Implemented:** deleted `.ai-version` I/O; removed auto-write of `AGENTIC_TOPOLOGY`; added `doctor/checks.go:checkTopologyStopgap` warning.

### #586 — Sweep skills, docs, and README
- **Implemented:** `skills/session-init.md` uses `gh agentic info` in place of `.ai-version`. `skills/gh-agentic-tool.md` describes resolver precedence. `README.md` drops the flat-file line and stale mount example. `docs/ARCHITECTURE.md` Mount model section rewritten around the resolver and `AGENTIC_FRAMEWORK_VERSION`. `docs/PROJECT_BRIEF.md` drops `.ai-version` from the init wizard's artifacts list. `grep -rn 'ai-version' skills/ docs/ README.md` now matches only the intentional `refactor-audit-571.md` inventory. `go build ./...` / `go test ./...` still pass.
- **Files changed:** skills/session-init.md, skills/gh-agentic-tool.md, README.md, docs/ARCHITECTURE.md, docs/PROJECT_BRIEF.md
- **Decisions:** Kept the audit doc's historical references intact — that file is the canonical inventory at time of migration.

## Remaining Tasks

- [ ] #587 — Regression test: charging-domain scenario passes without AGENTIC_TOPOLOGY stopgap ← current
