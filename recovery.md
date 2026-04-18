# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | a4c46db                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T10:55:00Z                             |

## Completed Tasks

### #494 — Scaffold gh agentic status command group and four sub-command stubs
- **Implemented:** Cobra command group with four leaf sub-commands as stubs.
  All downstream flags declared. Wired into root.
- **Files changed:** internal/cli/status.go, internal/cli/status_test.go, internal/cli/root.go
- **Decisions:** Shared `errStatusNotImplemented` sentinel; flag registrars split into list vs detail.

### #495 — Build internal/projectstatus package — types, GraphQL queries, typed errors
- **Implemented:** Complete data model (Requirement, Feature, summaries, TaskRef, BranchState,
  PRState, BlockedInfo) with canonical Stage enum. Injectable Deps with production GraphQL
  implementations. Typed errors (ErrIssueNotFound, ErrWrongType, ErrProjectNotConfigured,
  ErrProjectUnreachable) and ClassifyAPIError. Parent resolution prefers native trackedInIssues,
  falls back to `Closes #N` in the body. Extensive unit tests with fake deps.
- **Files changed:** internal/projectstatus/{types,deps,errors,queries,queries_default}.go + tests.
- **Decisions:** Blocked annotation deferred to task #501 (field on struct already). Stage parsing
  is the single chokepoint — ParseStage normalises spaces, hyphens, case. A bare `Closes #N` only
  matches within the same owning repo to prevent cross-repo number collisions.

## Remaining Tasks

- [ ] #496 — Implement 'gh agentic status requirements' list command ← current
- [ ] #497 — Implement 'gh agentic status requirement <N>' detail command
- [ ] #498 — Implement 'gh agentic status features' list command
- [ ] #499 — Implement 'gh agentic status feature <N>' detail command
- [ ] #500 — Implement --kanban renderer (vertical + --horizontal) with --json precedence
- [ ] #501 — Wire blocked-by dependency detection
- [ ] #502 — Extend 'gh agentic check' to verify AGENTIC_PROJECT_ID reachability
- [ ] #503 — Add centralised error renderer for status commands
- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests
