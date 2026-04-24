# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #624                               |
| Branch              | feature/624-rename-goose-vars      |
| Last commit         | b097257                            |
| Total tasks         | 3                                  |
| Last updated        | 2026-04-24T03:56:33Z               |

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

### #638 — Rename vars.GOOSE_PROVIDER / vars.GOOSE_MODEL in workflow files
- **Implemented:** Replaced every `${{ vars.GOOSE_PROVIDER }}` and
  `${{ vars.GOOSE_MODEL }}` lookup in `.github/workflows/agentic-pipeline.yml`
  (10 sites) and `.github/workflows/release.yml` (2 sites) with
  `${{ vars.AGENT_PROVIDER }}` / `${{ vars.AGENT_MODEL }}`. Env LHS names
  (`GOOSE_PROVIDER:`, `GOOSE_MODEL:`) and the YAML heredoc keys written to
  `~/.config/goose/config.yaml` were deliberately preserved — Goose CLI
  contract. Repo-wide grep for `vars.GOOSE_*` outside `.ai/` and the
  gitignored `.agentic-tools/` cache returns zero matches.
- **Files changed:** .github/workflows/agentic-pipeline.yml,
  .github/workflows/release.yml.
- **Decisions:** Issue had been closed externally without commits; reopened,
  completed the work, and re-closed with the proper commit. `go build` /
  `go test` were not re-run for this task — the change touches only YAML and
  Go is not installed on this self-hosted runner; task #637 already verified
  the Go side end-to-end.

## Remaining Tasks

- [ ] #639 — Document GOOSE_* → AGENT_* rename as a breaking-change migration ← current
