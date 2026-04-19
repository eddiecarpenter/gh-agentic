# Recovery State

| Field               | Value                                          |
|---------------------|------------------------------------------------|
| Feature issue       | #589                                           |
| Branch              | feature/589-raw-agent-output-skill-sync        |
| Last commit         | bf72758                                        |
| Total tasks         | 7                                              |
| Last updated        | 2026-04-19T11:25:00+00:00                      |

## Completed Tasks

### #590 — Add --raw TSV output to status list commands
- **Implemented:** `--raw` on `status requirements` / `status features` emits `number\tstage\ttitle\tblocked_by\towning_repo` with `-` for sparse cells.
- **Files changed:** `internal/cli/status.go`, list writers + tests + goldens.
- **Decisions:** Column order frozen by goldens.

### #591 — Add --raw frontmatter+markdown output to status detail commands
- **Implemented:** `--raw` on detail commands emits frontmatter `key: value` header, `---` separator, verbatim body. Empty values render as `key:` (no trailing space) via `writeRawHeaderLine`.
- **Files changed:** `internal/cli/status.go`, detail writers + tests + goldens.
- **Decisions:** `---` is unconditional. `linked_features` is space-separated numbers. `tasks_done_total` is `done/total`.

### #592 — Add --raw output to status pipeline command (combined + selectors)
- **Implemented:** `--raw` on pipeline emits `# requirements` / `# features` markers around the Task 1 list TSV. Selectors drop the irrelevant section. `--horizontal` / `--vertical` are no-ops under `--raw`.
- **Files changed:** `internal/cli/pipeline_cmd.go`, tests, three pipeline goldens.

### #593 — Add --verbose flag for timestamps on --raw output
- **Implemented:** `--verbose` appends `created_at` / `last_transitioned_at` columns to list TSV and inserts the same two lines after `owning_repo` in detail headers (ISO date). `--verbose` without `--raw` is a tested no-op.
- **Files changed:** all five `--raw`-supporting commands + tests + five `*_verbose.raw` goldens.

### #594 — Remove --json flag and JSON schema tests from status commands
- **Implemented:** Removed `--json` end-to-end. Dropped the flag, the writers (`write*JSON`), the envelope types (`pipelineJSONEnvelope`, `pipelineJSONTotals`, `projectstatus.ListEnvelope`, `projectstatus.ListTotals`), every `--json` reference in command help/examples, and every JSON-shape test (`status_json_schema_test.go`, `pipeline_json_schema_test.go`, `internal/cli/testdata/status_schemas/`, individual JSON tests in the per-command test files, the `JSONMatchesSchema` integration tests). Added `status_no_json_test.go` as the AC-1 regression — every status sub-command must respond `unknown flag: --json` with a non-zero exit. Scrubbed `docs/status-verification.md` to use `--raw`.
- **Files changed:** 25 files (8 modified Go source, 6 modified Go tests, 2 deleted Go test files, 1 new Go test file, 5 deleted JSON schema fixtures, `docs/status-verification.md`).
- **Decisions:** Skill / GitHub-CLI references to `--json` (e.g. `gh issue list --json`, `gh variable list --json`) are intentionally left in place — they reference the GitHub CLI's own flag, not `gh agentic`'s. Code-level documentation that explains the removal still mentions `--json` in past tense.

## Remaining Tasks

- [ ] #595 — Rewrite skills/gh-agentic-tool.md as agent single source of truth ← current
- [ ] #596 — Add TestGhAgenticToolSkillCoversCLI and Tool/Skill Sync rule
