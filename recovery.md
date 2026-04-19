# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #562                               |
| Branch              | feature/562-rename-kanban-pipeline |
| Last commit         | e6d0912                            |
| Total tasks         | 4                                  |
| Last updated        | 2026-04-19T05:02:00Z               |

## Completed Tasks

### #563 — Rename kanban → pipeline across Go code, tests, and testdata
- **Implemented:** File renames via `git mv` (pipeline.go, pipeline_cmd.go and _test counterparts, plus the testdata JSON schema fixture); every identifier listed in the task renamed at declaration and every caller; Cobra Use/Short/Long/Example on the sub-command now reference `pipeline`; error sentinel renamed (`errKanbanFlagRemoved` → `errPipelineCommandRenamed`); migration pointers emit `gh agentic status pipeline --requirements` / `--features`; renderer headings read `=== Requirements — Pipeline ===` and `=== Features — Pipeline ===`.
- **Files changed:** internal/cli/pipeline.go, pipeline_cmd.go, pipeline_test.go, pipeline_cmd_test.go, pipeline_json_schema_test.go, status.go, status_errors.go, status_errors_test.go, status_features.go, status_json_schema_test.go, status_requirements.go, status_test.go, root.go, testdata/status_schemas/pipeline_combined_envelope.schema.json, internal/projectstatus/types.go.
- **Decisions:** Preserved the legacy `--kanban` flag intercept surface (`statusListFlags.kanban` field, `registerRemovedKanbanFlag`, and the `--kanban has been removed from this command` message text) because the legacy flag name is what users type — renaming would break the intercept. Preserved the `Requirements Kanban` / `Features Kanban` view names in `internal/project/assets/project-template.json` and their mirrored test fixtures for task #565 to decide on.

### #564 — Fix rune-aware cell alignment in writeHorizontalPipeline + regression test
- **Implemented:** Swapped `len()` → `utf8.RuneCountInString` on every display-width site in `writeHorizontalPipeline` (top-border label fit check, dashes computation, content-row truncation, content-row padding); rewrote `truncateString` to operate on a `[]rune` slice so a mid-rune cut is structurally impossible and the output is always valid UTF-8. Added `internal/cli/pipeline_alignment_test.go` with two regression tests: `TestHorizontalPipeline_MultiByteRuneRowsStayAligned` (renders issue #467's `promote local → framework` at 252 cols and asserts rune-count parity plus boundary-glyph alignment across top border, content rows, and bottom border), and `TestTruncateString_IsRuneSafe` (asserts rune-safe clipping across pure multi-byte input). Both tests verified failing against the pre-fix code and passing after the fix.
- **Files changed:** internal/cli/pipeline.go, internal/cli/pipeline_alignment_test.go.
- **Decisions:** `utf8.RuneCountInString` is correct for the `→` case and all BMP single-width runes; CJK / emoji double-width correctness via `go-runewidth` is out of scope per #562. No new dependencies.

## Remaining Tasks

- [ ] #565 — Update documentation and skills vocabulary: kanban → pipeline ← current
- [ ] #566 — Final verification: catalogue regeneration, grep audit, build & test
