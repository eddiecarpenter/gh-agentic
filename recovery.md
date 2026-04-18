# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #508                               |
| Branch              | feature/508-horizontal-kanban-default-progress-icons |
| Last commit         | 4dd53fb                            |
| Total tasks         | 5                                  |
| Last updated        | 2026-04-18T21:12:00Z               |

## Completed Tasks

### #510 — feat: flip kanban default to horizontal + --vertical flag + auto-fallback to vertical on narrow terminals
- **Implemented:** Horizontal kanban is now the default for both `status requirements --kanban` and `status features --kanban`. New `--vertical` flag forces vertical; `--horizontal` now honours the user's choice even on narrow terminals. Narrow terminals auto-fall-back to vertical with a one-line notice. Mutual-exclusion validation on `--horizontal` + `--vertical`.
- **Files changed:** internal/cli/status.go, internal/cli/kanban.go, internal/cli/kanban_test.go, internal/cli/status_features.go, internal/cli/status_features_test.go, internal/cli/status_requirements.go, internal/cli/status_requirements_test.go, internal/cli/status_integration_test.go
- **Decisions:** Named width thresholds: `requirementKanbanMinWidth = 100`, `featureKanbanMinWidth = 120`. Legacy const aliases retained. `writeHorizontalKanban` now clamps layout width to `minWidth` rather than erroring.

### #511 — feat: load feature task counts for list/kanban views without changing JSON schemas
- **Implemented:** Added `TasksTotal int` / `TasksDone int` fields on `Feature` with `json:"-"` tags. `FetchFeatures` populates both counts per feature via the existing `FetchSubIssues` dep (best-effort: fetch failures → zero counts, not a hard error). `FetchFeature` mirrors the counts from its already-fetched Tasks slice. Tests cover zero/partial/full completion, ungraceful-degradation when dep not wired, JSON exclusion of the new fields, and cli-layer assertions that neither list nor detail `--json` payloads leak any new keys.
- **Files changed:** internal/projectstatus/types.go, internal/projectstatus/queries.go, internal/projectstatus/queries_test.go, internal/cli/status_json_schema_test.go
- **Decisions:** N+1 sub-issue fetch loop for V1; a future optimisation can batch via GraphQL aliases — called out in the code comment. Tasks 3–5 consume `f.TasksTotal`/`f.TasksDone` directly.

## Remaining Tasks

- [ ] #512 — feat: block-bar progress rendering utility with Unicode/ASCII fallback and 20-block cap ← current
- [ ] #513 — feat: render block-bar progress + N/M numeric count on feature kanban cards (horizontal + vertical)
- [ ] #514 — feat: show compact N/M task count on feature list (non-kanban) view
