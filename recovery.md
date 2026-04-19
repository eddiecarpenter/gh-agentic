# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #562                               |
| Branch              | feature/562-rename-kanban-pipeline |
| Last commit         | 0095e52                            |
| Total tasks         | 4                                  |
| Last updated        | 2026-04-19T05:06:00Z               |

## Completed Tasks

### #563 — Rename kanban → pipeline across Go code, tests, and testdata
- **Implemented:** File renames via `git mv` (pipeline.go, pipeline_cmd.go and _test counterparts, plus the testdata JSON schema fixture); every identifier listed in the task renamed at declaration and every caller; Cobra Use/Short/Long/Example on the sub-command now reference `pipeline`; error sentinel renamed (`errKanbanFlagRemoved` → `errPipelineCommandRenamed`); migration pointers emit `gh agentic status pipeline --requirements` / `--features`; renderer headings read `=== Requirements — Pipeline ===` and `=== Features — Pipeline ===`.
- **Files changed:** internal/cli/pipeline.go, pipeline_cmd.go, pipeline_test.go, pipeline_cmd_test.go, pipeline_json_schema_test.go, status.go, status_errors.go, status_errors_test.go, status_features.go, status_json_schema_test.go, status_requirements.go, status_test.go, root.go, testdata/status_schemas/pipeline_combined_envelope.schema.json, internal/projectstatus/types.go.
- **Decisions:** Preserved the legacy `--kanban` flag intercept surface because the legacy flag name is what users type. Deferred the `Requirements Kanban` / `Features Kanban` view-name decision to task #565.

### #564 — Fix rune-aware cell alignment in writeHorizontalPipeline + regression test
- **Implemented:** Swapped `len()` → `utf8.RuneCountInString` on every display-width site in `writeHorizontalPipeline`; rewrote `truncateString` to operate on a `[]rune` slice so a mid-rune cut is structurally impossible and output is always valid UTF-8. Added `internal/cli/pipeline_alignment_test.go` with two regression tests verified failing against the pre-fix code.
- **Files changed:** internal/cli/pipeline.go, internal/cli/pipeline_alignment_test.go.
- **Decisions:** `utf8.RuneCountInString` is correct for the `→` case and all BMP single-width runes; CJK double-width correctness via `go-runewidth` is out of scope. No new dependencies.

### #565 — Update documentation and skills vocabulary: kanban → pipeline
- **Implemented:** Updated command-table rows in README.md, docs/PROJECT_BRIEF.md, docs/ARCHITECTURE.md, LOCALRULES.md to `gh agentic status pipeline`. Updated docs/ARCHITECTURE.md source-tree diagram to `pipeline.go`. Renamed the `## Kanban` section in docs/status-verification.md to `## Pipeline` and updated every manual-verification example.
- **Files changed:** README.md, docs/PROJECT_BRIEF.md, docs/ARCHITECTURE.md, docs/status-verification.md, LOCALRULES.md.
- **Decisions:** Preserved the "Requirements Kanban" / "Features Kanban" view names in `internal/project/assets/project-template.json` (and their mirrored test fixtures + the illustrative example in `skills/update-project-template.md`). Two reasons documented in the commit message: (1) deployed projects already carry views with these literal names — renaming would cause `gh agentic repair` to create new Pipeline-named views, orphaning the old Kanban-named ones; (2) "Requirements Kanban" honestly describes a GitHub Projects `BOARD_LAYOUT` (kanban-style) view so the word sits on the boundary of GitHub-API-imposed terminology.

## Remaining Tasks

- [ ] #566 — Final verification: catalogue regeneration, grep audit, build & test ← current
