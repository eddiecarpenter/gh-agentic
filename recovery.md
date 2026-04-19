# Recovery State

| Field               | Value                                          |
|---------------------|------------------------------------------------|
| Feature issue       | #589                                           |
| Branch              | feature/589-raw-agent-output-skill-sync        |
| Last commit         | d304f2c                                        |
| Total tasks         | 7                                              |
| Last updated        | 2026-04-19T10:58:00+00:00                      |

## Completed Tasks

### #590 — Add --raw TSV output to status list commands
- **Implemented:** Added `raw bool` to `statusListFlags`, declared `--raw` in `registerStatusListFlags`, and added `writeRequirementsRaw` / `writeFeaturesRaw` writers that emit `number\tstage\ttitle\tblocked_by\towning_repo` with `-` for sparse cells.
- **Files changed:** `internal/cli/status.go`, `internal/cli/status_requirements.go`, `internal/cli/status_features.go`, `internal/cli/status_requirements_test.go`, `internal/cli/status_features_test.go`, `internal/cli/testdata/status_raw/{requirements,features}_list.raw`
- **Decisions:** Column order frozen by goldens. `rawField` strips embedded `\t` / `\n` to spaces. Sparse cells use `-`.

### #591 — Add --raw frontmatter+markdown output to status detail commands
- **Implemented:** Added `raw bool` to `statusDetailFlags`, declared `--raw`, and added `writeRequirementRaw` / `writeFeatureRaw` writers that emit a frontmatter-style header, a literal `---` separator, and the verbatim issue body. Empty header values render as `key:` (no trailing space) via the shared `writeRawHeaderLine` helper.
- **Files changed:** `internal/cli/status.go`, `internal/cli/status_requirement.go`, `internal/cli/status_feature.go`, `internal/cli/status_requirement_test.go`, `internal/cli/status_feature_test.go`, `internal/cli/testdata/status_raw/{requirement,feature}_detail.raw`
- **Decisions:** `---` separator is unconditional. `linked_features` is space-separated feature numbers. `tasks_done_total` is `done/total`. Body is byte-for-byte verbatim.

### #592 — Add --raw output to status pipeline command (combined + selectors)
- **Implemented:** Added `raw bool` to `pipelineFlags`, declared `--raw`, and added `writePipelineRaw` that emits `# requirements` / `# features` section markers around the Task 1 list TSV. Selectors drop the irrelevant section entirely. `--horizontal` / `--vertical` are no-ops under `--raw` (layout resolution is skipped). Per-section row formatting is delegated to `writeRequirementsRaw` / `writeFeaturesRaw` so the goldens stay in lock-step.
- **Files changed:** `internal/cli/pipeline_cmd.go`, `internal/cli/pipeline_cmd_test.go`, `internal/cli/testdata/status_raw/{pipeline_combined,pipeline_requirements,pipeline_features}.raw`
- **Decisions:** Single blank line between sections; the `# features` marker is sufficient visual separation. Layout flags are silently ignored under `--raw` rather than erroring.

## Remaining Tasks

- [ ] #593 — Add --verbose flag for timestamps on --raw output ← current
- [ ] #594 — Remove --json flag and JSON schema tests from status commands
- [ ] #595 — Rewrite skills/gh-agentic-tool.md as agent single source of truth
- [ ] #596 — Add TestGhAgenticToolSkillCoversCLI and Tool/Skill Sync rule
