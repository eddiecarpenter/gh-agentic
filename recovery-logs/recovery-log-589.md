# Recovery State

| Field               | Value                                          |
|---------------------|------------------------------------------------|
| Feature issue       | #589                                           |
| Branch              | feature/589-raw-agent-output-skill-sync        |
| Last commit         | cbd0ddc                                        |
| Total tasks         | 7                                              |
| Last updated        | 2026-04-19T11:38:00+00:00                      |

## Completed Tasks

### #590 — Add --raw TSV output to status list commands
- **Implemented:** `--raw` on `status requirements` / `status features` emits `number\tstage\ttitle\tblocked_by\towning_repo` with `-` for sparse cells.

### #591 — Add --raw frontmatter+markdown output to status detail commands
- **Implemented:** `--raw` on detail commands emits frontmatter `key: value` header, `---` separator, verbatim body. Empty values render as `key:` (no trailing space) via `writeRawHeaderLine`.

### #592 — Add --raw output to status pipeline command (combined + selectors)
- **Implemented:** `--raw` on pipeline emits `# requirements` / `# features` markers around the Task 1 list TSV. Selectors drop the irrelevant section. `--horizontal` / `--vertical` are no-ops under `--raw`.

### #593 — Add --verbose flag for timestamps on --raw output
- **Implemented:** `--verbose` appends `created_at` / `last_transitioned_at` columns to list TSV and inserts the same two lines after `owning_repo` in detail headers (ISO date). `--verbose` without `--raw` is a tested no-op.

### #594 — Remove --json flag and JSON schema tests from status commands
- **Implemented:** Removed `--json` end-to-end. Dropped the flag, the writers, the envelope types, every `--json` reference in command help/examples, and every JSON-shape test. Added `status_no_json_test.go` as the AC-1 regression. Scrubbed `docs/status-verification.md`.

### #595 — Rewrite skills/gh-agentic-tool.md as agent single source of truth
- **Implemented:** Wholesale rewrite of `skills/gh-agentic-tool.md`. Documents every command in the cobra tree (init / check / repair / mount / upgrade / info / auth\* / project\* / status\*), every declared flag, the `--raw` contract (list shape / single-item shape / pipeline shape / `--verbose`), and a Common Agent Questions decision matrix with concrete recipes. Header note about `TestGhAgenticToolSkillCoversCLI` is at the top.
- **Files changed:** `skills/gh-agentic-tool.md`
- **Decisions:** Stale entries from the task body removed — there is no `gh agentic doctor` command, and `gh agentic project check / repair / info` do not exist (those are top-level commands). Authoritative source is the cobra wiring in `internal/cli/`.

## Remaining Tasks

- [ ] #596 — Add TestGhAgenticToolSkillCoversCLI and Tool/Skill Sync rule ← current
