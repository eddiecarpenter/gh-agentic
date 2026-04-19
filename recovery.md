# Recovery State

| Field               | Value                                          |
|---------------------|------------------------------------------------|
| Feature issue       | #589                                           |
| Branch              | feature/589-raw-agent-output-skill-sync        |
| Last commit         | 89095c1                                        |
| Total tasks         | 7                                              |
| Last updated        | 2026-04-19T10:48:00+00:00                      |

## Completed Tasks

### #590 — Add --raw TSV output to status list commands
- **Implemented:** Added the `raw bool` field to `statusListFlags` and the `--raw` flag declaration in `registerStatusListFlags`. Added `writeRequirementsRaw` and `writeFeaturesRaw` writers that emit the tab-separated `number\tstage\ttitle\tblocked_by\towning_repo` shape with `-` for sparse cells. Branched both list handlers (`runStatusRequirements`, `runStatusFeatures`) to the raw writer when `flags.raw` is set. Added `rawField`, `rawBlockedField` helpers and `rawListSeparator`, `rawAbsentValue` constants in `status_requirements.go` so the features renderer reuses them.
- **Files changed:** `internal/cli/status.go`, `internal/cli/status_requirements.go`, `internal/cli/status_features.go`, `internal/cli/status_requirements_test.go`, `internal/cli/status_features_test.go`, `internal/cli/testdata/status_raw/requirements_list.raw`, `internal/cli/testdata/status_raw/features_list.raw`
- **Decisions:** Column order frozen by the goldens — later tasks must not reshuffle. Cells strip embedded tabs/newlines to spaces so the TSV stays parseable. Sparse cells use the `-` sentinel rather than empty strings.

## Remaining Tasks

- [ ] #591 — Add --raw frontmatter+markdown output to status detail commands ← current
- [ ] #592 — Add --raw output to status pipeline command (combined + selectors)
- [ ] #593 — Add --verbose flag for timestamps on --raw output
- [ ] #594 — Remove --json flag and JSON schema tests from status commands
- [ ] #595 — Rewrite skills/gh-agentic-tool.md as agent single source of truth
- [ ] #596 — Add TestGhAgenticToolSkillCoversCLI and Tool/Skill Sync rule
