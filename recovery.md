# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #508                               |
| Branch              | feature/508-horizontal-kanban-default-progress-icons |
| Last commit         | 0816657                            |
| Total tasks         | 5                                  |
| Last updated        | 2026-04-18T21:03:34Z               |

## Completed Tasks

### #510 — feat: flip kanban default to horizontal + --vertical flag + auto-fallback to vertical on narrow terminals
- **Implemented:** Horizontal kanban is now the default for both `status requirements --kanban` and `status features --kanban`. New `--vertical` flag forces vertical; `--horizontal` now honours the user's choice even on narrow terminals. Narrow terminals auto-fall-back to vertical with a one-line notice. Mutual-exclusion validation on `--horizontal` + `--vertical`.
- **Files changed:** internal/cli/status.go, internal/cli/kanban.go, internal/cli/kanban_test.go, internal/cli/status_features.go, internal/cli/status_features_test.go, internal/cli/status_requirements.go, internal/cli/status_requirements_test.go, internal/cli/status_integration_test.go
- **Decisions:** Named width thresholds: `requirementKanbanMinWidth = 100`, `featureKanbanMinWidth = 120`. Legacy const aliases retained. `writeHorizontalKanban` now clamps layout width to `minWidth` rather than erroring.

## Remaining Tasks

- [ ] #511 — feat: load feature task counts for list/kanban views without changing JSON schemas ← current
- [ ] #512 — feat: block-bar progress rendering utility with Unicode/ASCII fallback and 20-block cap
- [ ] #513 — feat: render block-bar progress + N/M numeric count on feature kanban cards (horizontal + vertical)
- [ ] #514 — feat: show compact N/M task count on feature list (non-kanban) view
