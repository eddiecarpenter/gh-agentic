# Recovery State

| Field               | Value                                              |
|---------------------|----------------------------------------------------|
| Feature issue       | #518                                               |
| Branch              | feature/518-kanban-command-busy-spinner            |
| Last commit         | (pending)                                          |
| Total tasks         | 6                                                  |
| Last updated        | 2026-04-19T01:52:00Z                               |

## Completed Tasks

### #519 — Add busy-spinner utility (delayed, non-TTY guarded) — internal/ui/busy.go
- **Implemented:** New `BusyRun(w io.Writer, label string, fn func() error) error` in `internal/ui/busy.go` with 500ms delay, non-TTY suppression (NO_COLOR / GH_NO_SPINNER / non-*os.File / isatty-false), auto-clear sequence, and shared-mutex safety for concurrent use. Added `NoopBusy` helper in `internal/testutil/noop_spinner.go` for later tasks.
- **Files changed:** internal/ui/busy.go, internal/ui/busy_test.go, internal/testutil/noop_spinner.go, internal/testutil/noop_spinner_test.go
- **Decisions:** Race-detector requested by the task but the runner has no gcc/cgo; verified logic via functional unit tests (suppression precedence, fast/slow paths, concurrent goroutines against counting writer). `BusyFunc` type alias follows the existing `SpinnerFunc` pattern so dependents can inject a fake.

## Remaining Tasks

- [ ] #521 — Wire busy spinner into existing status fetch commands ← current
- [ ] #522 — Scaffold gh agentic kanban Cobra command (flag wiring + help)
- [ ] #523 — Implement gh agentic kanban behaviour: stacked default, selectors, JSON envelope
- [ ] #524 — Remove --kanban flag from status requirements / status features
- [ ] #525 — End-to-end verification + JSON schema fixture for combined kanban envelope
