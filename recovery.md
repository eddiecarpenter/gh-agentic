# Recovery State

| Field               | Value                                          |
|---------------------|------------------------------------------------|
| Feature issue       | #589                                           |
| Branch              | feature/589-raw-agent-output-skill-sync        |
| Last commit         | 2545787                                        |
| Total tasks         | 7                                              |
| Last updated        | 2026-04-19T11:05:00+00:00                      |

## Completed Tasks

### #590 — Add --raw TSV output to status list commands
- **Implemented:** Added `raw bool` to `statusListFlags`, `--raw` registration, `writeRequirementsRaw` / `writeFeaturesRaw` writers emitting `number\tstage\ttitle\tblocked_by\towning_repo` with `-` for sparse cells.
- **Files changed:** `internal/cli/status.go`, `internal/cli/status_requirements.go`, `internal/cli/status_features.go`, tests, goldens.
- **Decisions:** Column order frozen by goldens; `rawField` strips embedded tabs/newlines.

### #591 — Add --raw frontmatter+markdown output to status detail commands
- **Implemented:** Added `raw bool` to `statusDetailFlags`, `--raw` registration, `writeRequirementRaw` / `writeFeatureRaw` writers emitting `key: value` header, literal `---`, verbatim body. Empty header values render as `key:` (no trailing space) via `writeRawHeaderLine`.
- **Files changed:** `internal/cli/status.go`, detail writers + tests + goldens.
- **Decisions:** `---` is unconditional; `linked_features` is space-separated numbers; `tasks_done_total` is `done/total`.

### #592 — Add --raw output to status pipeline command (combined + selectors)
- **Implemented:** Added `raw bool` to `pipelineFlags`, `--raw` registration, `writePipelineRaw` emitting `# requirements` / `# features` markers around the Task 1 list TSV. Selectors drop the irrelevant section. `--horizontal` / `--vertical` are no-ops under `--raw`.
- **Files changed:** `internal/cli/pipeline_cmd.go`, tests, three pipeline goldens.
- **Decisions:** Single blank line between sections; layout flags are silently ignored.

### #593 — Add --verbose flag for timestamps on --raw output
- **Implemented:** Added `verbose bool` to `statusListFlags`, `statusDetailFlags`, and `pipelineFlags`. `--verbose` appends `created_at` / `last_transitioned_at` columns to list TSV and inserts the same two `key: value` lines after `owning_repo` in detail headers. ISO-8601 date format. `--verbose` without `--raw` is a tested no-op. Pipeline raw delegates to list raw, so verbose covers both sections automatically.
- **Files changed:** `internal/cli/status.go`, `internal/cli/pipeline_cmd.go`, list/detail writers + tests, five `*_verbose.raw` goldens.
- **Decisions:** Verbose timestamps use `rawTimestampField` (zero → `-` for lists) and `rawDetailTimestamp` (zero → empty for headers, mirroring `formatISODate`).

## Remaining Tasks

- [ ] #594 — Remove --json flag and JSON schema tests from status commands ← current
- [ ] #595 — Rewrite skills/gh-agentic-tool.md as agent single source of truth
- [ ] #596 — Add TestGhAgenticToolSkillCoversCLI and Tool/Skill Sync rule
