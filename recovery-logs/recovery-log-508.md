# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #508                               |
| Branch              | feature/508-horizontal-kanban-default-progress-icons |
| Last commit         | 5606ec2                            |
| Total tasks         | 5                                  |
| Last updated        | 2026-04-18T21:22:00Z               |

## Completed Tasks

### #510 — feat: flip kanban default to horizontal + --vertical flag + auto-fallback to vertical on narrow terminals
- **Implemented:** Horizontal kanban is now the default for both `status requirements --kanban` and `status features --kanban`. New `--vertical` flag forces vertical; `--horizontal` now honours the user's choice even on narrow terminals. Narrow terminals auto-fall-back to vertical with a one-line notice. Mutual-exclusion validation on `--horizontal` + `--vertical`.
- **Files changed:** internal/cli/status.go, internal/cli/kanban.go, internal/cli/kanban_test.go, internal/cli/status_features.go, internal/cli/status_features_test.go, internal/cli/status_requirements.go, internal/cli/status_requirements_test.go, internal/cli/status_integration_test.go
- **Decisions:** Named width thresholds: `requirementKanbanMinWidth = 100`, `featureKanbanMinWidth = 120`. Legacy const aliases retained. `writeHorizontalKanban` now clamps layout width to `minWidth` rather than erroring.

### #511 — feat: load feature task counts for list/kanban views without changing JSON schemas
- **Implemented:** Added `TasksTotal int` / `TasksDone int` fields on `Feature` with `json:"-"` tags. `FetchFeatures` populates both counts per feature via the existing `FetchSubIssues` dep (best-effort: fetch failures → zero counts, not a hard error). `FetchFeature` mirrors the counts from its already-fetched Tasks slice. Tests cover zero/partial/full completion, graceful degradation when dep not wired, JSON exclusion of the new fields, and cli-layer assertions that neither list nor detail `--json` payloads leak any new keys.
- **Files changed:** internal/projectstatus/types.go, internal/projectstatus/queries.go, internal/projectstatus/queries_test.go, internal/cli/status_json_schema_test.go
- **Decisions:** N+1 sub-issue fetch loop for V1; a future optimisation can batch via GraphQL aliases — called out in the code comment. Tasks 3–5 consume `f.TasksTotal`/`f.TasksDone` directly.

### #512 — feat: block-bar progress rendering utility with Unicode/ASCII fallback and 20-block cap
- **Implemented:** New `ui.RenderProgressBar(done, total, unicode)` returns a bracketed block-bar: `[■■■□□□]` (Unicode) or `[###   ]` (ASCII). Cap at 20 blocks with proportional-rounded fill for larger totals. Zero-total convention: `[]`. Out-of-range `done` values clamp safely. Pure and side-effect-free; terminal detection stays with callers.
- **Files changed:** internal/ui/progress.go, internal/ui/progress_test.go
- **Decisions:** ASCII empty glyph is a single space (not `.`) — fixed choice consistent with UX-4 mockup. Integer rounding used so math package is not needed: `(done*20 + total/2) / total`.

### #513 — feat: render block-bar progress + N/M numeric count on feature kanban cards (horizontal + vertical)
- **Implemented:** `featureCards` now takes a `unicode bool` flag and appends a `[blocks] N/M tasks` line to every feature card. Both `writeHorizontalKanban` and `writeVerticalKanban` paths show the line. Requirement cards are left strictly untouched (AC-9). New negative-assertion tests guard against bleed into the requirements kanban.
- **Files changed:** internal/cli/kanban.go, internal/cli/kanban_test.go, internal/cli/status_features.go
- **Decisions:** Progress line is the second line of each card — before the optional `[blocked by ...]` line — so card layout stays predictable.

## Remaining Tasks

- [ ] #514 — feat: show compact N/M task count on feature list (non-kanban) view ← current
