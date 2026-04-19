# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #571                               |
| Branch              | feature/571-centralised-project-context |
| Last commit         | 86d2028                            |
| Total tasks         | 9                                  |
| Last updated        | 2026-04-19T11:38:00Z               |

## Completed Tasks

### #579 — Audit and publish call-site inventory
- **Implemented:** `docs/refactor-audit-571.md` — authoritative inventory of AGENTIC_* reads and `.ai-version` sites.

### #580 — Introduce unified project.Context resolver
- **Implemented:** `project.Resolve(deps) (*project.Context, error)` in `internal/project/context.go`. Thin wrappers `ResolveTopology` / `ResolveState` preserved.

### #581 — Migrate mount command to project.Resolve
- **Implemented:** `mount.go` resolves version + CP through `project.Resolve`.

### #582 — Migrate check and repair commands to project.Resolve
- **Implemented:** `check.go` RunE and `repair.go:buildPipelineCheckDeps` consume `project.Resolve`. Removed `runGetVariable` / `checkGetRepoVariable` helpers.

### #583 — Migrate status, info, auth, upgrade, init commands to project.Resolve
- **Implemented:** Status sub-commands, info.go, auth.go, init.go, project.go pickers, and doctor's `checkProjectAffiliation` / `checkProjectReachability` all route through the resolver.

### #584 — Enforce boundary: CI check for AGENTIC_* reads
- **Implemented:** `internal/project/boundary_test.go` — scanner test that fails CI if any file outside `internal/project/` references `project.ProjectVarName`, `TopologyVarName`, `FrameworkVersionVarName`, or `DefaultGetRepoVariable`. Allow-list marker available for sanctioned writes. Positive + negative coverage.

### #585 — Remove .ai-version file and AGENTIC_TOPOLOGY gate
- **Implemented:**
  - **.ai-version**: deleted `ReadAIVersion` / `WriteAIVersion` from `internal/mount/mount.go`. Removed every write site (firsttime.go, switch.go, project/init.go). Every local-version read now goes through `ReadAIVersionFromGit` (the clone's .git metadata). Updated templates (`templates.go` + `AGENTS.md.tmpl`) to use argument-less `gh agentic mount`. Updated `.github/workflows/agentic-pipeline-reusable.yml` to read `${{ vars.AGENTIC_FRAMEWORK_VERSION }}` instead of `cat .ai-version`. Matching test fixtures updated.
  - **AGENTIC_TOPOLOGY auto-write**: removed `cli/init.go` single-topology auto-write. `project/repair.go:repairTopologyVars` only writes the variable when `--topology` is explicitly supplied; the auto-detect path keeps setting `AGENTIC_FRAMEWORK_VERSION` on federated CPs but leaves `AGENTIC_TOPOLOGY` alone. Added `doctor/checks.go:checkTopologyStopgap` — warns when the variable is set but redundant with the resolver's inference (with a concrete `gh variable delete` remediation).
  - **Tests**: 4 new doctor unit tests for the stopgap helper (not-set, redundant federated, redundant single, explicit override). All existing tests pass.
- **Files changed:** 20 files. Key: `internal/mount/mount.go`, `firsttime.go`, `switch.go`, `templates.go`, `templates/AGENTS.md.tmpl`, `internal/cli/mount.go`, `info.go`, `init.go`, `mount_test.go`, `.github/workflows/agentic-pipeline-reusable.yml`, `internal/project/init.go`, `repair.go`, `internal/doctor/checks.go`, `checks_test.go`, `internal/init/wizard.go`, `wizard_test.go`.
- **Decisions:** Kept `ReadAIVersionFromGit` (reads from `.ai/.git`, not the removed flat file) — it is the only remaining local version source. Kept the `AGENTIC_TOPOLOGY` precedence rule in `topology.go` so an explicit override still works (required for backward compat with older domain repos that have the stopgap set).

## Remaining Tasks

- [ ] #586 — Sweep skills, docs, and README to remove .ai-version and stopgap references ← current
- [ ] #587 — Regression test: charging-domain scenario passes without AGENTIC_TOPOLOGY stopgap
