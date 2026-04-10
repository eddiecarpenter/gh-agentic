Major structural release that reorganises the template directory layout and introduces local override support.

## Features
- Restructures the template base directory from `base/` to `.ai/`, renames `AGENTS.md` to `RULEBOOK.md`, and replaces `TEMPLATE_VERSION`/`TEMPLATE_SOURCE` with `.ai/config.yml` (#184)
- Introduces `LOCALRULES.md` as the designated file for project-specific rule overrides, making `AGENTS.md` a read-only template-managed file (#193)
- Makes `REPOS.md` optional so single-repo projects can skip multi-repo configuration in session-init (#192)

## Fixes
- Sets project status to Done and removes the scoping label when auto-closing a parent requirement issue (#195)
- Guards the git commit step in publish-release when the version is unchanged and skips Stage 5 for non-issue branches (#194)

## ⚠️ Downstream Action Required
- **Directory rename:** All references to `base/` must be updated to `.ai/` — run `gh agentic sync` to pull this change
- **Local overrides:** Any customisations previously added to `AGENTS.md` must be moved to `LOCALRULES.md` — `AGENTS.md` is now template-managed and will be overwritten on sync
- **Config migration:** `TEMPLATE_VERSION` and `TEMPLATE_SOURCE` are replaced by `.ai/config.yml` — sync handles this automatically
