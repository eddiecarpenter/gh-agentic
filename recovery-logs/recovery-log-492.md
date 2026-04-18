# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | ce463e5                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T12:20:00Z                             |

## Completed Tasks

### #494 — Scaffold status command group and stubs
### #495 — Build internal/projectstatus package
### #496 — Implement 'gh agentic status requirements' list
### #497 — Implement 'gh agentic status requirement <N>' detail
### #498 — Implement 'gh agentic status features' list
### #499 — Implement 'gh agentic status feature <N>' detail
### #500 — Implement --kanban renderer (vertical + horizontal)
### #501 — Wire blocked-by dependency detection
### #502 — Extend 'gh agentic check' for AGENTIC_PROJECT_ID reachability
### #503 — Centralised error renderer for status commands
- **Implemented:** renderStatusError maps every error class (ErrProjectNotConfigured,
  ErrProjectUnreachable, ErrIssueNotFound, *ErrWrongType, network/auth/rate-limit/5xx,
  narrow-terminal horizontal kanban) to a concrete stderr message and returns
  cli.ErrSilent. All four status sub-commands rewired to route failures through it.
- **Files:** internal/cli/status_errors.go + test; status.go RunE bodies updated.

## Remaining Tasks

- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests ← current
