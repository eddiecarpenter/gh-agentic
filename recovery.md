# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | 4770ae1                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T11:50:00Z                             |

## Completed Tasks

### #494 — Scaffold status command group and stubs
### #495 — Build internal/projectstatus package
### #496 — Implement 'gh agentic status requirements' list
### #497 — Implement 'gh agentic status requirement <N>' detail
### #498 — Implement 'gh agentic status features' list
### #499 — Implement 'gh agentic status feature <N>' detail
### #500 — Implement --kanban renderer (vertical + horizontal)
- **Implemented:** Shared kanban renderer. Vertical view any width; horizontal view uses
  Unicode box-drawing (ASCII fallback), min 90/120 cols for requirements/features.
  --include-done appends "done" as rightmost column. --json silently beats --kanban with
  identical JSON schema.
- **Files:** internal/cli/kanban.go + test; status_requirements.go / status_features.go wired.
- **Decisions:** terminalWidth is a package-level var so tests can inject a deterministic
  width. Minimum widths are constants (kanbanMinHorizontalWidth{Requirements,Features}).

## Remaining Tasks

- [ ] #501 — Wire blocked-by dependency detection ← current
- [ ] #502 — Extend 'gh agentic check' to verify AGENTIC_PROJECT_ID reachability
- [ ] #503 — Add centralised error renderer for status commands
- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests
