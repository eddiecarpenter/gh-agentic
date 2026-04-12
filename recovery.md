# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #432                               |
| Branch              | feature/432-merge-framework-content |
| Last commit         | cbf474a                            |
| Total tasks         | 5                                  |
| Last updated        | 2026-04-12T22:46:00Z               |

## Completed Tasks

### #433 — Copy framework core content (RULEBOOK.md, skills/, standards/, concepts/) from ai-native-delivery to gh-agentic root
- **Implemented:** Copied RULEBOOK.md, 14 skill files, standards/go.md, and 2 concept files from ai-native-delivery/.ai/ to gh-agentic root
- **Files changed:** RULEBOOK.md, skills/*.md (14 files), standards/go.md, concepts/delivery-philosophy.md, concepts/feature-switches.md
- **Decisions:** None

### #434 — Copy recipes from ai-native-delivery .goose/recipes/ to gh-agentic root recipes/
- **Implemented:** Copied 8 recipe YAML files to recipes/ directory at repo root
- **Files changed:** recipes/*.yaml (8 files)
- **Decisions:** None

### #435 — Merge reusable workflows and composite actions from ai-native-delivery .github/
- **Implemented:** Added sonarcloud.yml. Verified overlapping workflows are identical. Composite actions already match. Preserved gh-agentic-specific files.
- **Files changed:** .github/workflows/sonarcloud.yml
- **Decisions:** Kept gh-agentic publish-release.yml (GoReleaser-based) over ai-native-delivery skeleton version

## Remaining Tasks

- [ ] #436 — Update all internal cross-references from ai-native-delivery to gh-agentic
- [ ] #437 — Add verification script to confirm framework content completeness
