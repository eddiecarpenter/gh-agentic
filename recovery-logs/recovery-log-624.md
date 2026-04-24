# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #624                               |
| Branch              | feature/624-rename-goose-vars      |
| Last commit         | 9817cce                            |
| Total tasks         | 3                                  |
| Last updated        | 2026-04-24T03:58:00Z               |

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

### #639 — Document GOOSE_* → AGENT_* rename as a breaking-change migration
- **Implemented:** Added `docs/migration-agent-vars-rename.md` describing the
  two renames, the canonical default values
  (`AGENT_PROVIDER=claude-code`, `AGENT_MODEL=default`), the required user
  action to set the new variables at the same scope before bumping framework
  version, and the scope note that `GOOSE_AGENT_PAT` is unaffected (handled
  by #622). Commit body carries `BREAKING CHANGE:` footer so
  `skills/release-notes.md` categorises it under Breaking Changes.
  `docs/PROJECT_BRIEF.md` required no change — never referenced the renamed
  variables. Final repo-wide grep confirms zero `GOOSE_PROVIDER` /
  `GOOSE_MODEL` references outside the Goose-CLI contract sites
  (workflow `env:` LHS, heredoc YAML keys) and the migration doc itself.
- **Files changed:** docs/migration-agent-vars-rename.md (new).
- **Decisions:** Issue had been closed externally without commits; reopened,
  completed the work, and re-closed. Same Go-runner caveat as #638 — task
  is doc-only, no Go regression risk.

## Remaining Tasks

_(none — all three tasks complete)_
