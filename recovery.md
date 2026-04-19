# Recovery State

| Field               | Value                                              |
|---------------------|----------------------------------------------------|
| Feature issue       | #518                                               |
| Branch              | feature/518-kanban-command-busy-spinner            |
| Last commit         | (pending task 5 commit)                            |
| Total tasks         | 6                                                  |
| Last updated        | 2026-04-19T02:45:00Z                               |

## Completed Tasks

### #519 — Add busy-spinner utility (delayed, non-TTY guarded) — internal/ui/busy.go
- **Implemented:** `BusyRun`, `NoopBusy`, all tests. 500ms delay / suppression precedence / auto-clear / mutex.

### #521 — Wire busy spinner into existing status fetch commands
- **Implemented:** `statusDeps.busy` field, threaded stderr, recordingBusy tests, non-TTY regression.

### #522 — Scaffold gh agentic kanban Cobra command
- **Implemented:** `newKanbanCmd`, flag surface, help, stub handler.

### #523 — Implement gh agentic kanban behaviour
- **Implemented:** Full runtime, JSON envelope with omitempty key-absence semantics, 15 behavioural tests.

### #524 — Remove --kanban flag from status requirements / status features
- **Implemented:** Split `registerStatusListFlags` (now only `--json`, `--this-repo`, `--include-done`). Added `registerRemovedKanbanFlag` that declares `--kanban` as hidden on status list commands. Added `errKanbanFlagRemoved` typed error with `suggestedCommand` field; `renderStatusError` renders it as the documented two-line message ("Error: --kanban has been removed from this command." / "Use: gh agentic kanban --<selector>"). `runStatusRequirements` / `runStatusFeatures` short-circuit to this error when `flags.kanban` is true. Stripped the dead `--kanban` / `--horizontal` / `--vertical` code paths from both handlers. Updated help Long/Example text to direct users to the new command. Deleted 15 legacy kanban-on-status tests across kanban_test.go, status_requirements_test.go, status_features_test.go, status_integration_test.go (equivalent coverage lives in kanban_cmd_test.go). Updated `TestStatusCmd_ListFlagsRegistered` to expect the reduced flag set; added `TestStatusCmd_KanbanFlagHiddenOnList` and `TestStatusCmd_KanbanFlagProducesMigrationError` asserting the breaking-change contract.
- **Files changed:** internal/cli/status.go, status_requirements.go, status_features.go, status_errors.go, status_test.go, kanban_test.go, status_requirements_test.go, status_features_test.go, status_integration_test.go
- **Decisions:** Kept `--kanban` declared-but-hidden (rather than fully removed) so we can intercept with a guided message rather than falling back to Cobra's "unknown flag" default. `--horizontal` and `--vertical` are fully removed from the status surface; unrecognised-flag errors are acceptable for those (they never made sense without --kanban). `statusListFlags` retains its kanban/horizontal/vertical fields because the kanban command still constructs a `statusListFlags` via `kanbanToStatusListFlags` to feed `resolveKanbanLayout` — the field is dead on status but live on kanban.

## Remaining Tasks

- [ ] #525 — End-to-end verification + JSON schema fixture for combined kanban envelope ← current
