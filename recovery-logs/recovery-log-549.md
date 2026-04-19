# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #549                               |
| Branch              | feature/549-kanban-under-status    |
| Last commit         | a10b609                            |
| Total tasks         | 2                                  |
| Last updated        | 2026-04-19T04:20:00Z               |

## Completed Tasks

### #551 — Move kanban command from root to status in Cobra tree
- **Implemented:** Removed top-level kanban registration from `root.go`, registered `newKanbanCmd()` as a child of `status` in `status.go`, updated long description and Example blocks, and rewrote the kanban registration tests (renamed `TestKanbanCmd_RegisteredOnRoot` → `TestKanbanCmd_RegisteredUnderStatus`, added `TestKanbanCmd_NotOnRoot`, `TestKanbanCmd_OldPathUnknownCommand`, `TestStatusCmd_HelpListsKanban`, replaced `TestKanbanCmd_RootLongDescriptionMentionsKanban` with `TestKanbanCmd_RootLongDescriptionOmitsKanban`). Also updated `TestStatusCmd_RegistersFourSubCommands` → `TestStatusCmd_RegistersExpectedSubCommands` and the bare-help token list to include `kanban`.
- **Files changed:** `internal/cli/root.go`, `internal/cli/status.go`, `internal/cli/kanban_cmd_test.go`, `internal/cli/status_test.go`
- **Decisions:** Updating `TestStatusCmd_RegistersFourSubCommands` (and the bare-help token list) was collateral to the status-child registration and not spelled out in #551's Files list, but is a direct consequence — without the update, the test fails.

### #552 — Update textual references from 'gh agentic kanban' to 'gh agentic status kanban'
- **Implemented:** Migration-error `suggestedCommand` pointers in `status_requirements.go` and `status_features.go` updated to the new path. `status.go` hidden `--kanban` flag description, Long + Example blocks on `requirements` and `features` list commands, and the registerStatusListFlags doc comment updated. `kanban_cmd.go` Example block, kanbanFlags / newKanbanCmd / runKanban doc comments updated. `status_errors.go` errKanbanFlagRemoved doc comment updated. `status_test.go` migration-error assertions updated; `TestStatusCmd_ListFlagsRegistered` comment refreshed. `kanban_combined_envelope.schema.json` `description` field refreshed — schema shape (envelope keys, totals fields, inner field lists) unchanged.
- **Files changed:** `internal/cli/status_requirements.go`, `internal/cli/status_features.go`, `internal/cli/status.go`, `internal/cli/kanban_cmd.go`, `internal/cli/status_errors.go`, `internal/cli/status_test.go`, `internal/cli/testdata/status_schemas/kanban_combined_envelope.schema.json`
- **Decisions:** Two test-file doc comments in `kanban_cmd_test.go` (on `TestKanbanCmd_NotOnRoot` and `TestKanbanCmd_OldPathUnknownCommand`) retain the bare phrase `gh agentic kanban` as historical reference — these comments describe the legacy path that the tests assert against; they are developer-facing context, not user-facing text, and removing them would harm readability.

## Remaining Tasks

(none — all tasks closed)

## Acceptance Criteria Verification

All 5 acceptance criteria from feature #549 are covered by passing tests:
- AC-1 (status kanban renders identically): covered by the behavioural kanban suite in `kanban_cmd_test.go` (handler untouched by #549).
- AC-2 (flags identical under new path): `TestKanbanCmd_AllFlagsRegistered`, `TestKanbanCmd_HelpListsFlags`, `TestKanbanCmd_RegisteredUnderStatus`.
- AC-3 (old path returns unknown command): `TestKanbanCmd_OldPathUnknownCommand`.
- AC-4 (root help omits kanban): `TestKanbanCmd_RootLongDescriptionOmitsKanban`.
- AC-5 (status help lists kanban): `TestStatusCmd_HelpListsKanban`.
