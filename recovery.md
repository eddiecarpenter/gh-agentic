# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | d1307b7                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T12:10:00Z                             |

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
- **Implemented:** checkProjectReachability runs on every topology. Pass surfaces the
  project title; Fail carries actionable remediation. FetchProjectTitle is injectable
  via CheckDeps for tests.
- **Files:** internal/doctorv2/checks.go + test; internal/cli/check.go wired to
  project.DefaultFetchProjectTitle.

## Remaining Tasks

- [ ] #503 — Add centralised error renderer for status commands ← current
- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests
