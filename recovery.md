# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | ed1bb4d                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T11:25:00Z                             |

## Completed Tasks

### #494 — Scaffold status command group and stubs
### #495 — Build internal/projectstatus package
### #496 — Implement 'gh agentic status requirements' list
### #497 — Implement 'gh agentic status requirement <N>' detail
### #498 — Implement 'gh agentic status features' list
- **Implemented:** runStatusFeatures + renderer. REPO column shown when cross-repo.
  --this-repo, --include-done, --json wired; --kanban guarded.
- **Files:** internal/cli/status_features.go + test; status.go wired.
- **Decisions:** Feature JSON items include all struct fields (parent_requirement, tasks,
  branch, pr) — list-view callers see those as empty or null but the schema is identical
  to detail items.

## Remaining Tasks

- [ ] #499 — Implement 'gh agentic status feature <N>' detail command ← current
- [ ] #500 — Implement --kanban renderer (vertical + --horizontal) with --json precedence
- [ ] #501 — Wire blocked-by dependency detection
- [ ] #502 — Extend 'gh agentic check' to verify AGENTIC_PROJECT_ID reachability
- [ ] #503 — Add centralised error renderer for status commands
- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests
