# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #624                               |
| Branch              | feature/624-rename-goose-vars      |
| Last commit         | 22a1876                            |
| Total tasks         | 3                                  |
| Last updated        | 2026-04-24T03:35:00Z               |

## Completed Tasks

### #637 — Rename GOOSE_PROVIDER/GOOSE_MODEL identifiers across Go source and tests
- **Implemented:** Renamed framework-managed variable identifiers across
  `internal/scope`, `internal/init`, and `internal/doctor`: sharedNames map,
  InitConfig struct fields, DefaultAgent* constants, pendingDescriptions map,
  checkVariablesAndSecrets variable list, FindShadowValues shared slice,
  and wizard UI titles. Tests updated in parallel. `go build ./...` and
  `go test ./...` both pass.
- **Files changed:** internal/scope/scope.go, internal/scope/scope_test.go,
  internal/init/runner.go, internal/init/wizard.go, internal/init/wizard_test.go,
  internal/init/form.go, internal/init/form_test.go, internal/doctor/checks.go,
  internal/doctor/repair.go, internal/doctor/repair_test.go, go.mod (tidy).
- **Decisions:** Constant values ("claude-code", "default") are unchanged —
  those are the Goose CLI's own expected identifiers. Only the constant/field
  names changed (GOOSE_* → AGENT_*).

## Remaining Tasks

- [ ] #638 — Rename vars.GOOSE_PROVIDER / vars.GOOSE_MODEL in workflow files ← current
- [ ] #639 — Document GOOSE_* → AGENT_* rename as a breaking-change migration
