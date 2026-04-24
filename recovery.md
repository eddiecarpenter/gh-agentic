# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #652                               |
| Branch              | feature/652-design-plan-comment    |
| Last commit         | 4d43220                            |
| Total tasks         | 2                                  |
| Last updated        | 2026-04-24T08:35:00Z               |

## Completed Tasks

### #660 — Create skills/capture-design-plan.md — canonical Design Plan template
- **Implemented:** Created the Reference-category skill `skills/capture-design-plan.md` defining the Markdown template feature-design publishes as a Design Plan comment before Task creation. Sections: Decomposition, Tasks (with `[planned]` placeholders), Alternatives Considered (with literal `_None — single obvious decomposition._` fallback), Refactor Assessment, optional Codebase Findings, optional Risks. Word-count bounds 300–500 soft / 1000 hard documented in Rules. Append-only amendment discipline (`Tasks (created)` subsection) documented. Regenerated `CATALOGUE.md` to list the new skill alphabetically under Reference. Added Go test file `internal/frameworkcheck/capture_design_plan_test.go`.
- **Files changed:** skills/capture-design-plan.md, CATALOGUE.md, internal/frameworkcheck/capture_design_plan_test.go
- **Decisions:** Skill is Reference-category. Amendment offers two approaches (edit original vs follow-up comment); feature-design.md picks the edit-original approach via `gh api PATCH` deterministically.

### #661 — Wire feature-design.md to publish Design Plan before tasks, halt on failure, amend with #N
- **Implemented:** Edited `skills/feature-design.md` — added `capture-design-plan` to `loads` (between `refactor-assessment` and `session-exit`); inserted new step 5 (Publish Design Plan comment via `gh issue comment --body-file`, with halt-on-failure wording `REFUSED: Design Plan publication failed — halting before task creation` naming the three blocked actions: task creation, branch creation, in-development label); renumbered existing steps 5–10 to 6–12; inserted new step 7 (Amend Design Plan comment with `#N` references via `gh api PATCH`, append-only `### Tasks (created)` subsection); updated step 11 exit-block example to include `Design Plan comment: <url>` line above the Refactor Assessment line; added a Rules bullet blocking task emission until publish succeeds. Refreshed `CATALOGUE.md` mtime (no content change — `loads` is not rendered). Added Go test `internal/frameworkcheck/feature_design_sapav_test.go` verifying loads contains `capture-design-plan`, publish precedes task-creation via index comparison, amend follows task-creation with append-only discipline, halt wording names the three blocked actions, and exit block contains `Design Plan comment:` line.
- **Files changed:** skills/feature-design.md, CATALOGUE.md, internal/frameworkcheck/feature_design_sapav_test.go
- **Decisions:** Amend approach locked to "edit original comment" via `gh api PATCH` — chosen deterministically so the agent does not re-decide per feature; the append-only rule from capture-design-plan preserves the audit trail either way.

## Remaining Tasks

_(none — all tasks complete)_
