# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #571                               |
| Branch              | feature/571-centralised-project-context |
| Last commit         | ab8ec10                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-19T10:44:00Z               |

## Completed Tasks

### #579 — Audit and publish call-site inventory
- **Implemented:** Published `docs/refactor-audit-571.md` — authoritative inventory of every `AGENTIC_*` read outside `internal/project/` and every `.ai-version` read/write across Go, workflows, templates, skills, and docs. Each entry names its target resolver field.
- **Files changed:** docs/refactor-audit-571.md
- **Decisions:** `internal/project/` and `internal/mount/` are both inside the canonical boundary; everything else migrates.

### #580 — Introduce unified project.Context resolver
- **Implemented:** Added `project.Resolve(deps) (*project.Context, error)` in `internal/project/context.go` as the single canonical entry point. Context exposes Topology, Role, LinkedRepos, ControlPlane, FrameworkVersion (per-topology), LocalAIVersion, VersionInSync, plus IsFederatedControlPlane / IsFederatedDomain / IsSingle helpers. `ResolveTopology` and `ResolveState` retained as thin wrappers. Tests cover all topology paths, charging-domain regression, wrapper parity, and role helpers.
- **Files changed:** internal/project/context.go, internal/project/context_test.go, internal/project/info.go, internal/project/topology.go
- **Decisions:** Wrappers preserved for incremental migration. `ResolveState` preserves legacy graph-based `DetectTopology` semantics.

### #581 — Migrate mount command to project.Resolve
- **Implemented:** Replaced `resolveMountVersion`, `detectFederatedCP`, `resolveFederatedCP` in `internal/cli/mount.go` with calls to `project.Resolve`. Added private `resolveContextForMount(root)` helper so both the version resolver and CP resolver share one read path. Mount's public behaviour is unchanged; all existing mount tests (first-time, invalid-tag, federated CP sparse-checkout, non-federated) pass. No direct `AGENTIC_*` / `ProjectVarName` / `DefaultGetRepoVariable` references remain in `mount.go` (only docstring mentions of variable names).
- **Files changed:** internal/cli/mount.go
- **Decisions:** `.ai-version` fallback retained (`localVersionFallback`) until task #585 removes the file.

## Remaining Tasks

- [ ] #582 — Migrate check and repair commands to project.Resolve ← current
- [ ] #583 — Migrate status, info, auth, upgrade, init commands to project.Resolve
- [ ] #584 — Enforce boundary: fail CI on direct AGENTIC_* reads outside internal/project/
- [ ] #585 — Remove .ai-version file and local AGENTIC_TOPOLOGY gate
- [ ] #586 — Sweep skills, docs, and README to remove .ai-version and stopgap references
- [ ] #587 — Regression test: charging-domain scenario passes without AGENTIC_TOPOLOGY stopgap
