# Recovery State

| Field               | Value                                              |
|---------------------|----------------------------------------------------|
| Feature issue       | #518                                               |
| Branch              | feature/518-kanban-command-busy-spinner            |
| Last commit         | (pending task 4 commit)                            |
| Total tasks         | 6                                                  |
| Last updated        | 2026-04-19T02:25:00Z                               |

## Completed Tasks

### #519 — Add busy-spinner utility (delayed, non-TTY guarded) — internal/ui/busy.go
- **Implemented:** `BusyRun` with 500ms delay, NO_COLOR / GH_NO_SPINNER / non-TTY suppression, auto-clear sequence, mutex-safe. `testutil.NoopBusy` for downstream tests.
- **Files changed:** internal/ui/busy.go, internal/ui/busy_test.go, internal/testutil/noop_spinner.go, internal/testutil/noop_spinner_test.go

### #521 — Wire busy spinner into existing status fetch commands
- **Implemented:** Added `busy ui.BusyFunc` to `statusDeps`, threaded stderr through all four handlers. recordingBusy tests + AC-11 non-TTY regression. Bulk-migrated all test call sites.
- **Files changed:** internal/cli/status.go, status_requirements.go, status_requirement.go, status_features.go, status_feature.go, status_busy_test.go (new), all *_test.go call sites.

### #522 — Scaffold gh agentic kanban Cobra command (flag wiring + help)
- **Implemented:** `internal/cli/kanban_cmd.go` with `newKanbanCmd`, `newKanbanCmdWithDeps`, `kanbanFlags`, `registerKanbanFlags`, stub handler. Registered on root.
- **Files changed:** internal/cli/kanban_cmd.go, internal/cli/kanban_cmd_test.go, internal/cli/root.go

### #523 — Implement gh agentic kanban behaviour: stacked default, selectors, JSON envelope
- **Implemented:** Real handler in `internal/cli/kanban_cmd.go` with combined `kanbanJSONEnvelope` / `kanbanJSONTotals` (omitempty-tagged so selector paths drop unused keys per AC-7), stacked / selector / selector-flag routing, layout resolution per kanban, busy-indicator integration with context-aware labels ("Fetching pipeline state…", "Fetching requirements…", "Fetching features…"), `--this-repo` filter applied to both lists, `--include-done` extending both column sets, `combinedTotalsLine` summing blocked across both lists. Helper `writeHorizontalKanbanWithHeading` adds the `=== Kanban ===` banner to match the vertical path's contract. `kanbanToStatusListFlags` bridges the selector-flag struct to the existing `resolveKanbanLayout`. Added 15 behavioural tests covering all AC-1–AC-7, AC-9, AC-11 acceptance criteria plus busy-wrapper integration.
- **Files changed:** internal/cli/kanban_cmd.go, internal/cli/kanban_cmd_test.go
- **Decisions:** Reused existing renderer helpers verbatim (`columnsForRequirements`, `featureCards`, `writeHorizontalKanban`, `writeVerticalKanban`, `resolveKanbanLayout`) — no rendering code duplication. JSON envelope uses existing `projectstatus.Requirement` / `projectstatus.Feature` tags verbatim — no new per-item fields (AC-14). Selector-flag behaviour is `omitempty` on the top-level slice fields; this produces key-absence semantics rather than `null`, which AC-7 tests assert via `_, present := parsed[key]`. Task #524 will complete the breaking-change migration by removing `--kanban` from the status sub-commands; task #525 will lock the JSON envelope with a snapshot fixture.

## Remaining Tasks

- [ ] #524 — Remove --kanban flag from status requirements / status features ← current
- [ ] #525 — End-to-end verification + JSON schema fixture for combined kanban envelope
