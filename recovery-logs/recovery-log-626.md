# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #626                               |
| Branch              | feature/626-migration-guide-goose-agent |
| Last commit         | aa845a7                            |
| Total tasks         | 3                                  |
| Last updated        | 2026-04-24T05:34:59Z               |

## Completed Tasks

### #647 — Author migration guide concepts/migration-to-github-app.md
- **Implemented:** Created `concepts/migration-to-github-app.md` — an end-to-end migration guide for existing domain repos to move off the `goose-agent` PAT identity onto the agentic GitHub App. Mirrors the style of `docs/migration-agent-vars-rename.md` and cross-references `docs/github-app-setup.md` rather than duplicating install/config details.
- **Files changed:** concepts/migration-to-github-app.md
- **Decisions:** None

### #648 — Reference migration guide from LOCALRULES.md
- **Implemented:** Appended a `## Migration` section to `LOCALRULES.md` with one-line links to `concepts/migration-to-github-app.md` and the sibling `docs/migration-agent-vars-rename.md`. No step-by-step content duplicated.
- **Files changed:** LOCALRULES.md
- **Decisions:** None

### #649 — Update release-notes skill to call out the goose-agent migration as a required action
- **Implemented:** Added a new Step 3 to `skills/release-notes.md` that detects newly added `concepts/migration-*.md` / `docs/migration-*.md` files in the release range via `git diff --diff-filter=A` and emits a `## ⚠️ Required Action` callout at the top of the release notes. Includes a worked example for the goose-agent → App migration, fixes the callout's placement in the Rules block, and renumbers the remaining steps. Existing Features/Fixes/Documentation/Chores categorisation unchanged.
- **Files changed:** skills/release-notes.md
- **Decisions:** None

## Remaining Tasks

_All tasks complete._
