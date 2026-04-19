# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #555                               |
| Branch              | feature/555-topology-resolver      |
| Last commit         | cb592a3                            |
| Total tasks         | 4                                  |
| Last updated        | 2026-04-19T04:33:18Z               |

## Completed Tasks

### #556 — Add shared topology-resolver in internal/project with unit tests
- **Implemented:** Added `ResolveTopology` in `internal/project/topology.go` as the single source of truth for pipeline topology detection, plus full unit test coverage (all outcomes, error path, caching assertion).
- **Files changed:** internal/project/topology.go, internal/project/topology_test.go
- **Decisions:** Resolver takes `GetRepoVariableFunc` (canonical project-package abstraction) and `FetchLinkedReposFunc`, plus `Owner`, `Repo`, `ProjectID` — so callers can wire existing `project.Deps` values directly, and tests need no shell mocking.

### #557 — Wire check.go and repair.go to the shared topology-resolver
- **Implemented:** Replaced the duplicated topology switch in both `internal/cli/check.go` and `internal/cli/repair.go` with `project.ResolveTopology`. Deleted the `federated-domain → single` downgrade and removed the local `resolveTopologyMode` helper (it now lives inside the resolver). `checkDeps` gained a `fetchLinkedRepos` field so tests can inject a fake when needed.
- **Files changed:** internal/cli/check.go, internal/cli/repair.go
- **Decisions:** In `repair.go`, `pdeps.GetRepoVariable` / `pdeps.FetchLinkedRepos` are preferred when set (production and tests wire these on `project.Deps`); fall back to the run-func adapter + `project.DefaultFetchLinkedRepos` only if the caller has not. In `check.go`, a tiny adapter `checkGetRepoVariable(run)` converts the existing `auth.RunCommandFunc` into the project-package `GetRepoVariableFunc`.

## Remaining Tasks

- [ ] #558 — Add regression test reproducing charging-domain misdetection ← current
- [ ] #559 — Final verification and PR manual sanity-check note
