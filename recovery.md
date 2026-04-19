# Recovery State

| Field               | Value                                              |
|---------------------|----------------------------------------------------|
| Feature issue       | #518                                               |
| Branch              | feature/518-kanban-command-busy-spinner            |
| Last commit         | (pending task 2 commit)                            |
| Total tasks         | 6                                                  |
| Last updated        | 2026-04-19T02:03:00Z                               |

## Completed Tasks

### #519 — Add busy-spinner utility (delayed, non-TTY guarded) — internal/ui/busy.go
- **Implemented:** New `BusyRun(w io.Writer, label string, fn func() error) error` in `internal/ui/busy.go` with 500ms delay, non-TTY suppression (NO_COLOR / GH_NO_SPINNER / non-*os.File / isatty-false), auto-clear sequence, and shared-mutex safety for concurrent use. Added `NoopBusy` helper in `internal/testutil/noop_spinner.go` for later tasks.
- **Files changed:** internal/ui/busy.go, internal/ui/busy_test.go, internal/testutil/noop_spinner.go, internal/testutil/noop_spinner_test.go
- **Decisions:** Race-detector requested by the task but the runner has no gcc/cgo; verified logic via functional unit tests. `BusyFunc` type alias follows the existing `SpinnerFunc` pattern so dependents can inject a fake.

### #521 — Wire busy spinner into existing status fetch commands
- **Implemented:** Added a `busy ui.BusyFunc` field to `statusDeps` (production wires `ui.BusyRun`). Threaded a `stderr io.Writer` parameter through `runStatusRequirements`, `runStatusRequirement`, `runStatusFeatures`, `runStatusFeature`; each wraps its data-fetch in `deps.busy(stderr, label, fn)` with a command-appropriate label. Cobra wrappers call `cmd.ErrOrStderr()`. Added `TestRunStatus*_InvokesBusyWrapper` tests (recordingBusy pattern) plus `TestStatusCommands_NoSpinnerBytesOnNonTTYStderr` to lock AC-11 against a bytes.Buffer stderr. All 70+ existing test call sites bulk-migrated to pass `io.Discard` for stderr.
- **Files changed:** internal/cli/status.go, internal/cli/status_requirements.go, internal/cli/status_requirement.go, internal/cli/status_features.go, internal/cli/status_feature.go, internal/cli/status_busy_test.go (new), internal/cli/status_*_test.go (imports + stderr args), internal/cli/kanban_test.go, internal/cli/status_integration_test.go, internal/cli/status_json_schema_test.go, internal/cli/status_errors_test.go
- **Decisions:** Stderr is threaded as an explicit second parameter rather than buried in deps — keeps the contract obvious and makes tests simple (pass `io.Discard`). `deps.busy` signature matches `ui.BusyFunc` so swapping in NoopBusy is a one-line override. Downstream kanban task will reuse the same `busy` field.

## Remaining Tasks

- [ ] #522 — Scaffold gh agentic kanban Cobra command (flag wiring + help) ← current
- [ ] #523 — Implement gh agentic kanban behaviour: stacked default, selectors, JSON envelope
- [ ] #524 — Remove --kanban flag from status requirements / status features
- [ ] #525 — End-to-end verification + JSON schema fixture for combined kanban envelope
