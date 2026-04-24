# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #652                               |
| Branch              | feature/652-design-plan-comment    |
| Last commit         | 9a9007d                            |
| Total tasks         | 2                                  |
| Last updated        | 2026-04-24T08:30:00Z               |

## Completed Tasks

### #660 — Create skills/capture-design-plan.md — canonical Design Plan template
- **Implemented:** Created the Reference-category skill `skills/capture-design-plan.md` defining the Markdown template feature-design publishes as a Design Plan comment before Task creation. Sections: Decomposition, Tasks (with `[planned]` placeholders), Alternatives Considered (with literal `_None — single obvious decomposition._` fallback), Refactor Assessment, optional Codebase Findings, optional Risks. Word-count bounds 300–500 soft / 1000 hard documented in Rules. Append-only amendment discipline (`Tasks (created)` subsection) documented. Regenerated `CATALOGUE.md` to list the new skill alphabetically under Reference. Added Go test file `internal/frameworkcheck/capture_design_plan_test.go` verifying frontmatter conformance, required section headings, literal fallback phrase, word-count bounds, append-only amendment wording, and catalogue listing.
- **Files changed:** skills/capture-design-plan.md, CATALOGUE.md, internal/frameworkcheck/capture_design_plan_test.go
- **Decisions:** Skill is Reference-category (not Operation) — it defines a template consumed by feature-design, not a procedure. Amendment offers two approaches (edit original comment vs follow-up comment); feature-design.md (#661) will pick one deterministically. Cross-repo references deliberately excluded from template — Tasks always live in the same repo as the Feature.

## Remaining Tasks

- [ ] #661 — Wire feature-design.md to publish Design Plan before tasks, halt on failure, amend with #N ← current
