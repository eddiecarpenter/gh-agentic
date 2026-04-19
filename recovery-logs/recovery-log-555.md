# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #555                               |
| Branch              | feature/555-topology-resolver      |
| Last commit         | 92ff97e                            |
| Total tasks         | 4                                  |
| Last updated        | 2026-04-19T04:35:30Z               |

## Completed Tasks

### #556 — Add shared topology-resolver in internal/project with unit tests
- **Implemented:** Added `ResolveTopology` in `internal/project/topology.go` as the single source of truth for pipeline topology detection, plus full unit test coverage (all outcomes, error path, caching assertion).
- **Files changed:** internal/project/topology.go, internal/project/topology_test.go
- **Decisions:** Resolver takes `GetRepoVariableFunc` (canonical project-package abstraction) and `FetchLinkedReposFunc`, plus `Owner`, `Repo`, `ProjectID` — so callers can wire existing `project.Deps` values directly, and tests need no shell mocking.

### #557 — Wire check.go and repair.go to the shared topology-resolver
- **Implemented:** Replaced the duplicated topology switch in both `internal/cli/check.go` and `internal/cli/repair.go` with `project.ResolveTopology`. Deleted the `federated-domain → single` downgrade and removed the local `resolveTopologyMode` helper (it now lives inside the resolver). `checkDeps` gained a `fetchLinkedRepos` field so tests can inject a fake when needed.
- **Files changed:** internal/cli/check.go, internal/cli/repair.go
- **Decisions:** In `repair.go`, `pdeps.GetRepoVariable` / `pdeps.FetchLinkedRepos` are preferred when set (production and tests wire these on `project.Deps`); fall back to the run-func adapter + `project.DefaultFetchLinkedRepos` only if the caller has not. In `check.go`, a tiny adapter `checkGetRepoVariable(run)` converts the existing `auth.RunCommandFunc` into the project-package `GetRepoVariableFunc`.

### #558 — Add regression test reproducing charging-domain misdetection
- **Implemented:** Added `TestResolveTopology_ChargingDomainRegression` in `internal/project/topology_regression_test.go` that reconstructs the exact production scenario from NewOpenBSS/charging-domain (no local AGENTIC_TOPOLOGY, no local AGENTIC_FRAMEWORK_VERSION, project with >1 linked repos including a CP) and asserts `federated-domain`.
- **Files changed:** internal/project/topology_regression_test.go
- **Decisions:** Kept the regression in its own file so the docstring and history clearly reference Feature #555; the test also asserts FetchLinkedRepos is invoked exactly once to protect the caching contract.

### #559 — Final verification and PR manual sanity-check note
- **Implemented:** Re-ran `go build ./...` and `go test ./...` — both clean. Recorded the manual-verification note (shared org-level values no longer reported as missing on `NewOpenBSS/charging-domain`) in the final commit body so it surfaces in the PR's commit history.
- **Files changed:** (empty verification commit — no source files)
- **Decisions:** The Dev Session workflow hard-codes the PR body to `Closes #N`, so the verification note is attached to the last commit (the standard place reviewers look). A `PR_BODY.md` fragment was considered but not used — there is no wiring on this repo's workflow to consume it.

## Remaining Tasks

(none)
