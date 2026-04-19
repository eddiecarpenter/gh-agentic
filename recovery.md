# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #571                               |
| Branch              | feature/571-centralised-project-context |
| Last commit         | a50af64                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-19T11:14:00Z               |

## Completed Tasks

### #579 — Audit and publish call-site inventory
- **Implemented:** `docs/refactor-audit-571.md` — inventory of every AGENTIC_* read outside `internal/project/` and every `.ai-version` read/write.

### #580 — Introduce unified project.Context resolver
- **Implemented:** `project.Resolve(deps) (*project.Context, error)` in `internal/project/context.go`. `ResolveTopology` and `ResolveState` retained as thin wrappers.

### #581 — Migrate mount command to project.Resolve
- **Implemented:** `resolveMountVersion`, `detectFederatedCP`, `resolveFederatedCP` now go through `project.Resolve`. `.ai-version` fallback retained until #585.

### #582 — Migrate check and repair commands to project.Resolve
- **Implemented:** `check.go` RunE and `repair.go:buildPipelineCheckDeps` use a single `project.Resolve` call. Removed the `runGetVariable` / `checkGetRepoVariable` helpers.

### #583 — Migrate status, info, auth, upgrade, init commands to project.Resolve
- **Implemented:** Status/pipeline sub-commands, info.go, auth.go, init.go, project.go pickers, and doctor's `checkProjectAffiliation`/`checkProjectReachability` all route through the resolver. Doctor tests updated to populate `deps.ProjectID`.

### #584 — Enforce boundary: CI check for AGENTIC_* reads
- **Implemented:** `internal/project/boundary_test.go` — repo-walking scanner that fails `go test ./internal/project/...` if any file outside `internal/project/` references `project.ProjectVarName`, `project.TopologyVarName`, `project.FrameworkVersionVarName`, or `project.DefaultGetRepoVariable`. Positive case (clean tree) and negative case (table-driven synthetic violations, including allow-list and comment-only handling) both covered. Pre-gate cleanup: migrated `project.go` SwitchProject picker to `currentProjectID`, replaced `project.ProjectVarName` in error-format strings with bare `"AGENTIC_PROJECT_ID"` literals, and annotated the sanctioned topology-variable write in `cli/init.go` with `// boundary-allow: write path`.
- **Files changed:** internal/project/boundary_test.go, internal/cli/init.go, internal/cli/pipeline_cmd.go, internal/cli/project.go, internal/cli/status_features.go, internal/cli/status_requirements.go
- **Decisions:** Scanner targets identifier references only; string literals (used in user-facing messages) are NOT forbidden. Lines may opt out with `// boundary-allow: <rationale>`.

## Remaining Tasks

- [ ] #585 — Remove .ai-version file and local AGENTIC_TOPOLOGY gate ← current
- [ ] #586 — Sweep skills, docs, and README to remove .ai-version and stopgap references
- [ ] #587 — Regression test: charging-domain scenario passes without AGENTIC_TOPOLOGY stopgap
