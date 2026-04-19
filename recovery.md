# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #571                               |
| Branch              | feature/571-centralised-project-context |
| Last commit         | 8f205ee                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-19T11:04:00Z               |

## Completed Tasks

### #579 — Audit and publish call-site inventory
- **Implemented:** Published `docs/refactor-audit-571.md` — inventory of every AGENTIC_* read outside `internal/project/` and every `.ai-version` read/write.
- **Files changed:** docs/refactor-audit-571.md

### #580 — Introduce unified project.Context resolver
- **Implemented:** Added `project.Resolve(deps) (*project.Context, error)`. Context exposes Topology, Role, LinkedRepos, ControlPlane, FrameworkVersion, LocalAIVersion, VersionInSync, plus role helpers. `ResolveTopology` and `ResolveState` retained as thin wrappers.
- **Files changed:** internal/project/context.go, context_test.go, info.go, topology.go

### #581 — Migrate mount command to project.Resolve
- **Implemented:** Replaced `resolveMountVersion`, `detectFederatedCP`, `resolveFederatedCP` in `internal/cli/mount.go` with calls to `project.Resolve`. `.ai-version` fallback retained until #585.
- **Files changed:** internal/cli/mount.go

### #582 — Migrate check and repair commands to project.Resolve
- **Implemented:** Replaced the `runGetVariable + checkGetRepoVariable + ResolveTopology` dance with a single `project.Resolve` call in check.go RunE and repair.go:buildPipelineCheckDeps. Removed the now-dead helpers.
- **Files changed:** internal/cli/check.go, internal/cli/repair.go

### #583 — Migrate status, info, auth, upgrade, init commands to project.Resolve
- **Implemented:** `defaultResolveProjectID` now uses `project.Resolve` → `ctx.ProjectID`. `info.go:collectInfo` uses `project.Resolve` directly (replaces `ResolveState`). `auth.go:isFederatedControlPlane` delegates to `ctx.IsFederatedControlPlane()`. `init.go` "already-initialised?" guard uses `ctx.ProjectID`. `project.go` picker commands (join / list / switch) use shared `currentProjectID(deps)` helper backed by the resolver. `doctor/checks.go:checkProjectAffiliation` trusts `deps.ProjectID` / `deps.Topology` (no more `checkVariable` calls for identity vars). `checkProjectReachability` dropped its stale fallback gh-CLI read. Doctor tests updated to populate `deps.ProjectID` explicitly. `go build ./...` and `go test ./...` pass.
- **Files changed:** internal/cli/auth.go, info.go, init.go, project.go, status_requirements.go, internal/doctor/checks.go, checks_test.go, repair_test.go
- **Decisions:** The remaining AGENTIC_* mentions outside `internal/project/` are sanctioned per the audit: scope.ScopeFor routing map, doctor diagnostic messages, docstrings, user-facing error strings, init wizard write path, and one federated-cp variable-existence diagnostic in checkVariablesAndSecrets.

## Remaining Tasks

- [ ] #584 — Enforce boundary: fail CI on direct AGENTIC_* reads outside internal/project/ ← current
- [ ] #585 — Remove .ai-version file and local AGENTIC_TOPOLOGY gate
- [ ] #586 — Sweep skills, docs, and README to remove .ai-version and stopgap references
- [ ] #587 — Regression test: charging-domain scenario passes without AGENTIC_TOPOLOGY stopgap
