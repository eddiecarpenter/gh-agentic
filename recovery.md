# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #571                               |
| Branch              | feature/571-centralised-project-context |
| Last commit         | 5193a7a                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-19T10:52:00Z               |

## Completed Tasks

### #579 — Audit and publish call-site inventory
- **Implemented:** Published `docs/refactor-audit-571.md` — authoritative inventory of every `AGENTIC_*` read outside `internal/project/` and every `.ai-version` read/write.
- **Files changed:** docs/refactor-audit-571.md
- **Decisions:** `internal/project/` and `internal/mount/` are inside the canonical boundary; everything else migrates.

### #580 — Introduce unified project.Context resolver
- **Implemented:** `project.Resolve(deps) (*project.Context, error)` in `internal/project/context.go`. Context exposes Topology, Role, LinkedRepos, ControlPlane, FrameworkVersion (per-topology), LocalAIVersion, VersionInSync, plus role helpers. `ResolveTopology` and `ResolveState` retained as thin wrappers.
- **Files changed:** internal/project/context.go, internal/project/context_test.go, internal/project/info.go, internal/project/topology.go
- **Decisions:** Wrappers preserved for incremental migration; `ResolveState` keeps legacy graph-based `DetectTopology` semantics.

### #581 — Migrate mount command to project.Resolve
- **Implemented:** Replaced `resolveMountVersion`, `detectFederatedCP`, `resolveFederatedCP` in `internal/cli/mount.go` with calls to `project.Resolve`. Added `resolveContextForMount(root)` so the version resolver and CP resolver share one read path.
- **Files changed:** internal/cli/mount.go
- **Decisions:** `.ai-version` fallback retained until #585.

### #582 — Migrate check and repair commands to project.Resolve
- **Implemented:** Replaced the `runGetVariable + checkGetRepoVariable + ResolveTopology` dance in `check.go` RunE and `repair.go:buildPipelineCheckDeps` with single `project.Resolve` calls. Returned Context feeds `doctor.CheckDeps` (Topology, ProjectID). Removed the now-dead `runGetVariable` and `checkGetRepoVariable` helpers. `go build ./...` and `go test ./...` pass.
- **Files changed:** internal/cli/check.go, internal/cli/repair.go
- **Decisions:** None beyond the pattern set by #580.

## Remaining Tasks

- [ ] #583 — Migrate status, info, auth, upgrade, init commands to project.Resolve ← current
- [ ] #584 — Enforce boundary: fail CI on direct AGENTIC_* reads outside internal/project/
- [ ] #585 — Remove .ai-version file and local AGENTIC_TOPOLOGY gate
- [ ] #586 — Sweep skills, docs, and README to remove .ai-version and stopgap references
- [ ] #587 — Regression test: charging-domain scenario passes without AGENTIC_TOPOLOGY stopgap
