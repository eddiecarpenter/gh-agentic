# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #549                               |
| Branch              | feature/549-kanban-under-status    |
| Last commit         | bb8088c                            |
| Total tasks         | 2                                  |
| Last updated        | 2026-04-19T04:10:00Z               |

## Completed Tasks

### #551 — Move kanban command from root to status in Cobra tree
- **Implemented:** Removed top-level kanban registration from `root.go`, registered `newKanbanCmd()` as a child of `status` in `status.go`, updated long description and Example blocks, and rewrote the kanban registration tests (renamed `TestKanbanCmd_RegisteredOnRoot` → `TestKanbanCmd_RegisteredUnderStatus`, added `TestKanbanCmd_NotOnRoot`, `TestKanbanCmd_OldPathUnknownCommand`, `TestStatusCmd_HelpListsKanban`, replaced `TestKanbanCmd_RootLongDescriptionMentionsKanban` with `TestKanbanCmd_RootLongDescriptionOmitsKanban`). Also updated `TestStatusCmd_RegistersFourSubCommands` → `TestStatusCmd_RegistersExpectedSubCommands` and the bare-help token list to include `kanban`.
- **Files changed:** `internal/cli/root.go`, `internal/cli/status.go`, `internal/cli/kanban_cmd_test.go`, `internal/cli/status_test.go`
- **Decisions:** Updating `TestStatusCmd_RegistersFourSubCommands` (and the bare-help token list) was collateral to the status-child registration and not spelled out in #551's Files list, but is a direct consequence — without the update, the test fails.

## Remaining Tasks

- [ ] #552 — Update textual references from 'gh agentic kanban' to 'gh agentic status kanban' ← current
