# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #571                               |
| Branch              | feature/571-centralised-project-context |
| Last commit         | d43a798                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-19T10:24:00Z               |

## Completed Tasks

### #579 — Audit and publish call-site inventory
- **Implemented:** Published `docs/refactor-audit-571.md` — the authoritative inventory of every direct `AGENTIC_*` read outside `internal/project/`, every `.ai-version` read/write across Go, workflows, templates, skills, and docs. Each entry names its target resolver field for tasks #580–#587 to reference.
- **Files changed:** docs/refactor-audit-571.md
- **Decisions:** `internal/project/` and `internal/mount/` are both inside the canonical boundary for #571 (resolver + `.ai-version` I/O); everything else migrates. The proposed resolver API consolidates `ResolveState` + `ResolveTopology` into one `project.Resolve` / `project.Context` entry point.

## Remaining Tasks

- [ ] #580 — Introduce unified project.Context resolver (refactor ResolveTopology + ResolveState) ← current
- [ ] #581 — Migrate mount command to project.Resolve
- [ ] #582 — Migrate check and repair commands to project.Resolve
- [ ] #583 — Migrate status, info, auth, upgrade, init commands to project.Resolve
- [ ] #584 — Enforce boundary: fail CI on direct AGENTIC_* reads outside internal/project/
- [ ] #585 — Remove .ai-version file and local AGENTIC_TOPOLOGY gate
- [ ] #586 — Sweep skills, docs, and README to remove .ai-version and stopgap references
- [ ] #587 — Regression test: charging-domain scenario passes without AGENTIC_TOPOLOGY stopgap
