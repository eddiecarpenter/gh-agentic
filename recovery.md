# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #571                               |
| Branch              | feature/571-centralised-project-context |
| Last commit         | b66f3dd                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-19T10:36:00Z               |

## Completed Tasks

### #579 — Audit and publish call-site inventory
- **Implemented:** Published `docs/refactor-audit-571.md` — the authoritative inventory of every direct `AGENTIC_*` read outside `internal/project/`, every `.ai-version` read/write across Go, workflows, templates, skills, and docs. Each entry names its target resolver field for tasks #580–#587 to reference.
- **Files changed:** docs/refactor-audit-571.md
- **Decisions:** `internal/project/` and `internal/mount/` are both inside the canonical boundary for #571; everything else migrates. Proposed resolver API consolidates `ResolveState` + `ResolveTopology` into one `project.Resolve` / `project.Context` entry point.

### #580 — Introduce unified project.Context resolver
- **Implemented:** Added `project.Resolve(deps) (*project.Context, error)` in `internal/project/context.go` as the single canonical entry point. `Context` exposes topology (`single` / `federated-cp` / `federated-domain`), derived `Role` (`standalone` / `cp` / `domain`), project graph (LinkedRepos + ControlPlane), and authoritative `FrameworkVersion` resolved per-topology (reads `AGENTIC_FRAMEWORK_VERSION` from this repo for CP/single; from the CP repo for federated-domain). `ResolveTopology` and `ResolveState` are retained as thin wrappers that delegate to `Resolve` without breaking historic behaviour. Unit tests cover single, federated-cp, federated-domain, unaffiliated, deleted project, charging-domain regression, wrapper parity, and role helpers. `go build ./...` and `go test ./...` pass.
- **Files changed:** internal/project/context.go, internal/project/context_test.go, internal/project/info.go, internal/project/topology.go
- **Decisions:** Wrappers (`ResolveTopology` / `ResolveState`) preserved for incremental migration; they remain callable and go away in the final cleanup. `ResolveState` uses `DetectTopology(deps.RepoFullName, ctx.LinkedRepos)` (graph-based) to preserve the legacy `ProjectState.Topology` enum semantics that consumers still rely on.

## Remaining Tasks

- [ ] #581 — Migrate mount command to project.Resolve ← current
- [ ] #582 — Migrate check and repair commands to project.Resolve
- [ ] #583 — Migrate status, info, auth, upgrade, init commands to project.Resolve
- [ ] #584 — Enforce boundary: fail CI on direct AGENTIC_* reads outside internal/project/
- [ ] #585 — Remove .ai-version file and local AGENTIC_TOPOLOGY gate
- [ ] #586 — Sweep skills, docs, and README to remove .ai-version and stopgap references
- [ ] #587 — Regression test: charging-domain scenario passes without AGENTIC_TOPOLOGY stopgap
