# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | bd19f4d                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T11:05:00Z                             |

## Completed Tasks

### #494 — Scaffold gh agentic status command group and four sub-command stubs
- **Implemented:** Cobra command group with four leaf sub-commands as stubs.
- **Files:** internal/cli/status.go, status_test.go, root.go
- **Decisions:** Shared `errStatusNotImplemented` sentinel; flag registrars split list/detail.

### #495 — Build internal/projectstatus package — types, GraphQL queries, typed errors
- **Implemented:** Complete data model + injectable Deps + ClassifyAPIError.
- **Files:** internal/projectstatus/{types,deps,errors,queries,queries_default}.go + tests.
- **Decisions:** Blocked annotation deferred to #501. Stage parsing normalises spaces/hyphens/case.

### #496 — Implement 'gh agentic status requirements' list command
- **Implemented:** Handler + renderer (table + JSON envelope). --include-done / --this-repo /
  --json wired. --kanban and --horizontal return scoped not-yet-implemented until task #500.
- **Files:** internal/cli/status_requirements.go + test; internal/cli/status.go updated to
  wire newStatusRequirementsCmdWithDeps. types.go gained JSON tags for the stable schema.
- **Decisions:** statusDeps struct is the shared CLI-layer injectable. REPO column shows only
  when at least one row is cross-repo. Totals line uses singular/plural form and a `(N blocked)`
  suffix when applicable.

## Remaining Tasks

- [ ] #497 — Implement 'gh agentic status requirement <N>' detail command ← current
- [ ] #498 — Implement 'gh agentic status features' list command
- [ ] #499 — Implement 'gh agentic status feature <N>' detail command
- [ ] #500 — Implement --kanban renderer (vertical + --horizontal) with --json precedence
- [ ] #501 — Wire blocked-by dependency detection
- [ ] #502 — Extend 'gh agentic check' to verify AGENTIC_PROJECT_ID reachability
- [ ] #503 — Add centralised error renderer for status commands
- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests
