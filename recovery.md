# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | 26c9c73                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T11:35:00Z                             |

## Completed Tasks

### #494 — Scaffold status command group and stubs
### #495 — Build internal/projectstatus package
### #496 — Implement 'gh agentic status requirements' list
### #497 — Implement 'gh agentic status requirement <N>' detail
### #498 — Implement 'gh agentic status features' list
### #499 — Implement 'gh agentic status feature <N>' detail
- **Implemented:** runStatusFeature + renderer. UX-3 layout with parent/branch/PR lines and
  task checklist. ✓/☐ glyphs with [x]/[ ] ASCII fallback. --json single-object.
- **Files:** internal/cli/status_feature.go + test; status.go wired;
  internal/ui/styles.go gained TerminalSupportsUTF8().
- **Decisions:** Empty resources render as explicit "(none)" / "(no branch yet)" /
  "(no PR opened)" rather than blank lines — avoids any "nil" leakage.

## Remaining Tasks

- [ ] #500 — Implement --kanban renderer (vertical + --horizontal) with --json precedence ← current
- [ ] #501 — Wire blocked-by dependency detection
- [ ] #502 — Extend 'gh agentic check' to verify AGENTIC_PROJECT_ID reachability
- [ ] #503 — Add centralised error renderer for status commands
- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests
