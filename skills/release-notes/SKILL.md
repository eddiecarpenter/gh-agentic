---
name: release-notes
description: Generates human-readable, well-structured release notes from the git commit history between the current tag and the previous one, then updates the existing GitHub release body with the AI-written notes. Categorises commits as Features / Fixes / Documentation / Chores; detects newly-added migration docs and prepends a `## ⚠️ Required Action` callout linking each one. Use when a release workflow fires on a version tag pushed to main and the release body needs structured notes. Use even when the caller doesn't say "release notes" — phrases like "generate release notes for v1.2.3", "update the release body", "publish notes for the latest tag" should trigger this skill.
triggers: automated
user-invocable: true
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/step-skip-rule.md
emits-exit-block: false
---

# Release Notes

## Goal

Take the commits in a release range (current tag → previous tag),
write human-readable notes that answer "what changed and why does
it matter?", and update the existing GitHub release body with
those notes. The release itself is created by the publish workflow
(GoReleaser or similar); this skill does not create releases — it
only edits an existing one's body.

The output is read by humans deciding whether to sync, product
owners reviewing what changed, and downstream-repo owners deciding
whether the release requires action on their side. The notes are
NOT a raw commit log.

## Output Artefacts

- A release notes document in canonical form, written to
  `/tmp/release-notes.md` and applied to the GitHub release via
  `gh release edit <tag> --notes-file`.
- Optionally, a `## ⚠️ Required Action` section at the top, when
  the release range contains newly-added migration docs.
- A printed one-line summary at the end of the run with the
  release URL and the section counts.

The skill's three valid terminal outputs:

**A. Notes published.** Commits were collected, categorised, and
the GitHub release body was updated. Migration callout was
emitted if applicable.

**B. No notable commits.** The range contained only merge commits,
automated chores, and version bumps. The skill emits a one-line
notes body ("Internal release — no human-visible changes since
<prev>") and updates the release.

**C. Release missing.** The named tag does not have a corresponding
GitHub release. The skill exits cleanly without creating one — the
publish workflow's responsibility, not this skill's. Emit a clear
diagnosis.

## Definitions

- `skills/definitions/error-handling.md` — severity taxonomy for
  `INVALID_TAG`, `RELEASE_MISSING`, `GIT_RANGE_FAILED`,
  `RELEASE_EDIT_FAILED`.
- `skills/definitions/step-skip-rule.md` — articulation-as-enforcement.
  The "no migration docs added" branch in step 3 is a natural
  no-op, not a skip violation.

## Dependencies

This skill makes no calls to other skills. It uses `git` and the
`gh` CLI directly. No `apply-label`, `set-issue-status`,
`post-issue-comment`, etc.

## Steps

The **step-skip rule** applies. The migration-detection branch in
step 3 is naturally no-op when no migration docs were added; not
a skip.

**Inputs.** The skill receives two inputs from the caller (a
release workflow):

- `tag` (string, required) — the version tag for the release being
  published, e.g. `v1.2.3`.
- `repo` (string, optional) — the repo to operate on. If omitted,
  resolve via `gh repo view --json nameWithOwner -q .nameWithOwner`.

Hold as `<tag>` and `<repo>`.

---

### Section A — Range and commits

1. **Validate the tag exists.** Confirm the tag is real:

   ```bash
   git tag -l "$tag"
   ```

   - Empty result → raise `INVALID_TAG` (`ERROR`); exit. The caller
     gave a tag that doesn't exist locally.
   - Otherwise → continue.

2. **Determine the previous tag.** List tags by version, exclude
   the current one, take the head:

   ```bash
   PREV_TAG=$(git tag --sort=-version:refname | grep -v "^${tag}\$" | head -1)
   ```

   - `PREV_TAG` empty → this is the first release; the range is
     the full history. Note that in step 4's summary line.
   - `PREV_TAG` set → the range is `${PREV_TAG}..${tag}`.

3. **Detect added migration docs.** Scan the release range for
   newly-added files matching `concepts/migration-*.md` or
   `docs/migration-*.md` (case-insensitive on `migration` to catch
   `MIGRATION-*.md`):

   ```bash
   if [ -n "${PREV_TAG}" ]; then
     git diff --name-status --diff-filter=A "${PREV_TAG}..${tag}" -- \
       'concepts/migration-*.md' 'docs/migration-*.md' \
       'docs/MIGRATION-*.md'
   else
     git log --name-status --diff-filter=A --pretty=format: -- \
       'concepts/migration-*.md' 'docs/migration-*.md' \
       'docs/MIGRATION-*.md' | sort -u
   fi
   ```

   `--diff-filter=A` restricts to ADDED files, so edits to an
   existing migration doc do not re-emit the callout in every
   subsequent release. Hold the result as `<added-migrations>`.

   For each entry, read its first-line `# …` heading as the title,
   for use in the callout in step 5.

4. **Collect commits in the range.** Subject and short hash only;
   the body (and any `Reuse:` trailer) is not consumed:

   ```bash
   if [ -n "${PREV_TAG}" ]; then
     git log --pretty=format:"%s (%h)" "${PREV_TAG}..${tag}"
   else
     git log --pretty=format:"%s (%h)" "${tag}"
   fi
   ```

   On non-zero exit → raise `GIT_RANGE_FAILED` (`ERROR`); exit.

   Hold the list as `<commits>`.

---

### Section B — Categorise and write

5. **Categorise each commit.** Conventional-commits prefix decides
   the section. Both `type:` and `type(scope):` forms are accepted.

   | Prefix | Section |
   |---|---|
   | `feat:` / `feat(...):` | Features |
   | `fix:` / `fix(...):` | Fixes |
   | `docs:` / `docs(...):` | Documentation |
   | `chore:` / `chore(...):`, `ci:`, `refactor:`, `test:`, `style:`, `perf:`, `build:` | Chores (only if notable) |
   | `Merge pull request ...` | Omit |
   | Auto-bump / sync / no-prefix junk commits | Omit |

   **Specific automated-commit patterns to omit** (release notes
   are for human-visible changes; these are tooling noise):

   - `Merge pull request #...` and `Merge branch ...`
   - `chore: update TEMPLATE_VERSION ...`
   - `chore: update AGENTIC_FRAMEWORK_VERSION ...`
   - `chore: sync ...` (any sync commit from the framework mount
     or a downstream mirror operation)
   - `chore: bump ...` (version-bump-only commits)
   - `chore: archive recovery log for #...` (the dev-session's
     end-of-session bookkeeping commit; not human-visible)
   - `chore: recovery checkpoint — ...` (the dev-session's
     mid-task breadcrumb commits; not human-visible)
   - Commits with no conventional-commits prefix and no clear
     human content (e.g. `WIP`, `fixup`, `temp`)

   When in doubt, omit. A release-note bullet for tooling noise
   adds nothing for the human reader.

   For each retained commit, write a one-sentence, present-tense
   bullet that answers "what changed and why it matters" — NOT
   "what was the commit message". Examples:

   - Commit: `feat: add foreground-recovery skill (#42)` →
     Bullet: `Adds the foreground-recovery skill so humans can
     interactively diagnose and clear stuck pipeline state.`
   - Commit: `fix: drop scheduled stage; rename to ready-to-implement` →
     Bullet: `Renames the Requirement-lifecycle stage from
     "scheduled" to "ready-to-implement" across the Go CLI, the
     project template, and the workflow YAML.`

   The agent paraphrases for the human; it does NOT echo the
   commit subject verbatim. Read changed files for ambiguous
   commits if needed.

   Hold per-section bullet lists as `<features>`, `<fixes>`,
   `<docs>`, `<chores>`.

6. **Decide if this is a no-notable-changes release.** If every
   `<commits>` entry was filtered (only merges / auto-bumps /
   junk), and `<added-migrations>` is empty → Output B path:

   - Compose a one-line body:
     ```
     Internal release — no human-visible changes since <PREV_TAG>.
     ```
   - Skip to step 8.

7. **Compose the notes file.** Build `/tmp/release-notes.md` in
   the canonical shape:

   ```markdown
   <One-sentence summary of what this release delivers overall.>

   ## ⚠️ Required Action          ← only when <added-migrations> is non-empty
   - <One bullet per added migration doc, linking it at the release tag>

   ## Features
   - <bullet>
   - ...

   ## Fixes
   - <bullet>

   ## Documentation
   - <bullet>

   ## Chores
   - <bullet>
   ```

   **Rules:**
   - The one-sentence summary at the top is mandatory; it is the
     human's TL;DR.
   - Omit any section whose bullet list is empty.
   - The migration-callout placement is fixed: immediately after
     the summary, before `## Features`. Never fold a migration
     link into `## Documentation` — it is a required-action
     signal, not a doc improvement.
   - The release tag and a title MUST NOT appear in the body —
     GitHub renders those separately.
   - Each migration bullet links at the release tag, e.g.
     `https://github.com/<repo>/blob/<tag>/<path>`.
   - Each bullet is one sentence, present tense: "Adds...",
     "Renames...", "Removes...", "Fixes...", "Documents...".

   Write the composed body to `/tmp/release-notes.md`. Use the
   `Write` tool — never shell `echo`/heredoc, since the content
   may contain backticks, dollar signs, etc.

---

### Section C — Publish

8. **Confirm the release exists.** Before editing:

   ```bash
   gh release view "$tag" --repo "$repo" --json url --jq .url
   ```

   - Non-zero exit / "release not found" → Output C. Surface a
     clear diagnosis ("Release `<tag>` does not exist on
     `<repo>`. The publish workflow must create the release first;
     this skill only edits existing ones.") and exit cleanly with
     a `RELEASE_MISSING` (`WARN`).
   - Otherwise → continue. Hold the URL as `<release-url>`.

9. **Update the release body.**

   ```bash
   gh release edit "$tag" --repo "$repo" \
     --notes-file /tmp/release-notes.md
   ```

   Verifies the edit by re-querying the release body:

   ```bash
   gh release view "$tag" --repo "$repo" --json body --jq .body
   ```

   The returned body must equal what was written. On mismatch or
   non-zero exit → raise `RELEASE_EDIT_FAILED` (`ERROR`); the
   notes file is on disk for the human to apply manually.

10. **Print the summary line.** One-line stdout for the workflow
    log:

    ```
    Release notes published: <release-url>
      Features: <N>  Fixes: <N>  Docs: <N>  Chores: <N>  Migrations: <N>
    ```

    For Output B (no notable changes):
    ```
    Release notes published (internal release): <release-url>
    ```

    For Output C (no release):
    ```
    Release notes NOT published — release <tag> does not exist on
    <repo>. The notes file at /tmp/release-notes.md was generated;
    apply manually with `gh release edit <tag> --notes-file ...`
    once the release is created.
    ```

## Error Handling

- `INVALID_TAG` from step 1 (the tag does not exist locally) →
  severity `ERROR`; propagate. Caller bug — they passed a tag
  that wasn't created.
- `GIT_RANGE_FAILED` from step 4 (`git log` non-zero) → severity
  `ERROR`; propagate. Surface the underlying error; the human
  investigates.
- `RELEASE_MISSING` from step 8 (no GitHub release for the tag) →
  severity `WARN`. Output C. Notes file remains on disk for manual
  application; this is recoverable.
- `RELEASE_EDIT_FAILED` from step 9 (`gh release edit` failed or
  verification mismatch) → severity `ERROR`. The local notes file
  is preserved; surface the manual-apply command in the exit
  message so the human can finish.
- All other errors: propagate (default).
