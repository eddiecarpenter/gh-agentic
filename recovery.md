# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #555                               |
| Branch              | feature/555-topology-resolver      |
| Last commit         | b24ba86                            |
| Total tasks         | 4                                  |
| Last updated        | 2026-04-19T04:31:13Z               |

## Completed Tasks

### #556 — Add shared topology-resolver in internal/project with unit tests
- **Implemented:** Added `ResolveTopology` in `internal/project/topology.go` as the single source of truth for pipeline topology detection, plus full unit test coverage (all outcomes, error path, caching assertion).
- **Files changed:** internal/project/topology.go, internal/project/topology_test.go
- **Decisions:** Resolver takes `GetRepoVariableFunc` (canonical project-package abstraction) and `FetchLinkedReposFunc`, plus `Owner`, `Repo`, `ProjectID` — so callers can wire existing `project.Deps` values directly, and tests need no shell mocking.

## Remaining Tasks

- [ ] #557 — Wire check.go and repair.go to the shared topology-resolver ← current
- [ ] #558 — Add regression test reproducing charging-domain misdetection
- [ ] #559 — Final verification and PR manual sanity-check note
