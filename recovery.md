# Recovery State

| Field               | Value                                              |
|---------------------|----------------------------------------------------|
| Feature issue       | #518                                               |
| Branch              | feature/518-kanban-command-busy-spinner            |
| Last commit         | (pending task 3 commit)                            |
| Total tasks         | 6                                                  |
| Last updated        | 2026-04-19T02:10:00Z                               |

## Completed Tasks

### #519 — Add busy-spinner utility (delayed, non-TTY guarded) — internal/ui/busy.go
- **Implemented:** `BusyRun(w io.Writer, label string, fn func() error) error` with 500ms delay, NO_COLOR / GH_NO_SPINNER / non-TTY suppression, auto-clear sequence, mutex-safe. `testutil.NoopBusy` for downstream tests.
- **Files changed:** internal/ui/busy.go, internal/ui/busy_test.go, internal/testutil/noop_spinner.go, internal/testutil/noop_spinner_test.go
- **Decisions:** Race-detector requested but runner has no gcc; logic validated by functional tests. `BusyFunc` mirrors existing `SpinnerFunc` pattern.

### #521 — Wire busy spinner into existing status fetch commands
- **Implemented:** Added `busy ui.BusyFunc` to `statusDeps`, threaded `stderr io.Writer` through all four status handlers, wrapped each fetch with command-appropriate label. Added recordingBusy tests + AC-11 non-TTY-stderr regression. Bulk-migrated all existing test call sites.
- **Files changed:** internal/cli/status.go, status_requirements.go, status_requirement.go, status_features.go, status_feature.go, status_busy_test.go (new), all *_test.go call sites.
- **Decisions:** Explicit stderr parameter (rather than deps-only) keeps contract clear. Downstream kanban will reuse the same `deps.busy`.

### #522 — Scaffold gh agentic kanban Cobra command (flag wiring + help)
- **Implemented:** `internal/cli/kanban_cmd.go` with `newKanbanCmd`, `newKanbanCmdWithDeps`, `kanbanFlags`, `registerKanbanFlags`, stub handler `runKanban` returning `errKanbanNotImplemented`. Registered on root with long-description update. Tests verify registration, help, every flag declared, mutual-exclusion error, NoArgs rejection, bare-invocation returns stub sentinel, root help mentions kanban.
- **Files changed:** internal/cli/kanban_cmd.go, internal/cli/kanban_cmd_test.go, internal/cli/root.go
- **Decisions:** Kept the stub handler parameter signature matching the future real handler (stdout, stderr, flags, deps) so task #523 needs only to replace the body. `errKanbanNotImplemented` is a package-private sentinel for test assertion via `errors.Is`.

## Remaining Tasks

- [ ] #523 — Implement gh agentic kanban behaviour: stacked default, selectors, JSON envelope ← current
- [ ] #524 — Remove --kanban flag from status requirements / status features
- [ ] #525 — End-to-end verification + JSON schema fixture for combined kanban envelope
