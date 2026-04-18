# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | cea2560                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T11:15:00Z                             |

## Completed Tasks

### #494 — Scaffold gh agentic status command group and four sub-command stubs
- **Files:** internal/cli/status.go, status_test.go, root.go
- **Decisions:** Shared `errStatusNotImplemented` sentinel.

### #495 — Build internal/projectstatus package
- **Files:** internal/projectstatus/{types,deps,errors,queries,queries_default}.go + tests.

### #496 — Implement 'gh agentic status requirements' list
- **Files:** internal/cli/status_requirements.go + test; status.go updated.

### #497 — Implement 'gh agentic status requirement <N>' detail
- **Implemented:** Handler + renderer. UX-3 layout (title, stage/dates, optional Blocked line,
  body, `---` separator, linked features with branch/PR one-liners). JSON single-object.
  ErrIssueNotFound annotated with current repo; *ErrWrongType passes through.
- **Files:** internal/cli/status_requirement.go + test; status.go wired.
- **Decisions:** parseIssueNumberArg tolerates `#N` and plain `N`; rejects zero/negative.

## Remaining Tasks

- [ ] #498 — Implement 'gh agentic status features' list command ← current
- [ ] #499 — Implement 'gh agentic status feature <N>' detail command
- [ ] #500 — Implement --kanban renderer (vertical + --horizontal) with --json precedence
- [ ] #501 — Wire blocked-by dependency detection
- [ ] #502 — Extend 'gh agentic check' to verify AGENTIC_PROJECT_ID reachability
- [ ] #503 — Add centralised error renderer for status commands
- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests
