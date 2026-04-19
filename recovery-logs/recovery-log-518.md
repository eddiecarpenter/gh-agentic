# Recovery State

| Field               | Value                                              |
|---------------------|----------------------------------------------------|
| Feature issue       | #518                                               |
| Branch              | feature/518-kanban-command-busy-spinner            |
| Last commit         | (pending task 6 commit)                            |
| Total tasks         | 6                                                  |
| Last updated        | 2026-04-19T02:55:00Z                               |

## Completed Tasks

### #519 — Add busy-spinner utility (delayed, non-TTY guarded) — internal/ui/busy.go
- **Implemented:** BusyRun/NoopBusy with full suppression precedence + auto-clear.

### #521 — Wire busy spinner into existing status fetch commands
- **Implemented:** deps.busy threaded through four handlers with stderr plumbing.

### #522 — Scaffold gh agentic kanban Cobra command
- **Implemented:** Full flag surface + stub handler.

### #523 — Implement gh agentic kanban behaviour
- **Implemented:** Real handler, combined JSON envelope with selector-scoped key omission, 15 behavioural tests.

### #524 — Remove --kanban flag from status requirements / status features
- **Implemented:** Hidden --kanban flag with guided migration error; dead layout flags stripped.

### #525 — End-to-end verification + JSON schema fixture for combined kanban envelope
- **Implemented:** New fixture `internal/cli/testdata/status_schemas/kanban_combined_envelope.schema.json` locking the default and selector envelopes plus the inner Requirement/Feature key sets (verbatim match to the existing status schemas per AC-14). New `internal/cli/kanban_json_schema_test.go` with 6 schema-bound tests: `TestKanbanJSON_CombinedEnvelopeSchema`, `*InnerRequirementFields`, `*InnerFeatureFields`, `*RequirementsSelectorOmitsFeaturesKey`, `*FeaturesSelectorOmitsRequirementsKey`, `*JQParseableOutput`. Smoke run against compiled binary verified: `kanban --help`, `status requirements --help` / `status features --help` (no layout flags), `status requirements --kanban` / `status features --kanban` (exit 1 + guided error). All ACs 1–14 now have test coverage; summary posted as closing comment on #525.
- **Files changed:** internal/cli/testdata/status_schemas/kanban_combined_envelope.schema.json, internal/cli/kanban_json_schema_test.go
- **Decisions:** Schema fixture captures key sets for three envelope shapes (default / --requirements selector / --features selector) and six key-set variants (plus inner Requirement / Feature). `keysExactly` helper enforces bidirectional conformance — missing-key and extra-key violations both fatal. Live federated smoke (hitting GitHub) not run — environment has no gh session; covered via build + unit tests and binary help/error paths.

## Remaining Tasks

(none)
