---
name: build-catalogue
description: Regenerates CATALOGUE.md from every skill's YAML frontmatter in a deterministic, diff-friendly order (grouped by category, alphabetical within category). Use when CATALOGUE.md is missing or stale (any skill mtime newer than the catalogue), or after a skill has been added, removed, or had its frontmatter edited.
category: Operation
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# Build Catalogue

## Purpose

Rebuild `CATALOGUE.md` at the repository root from the YAML frontmatter of every
skill file in `skills/`. The catalogue is the index `session-init` reads instead
of eagerly loading every skill body — regenerating it cheaply and deterministically
is the core of the lazy-loading design.

## When to Run

- Invoked by `session-init` when the catalogue is missing or stale (any
  `skills/*.md` file has a modification time newer than `CATALOGUE.md`).
- Invoked manually by an author after editing skill frontmatter.
- **Not** a session-ending skill. It runs to completion, writes the file, and
  returns control silently.

## Procedure

Execute these steps in order. The procedure is deterministic: the same set of
frontmatter values must produce a byte-identical `CATALOGUE.md`.

### Step 1 — Enumerate skill files

List every `*.md` file in `skills/` at the repo root. Exclude:

- `CATALOGUE.md` itself (it lives at the repo root, not in `skills/` — listed
  here only to make the exclusion explicit)
- The skill `build-catalogue.md` is **included** (it is still a skill; its own
  frontmatter is rendered in the catalogue).

The enumeration list is the authoritative set for this run. Do not scan
subdirectories — the framework places all skills directly under `skills/`.

### Step 2 — Parse frontmatter

For each enumerated file:

1. Read the file.
2. Confirm it begins with a `---\n` fence. If not, **fail loudly** (see
   "Failure mode" below).
3. Locate the closing `\n---\n` fence and parse the content between the fences
   as YAML.
4. Validate the required fields per `skills/skill-categories.md`:
   `name`, `description`, `category`, `triggers`, `loads`, `emits-exit-block`,
   `exit-hands-to`. Enforce the category/exit-block/exit-hands-to consistency
   rules.
5. Record the tuple `(category, name, description, triggers)` for rendering.

If any file fails parsing or validation, abort and report per "Failure mode".
Silently skipping an invalid skill is forbidden.

### Step 3 — Group by category

Group the parsed tuples by `category` using this **fixed order**:

1. Session
2. Recovery
3. Bootstrap
4. Operation
5. Information
6. Reference

This order mirrors the taxonomy order in `skills/skill-categories.md` and must
not change. Renderers, diff tools, and session-init rely on it.

### Step 4 — Sort alphabetically within category

Within each category, sort the skills alphabetically by `name` using byte-wise
(ASCII) ordering. `name` values are already constrained to `[a-z0-9-]`, so no
locale-dependent collation is needed. Byte-wise sort is the deterministic
choice.

### Step 5 — Render the catalogue

Emit `CATALOGUE.md` at the repo root using the exact layout below. The first
two lines (title + "Do not edit by hand" banner) are fixed. Category headings
are always emitted in the fixed order, even when a category has no skills (in
which case the heading is followed by a single-line "_(no skills)_" placeholder
so the heading is never orphaned). This keeps the catalogue structure stable
across refactors and makes a category appearing or disappearing visible in
diffs.

**Exact layout:**

```markdown
# Skill Catalogue

Generated from skill frontmatter by skills/build-catalogue.md. Do not edit by hand.

## Session skills

- **<name>** — <description>
  Triggers: <triggers>

- **<name>** — <description>
  Triggers: <triggers>

## Recovery skills

- **<name>** — <description>
  Triggers: <triggers>

## Bootstrap skills

- **<name>** — <description>
  Triggers: <triggers>

## Operation skills

- **<name>** — <description>
  Triggers: <triggers>

## Information skills

- **<name>** — <description>
  Triggers: <triggers>

## Reference skills

- **<name>** — <description>
  Triggers: <triggers>
```

Rendering rules:

- **Per skill:** one bullet block with two lines — first line `- **<name>** — <description>`, second line `  Triggers: <triggers>`. An empty line separates bullet blocks within a category.
- **`triggers` rendering:** if the frontmatter value is a string, render it verbatim. If it is a list, render as a comma-separated list in the YAML order (e.g. `session-start, post-sync`). Quoted values (e.g. `"automation: in-design"`) render without surrounding quotes.
- **Empty category:** emit the heading, then a single line `_(no skills)_`, then a blank line. Never omit the heading.
- **Do not emit** body content, `loads`, `emits-exit-block`, or `exit-hands-to`. The catalogue is an index, not a frontmatter dump.
- **File ends with a single trailing newline.** No trailing whitespace on any line.

### Step 6 — Write atomically

Write `CATALOGUE.md` to a temp file and rename it into place so interrupted
runs do not leave a half-written catalogue. On any modern POSIX filesystem,
`os.Rename` (Go) or `shutil.move` (Python) on the same filesystem is atomic.

## Determinism guarantee

Given the same set of skill frontmatter values, this procedure must produce a
**byte-identical** `CATALOGUE.md` on every run. This is how `session-init`'s
mtime-based staleness check yields clean diffs: a no-op regeneration changes
only the file's mtime, not its content. Contributors checking `git diff
CATALOGUE.md` after a regeneration should see no changes unless a skill's
frontmatter actually changed.

Sources of non-determinism to avoid:

- Do not embed the current date, commit SHA, or runner info in the file.
- Do not emit list-style `triggers` in a different order than the frontmatter
  declares.
- Do not use locale-dependent collation for the within-category sort.

## Failure mode

If any skill fails to parse or violates the schema:

1. **Stop immediately** — do not emit a partial catalogue, do not silently skip
   the offending skill.
2. Print an actionable message naming the file and the specific violation, in
   the same format `gh agentic check` uses for frontmatter errors (see
   `skills/skill-categories.md`):

   ```
   ✗ skills/<file>.md
     frontmatter: <specific violation>
     expected: <expected shape or enum>
     see: skills/skill-categories.md for the schema
   ```

3. Instruct the user to run `gh agentic check` to see the full list of
   violations, and to fix the offending file before re-running the catalogue
   build.
4. Exit non-zero. If invoked by `session-init`, the session halts with the
   failure message — it is better to stop than to proceed with a broken index.

## Rules

- **This skill is deterministic.** Any non-determinism is a defect.
- **Do not edit `CATALOGUE.md` by hand.** Change frontmatter, re-run this skill.
- **The six categories and their order are fixed.** Any change is a framework
  update and must land through `skills/skill-categories.md` first.
- **Empty categories emit their heading.** The catalogue's structure is part of
  its contract with session-init and with human readers.
- **No body content in the catalogue.** `loads`, `emits-exit-block`,
  `exit-hands-to` are intentionally omitted — they are consumed by validators
  and by session-init via direct frontmatter reads, not via the catalogue.
- **Atomic write.** Never leave a half-written catalogue on disk.
