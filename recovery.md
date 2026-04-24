# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #652                               |
| Branch              | feature/652-design-plan-comment    |
| Last commit         | f536940                            |
| Total tasks         | 2                                  |
| Last updated        | 2026-04-24T08:32:00Z               |

## Current Task

### #661 — Wire feature-design.md to publish Design Plan before tasks, halt on failure, amend with #N
- **Status:** in-progress
- **Last step:** edited skills/feature-design.md — added capture-design-plan to loads (between refactor-assessment and session-exit), inserted new step 5 (Publish Design Plan comment) with halt-on-failure wording naming the three blocked actions (task creation, branch creation, in-development label), renumbered existing steps to 6–12, inserted new step 7 (Amend Design Plan comment with #N references — append-only via gh api PATCH), updated step 11 exit block to include `Design Plan comment: <url>` line above refactor-assessment line, added Rules bullet blocking task emission until publish succeeds. Touched CATALOGUE.md to refresh mtime (no content change — loads is not rendered into the catalogue).
- **Next step:** create internal/frameworkcheck/feature_design_sapav_test.go verifying loads contains capture-design-plan, publish precedes task-creation, amend follows task-creation, halt wording names the three blocked actions, exit block shows Design Plan comment line
- **Notes:** Reuse check complete — outcome: as-is — gh issue comment (native CLI) for publish/amend; existing feature-design halt-on-missing-AC pattern for halt-on-publish-failure

## Completed Tasks

### #660 — Create skills/capture-design-plan.md — canonical Design Plan template
- **Implemented:** Created the Reference-category skill `skills/capture-design-plan.md` defining the Markdown template feature-design publishes as a Design Plan comment before Task creation. Sections: Decomposition, Tasks (with `[planned]` placeholders), Alternatives Considered (with literal `_None — single obvious decomposition._` fallback), Refactor Assessment, optional Codebase Findings, optional Risks. Word-count bounds 300–500 soft / 1000 hard documented in Rules. Append-only amendment discipline (`Tasks (created)` subsection) documented. Regenerated `CATALOGUE.md` to list the new skill alphabetically under Reference. Added Go test file `internal/frameworkcheck/capture_design_plan_test.go`.
- **Files changed:** skills/capture-design-plan.md, CATALOGUE.md, internal/frameworkcheck/capture_design_plan_test.go
- **Decisions:** Skill is Reference-category. Amendment offers two approaches (edit original vs follow-up comment); feature-design.md picks one deterministically (#661 selects edit-original via gh api PATCH).

## Remaining Tasks

- [ ] #661 — Wire feature-design.md to publish Design Plan before tasks, halt on failure, amend with #N ← current
