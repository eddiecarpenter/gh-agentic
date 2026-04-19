# Recovery State

| Field               | Value                                          |
|---------------------|------------------------------------------------|
| Feature issue       | #589                                           |
| Branch              | feature/589-raw-agent-output-skill-sync        |
| Last commit         | 4054a6f                                        |
| Total tasks         | 7                                              |
| Last updated        | 2026-04-19T10:53:00+00:00                      |

## Completed Tasks

### #590 — Add --raw TSV output to status list commands
- **Implemented:** Added `raw bool` to `statusListFlags`, declared `--raw` in `registerStatusListFlags`, and added `writeRequirementsRaw` / `writeFeaturesRaw` writers that emit `number\tstage\ttitle\tblocked_by\towning_repo` with `-` for sparse cells.
- **Files changed:** `internal/cli/status.go`, `internal/cli/status_requirements.go`, `internal/cli/status_features.go`, `internal/cli/status_requirements_test.go`, `internal/cli/status_features_test.go`, `internal/cli/testdata/status_raw/{requirements,features}_list.raw`
- **Decisions:** Column order frozen by goldens — later tasks must not reshuffle. `rawField` strips embedded `\t` / `\n` to spaces. Sparse cells use the `-` sentinel.

### #591 — Add --raw frontmatter+markdown output to status detail commands
- **Implemented:** Added `raw bool` to `statusDetailFlags`, declared `--raw` in `registerStatusDetailFlags`, and added `writeRequirementRaw` / `writeFeatureRaw` writers that emit a frontmatter-style header, the literal `---` separator, and the verbatim issue body. Empty header values render as `key:` (no trailing space) via the shared `writeRawHeaderLine` helper.
- **Files changed:** `internal/cli/status.go`, `internal/cli/status_requirement.go`, `internal/cli/status_feature.go`, `internal/cli/status_requirement_test.go`, `internal/cli/status_feature_test.go`, `internal/cli/testdata/status_raw/{requirement,feature}_detail.raw`
- **Decisions:** `---` separator is unconditional (also for empty body). `linked_features` is space-separated feature numbers (no `#`). `tasks_done_total` is `done/total`. Body is emitted byte-for-byte; trailing newline is added only when the source body lacks one.

## Remaining Tasks

- [ ] #592 — Add --raw output to status pipeline command (combined + selectors) ← current
- [ ] #593 — Add --verbose flag for timestamps on --raw output
- [ ] #594 — Remove --json flag and JSON schema tests from status commands
- [ ] #595 — Rewrite skills/gh-agentic-tool.md as agent single source of truth
- [ ] #596 — Add TestGhAgenticToolSkillCoversCLI and Tool/Skill Sync rule
