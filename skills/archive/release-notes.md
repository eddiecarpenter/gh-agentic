---
name: release-notes
description: Generates human-readable, well-structured release notes from git commit history and updates the GitHub release body with the AI-written notes. Use when the Release recipe fires on a version tag being pushed to main and the release body needs categorised (Features/Fixes/Documentation/Chores) notes.
category: Operation
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# Release Notes

## Purpose

Generate human-readable, well-structured release notes from git commit history
and create the GitHub release. This skill is invoked by the Release recipe when
a version tag is pushed to `main`.

---

## Steps

### Step 1 — Determine the Previous Tag

List all tags sorted by version, excluding the current tag:

```bash
git tag --sort=-version:refname | grep -v "^{{ tag }}$" | head -1
```

Store as PREV_TAG. If no previous tag exists, the release covers the full history.

### Step 2 — Collect Commits

Get all commits since the previous tag:

```bash
# If PREV_TAG exists:
git log --pretty=format:"%s (%h)" "${PREV_TAG}..HEAD"

# If no PREV_TAG:
git log --pretty=format:"%s (%h)"
```

Review the commits. Read changed files if a commit message is ambiguous.

### Step 3 — Detect Migration Guides and Prepare the Required-Action Callout

Release notes for any version that ships a new migration guide must surface the
migration as a **required action for downstream consumers**, at the very top of
the notes, so owners of domain repos cannot miss it. This step detects those
migrations automatically from the file history in the release range — no
per-release human edits required.

**Detection.** Scan the release range for commits that add or modify any file
matching `concepts/migration-*.md` or `docs/migration-*.md`. Use `git diff` with
status filters to separate adds from other modifications:

```bash
# If PREV_TAG exists:
git diff --name-status --diff-filter=A "${PREV_TAG}..HEAD" -- \
  'concepts/migration-*.md' 'docs/migration-*.md'

# If no PREV_TAG:
git log --name-status --diff-filter=A --pretty=format: -- \
  'concepts/migration-*.md' 'docs/migration-*.md' | sort -u
```

The `--diff-filter=A` restricts output to newly **added** files — minor edits
to an existing migration doc do not re-emit the callout in every subsequent
release. Store the list as `ADDED_MIGRATIONS`.

**Decision.**
- If `ADDED_MIGRATIONS` is empty → emit no callout; Step 4 proceeds normally
  (preserves the existing behaviour for releases with no migration work).
- If `ADDED_MIGRATIONS` is non-empty → Step 4 must prepend a
  `## ⚠️ Required Action` section naming each added migration doc by title and
  linking it at the release tag (`https://github.com/{{ repo }}/blob/{{ tag }}/<path>`).
  Title is taken from the doc's first-line `# …` heading.

**Worked example.** For the release that ships the `goose-agent` → GitHub App
identity swap, `git diff --diff-filter=A` surfaces
`concepts/migration-to-github-app.md`. The expected callout at the top of the
release notes is:

```markdown
## ⚠️ Required Action

This release ships a breaking identity change. Domain repos **must** perform the
following migration before upgrading:

- **[Migration — `goose-agent` PAT identity replaced by the agentic GitHub App](https://github.com/eddiecarpenter/gh-agentic/blob/v1.2.3/concepts/migration-to-github-app.md)**
  — install the agentic GitHub App, set `AGENTIC_APP_ID` and `AGENTIC_APP_PRIVATE_KEY`,
  mount the new framework version, and retire the legacy PAT credentials.

Skipping this migration before upgrading will cause every workflow run to fail
on token minting. See the guide for the full cutover sequence and verification
checklist.
```

If multiple migration docs are added in the same release, emit one bullet per
doc under the same `## ⚠️ Required Action` heading.

### Step 4 — Write the Release Notes

Release notes are read by humans — developers deciding whether to sync, product
owners reviewing what changed. They are not a raw commit log. They answer:
**what changed, and why does it matter?**

**Format:**

```markdown
<One sentence summary of what this release delivers overall.>

## ⚠️ Required Action   ← only if Step 3 produced a non-empty ADDED_MIGRATIONS list
- <One bullet per added migration guide, linking it at the release tag>

## Features
- <What the feature does and why it matters>

## Fixes
- <What was broken and what is now correct>

## Documentation
- <What was documented or clarified>

## Chores
- <Infrastructure, dependency updates, tooling — only if notable>
```

**Rules:**
- Omit any section that has no entries
- Do not include the release tag or a title — GitHub adds the title separately
- Each bullet is one sentence, present tense: "Adds...", "Fixes...", "Removes..."
- Name the thing that changed, not the mechanism
- **Migration callout placement is fixed.** When Step 3 reports added migration
  docs, the `## ⚠️ Required Action` section is emitted **immediately after** the
  one-sentence summary and **before** `## Features`. Do not fold migration links
  into the `## Documentation` section — a migration guide is a required-action
  signal, not a documentation improvement, and must retain its prominence at the
  top of the notes. When Step 3 reports no added migration docs, omit the
  `## ⚠️ Required Action` section entirely (no-op; preserves existing behaviour).

**Commit categorisation:**

| Commit prefix | Section |
|---|---|
| `feat:` | Features |
| `fix:` | Fixes |
| `docs:` | Documentation |
| `chore:`, `ci:`, `refactor:`, `test:` | Chores (only if notable) |
| Merge commits, version bumps, automated commits | Omit |

**What to omit:**
- Merge commits (`Merge pull request #N from ...`)
- Automated commits (`chore: update .ai/config.yml`, `chore: sync ...`)
- Minor CI or tooling changes with no user impact
- Commits that duplicate another commit in the same release

**For framework releases specifically:**
Flag any changes that require downstream action — new required secrets, changed
RULEBOOK.md rules, renamed or new skills, breaking changes to recipe parameters.
These are the changes downstream owners most need to know about before running
`gh agentic sync`.

Write the release notes to `/tmp/release-notes.md`.

### Step 5 — Update the GitHub Release

The release already exists — it was created by the local project's publish workflow
(GoReleaser, a stub creation step, or any other means). Update its body with the
AI-generated notes:

```bash
gh release edit {{ tag }} \
  --repo {{ repo }} \
  --notes-file /tmp/release-notes.md
```

This replaces whatever notes the release was created with. If the release does not
exist for any reason, report and exit cleanly — do not attempt to create it.

### Step 6 — Report

Output the GitHub release URL and a one-line summary of what was included.
