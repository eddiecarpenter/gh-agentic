# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | 8af0fcd                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T12:00:00Z                             |

## Completed Tasks

### #494 — Scaffold status command group and stubs
### #495 — Build internal/projectstatus package
### #496 — Implement 'gh agentic status requirements' list
### #497 — Implement 'gh agentic status requirement <N>' detail
### #498 — Implement 'gh agentic status features' list
### #499 — Implement 'gh agentic status feature <N>' detail
### #500 — Implement --kanban renderer (vertical + horizontal)
### #501 — Wire blocked-by dependency detection
- **Implemented:** FetchBlocker with native trackedIssues path + strict anchored
  `^Blocked-by:\s+...$` convention fallback. Populated on every list/detail query.
- **Files:** internal/projectstatus/blocked.go + test; deps.go / queries.go /
  queries_default.go updated.
- **Decisions:** Native source wins over convention. Native query returns the first
  *open* tracked issue as the blocker with its title as the reason. Failures in the
  native lookup are swallowed at the composer level so a transient error does not
  mask the whole list; the CLI layer already surfaces classification separately.

## Remaining Tasks

- [ ] #502 — Extend 'gh agentic check' to verify AGENTIC_PROJECT_ID reachability ← current
- [ ] #503 — Add centralised error renderer for status commands
- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests
