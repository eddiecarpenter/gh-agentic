---
name: gh-agentic
description: Picks the right gh agentic CLI command (with the right flags, especially --raw) to answer framework, project, requirement, feature, or pipeline questions — and runs it. The cheapest token-cost path to read framework state. Use when the agent needs to retrieve project / pipeline / requirement / feature state to answer a human question or to inform another skill's decision. Use even when the user doesn't say "gh agentic" — phrases like "what version are we on", "what's the framework state", "list open requirements", "what features are in flight", "show me feature N", "what's blocked", "is the pipeline healthy", "what's broken" should trigger this skill.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
emits-exit-block: false
---

# gh-agentic

> **CI-verified against the CLI surface by `TestGhAgenticToolSkillCoversCLI`.**
> Every command path and every declared flag listed below is covered by that
> test. When the CLI changes (new/removed/renamed command or flag), update
> this skill in the same PR. The build fails if it drifts.

## Why this skill matters for token cost

**`gh agentic` with `--raw` is the cheapest path — by an order of
magnitude — to read framework, requirement, feature, and pipeline
state.** Compared to the alternatives:

| Path | Typical token cost | Why |
|---|---|---|
| `gh agentic status features --raw` | ~10 tokens per item | TSV, header + 1 row per item, no body |
| `gh issue list --label feature --json number,title,state,body` | ~200–2000 tokens per item | Full issue bodies, JSON envelopes, label arrays, timestamps |
| `gh api graphql ...` (manual queries) | varies, usually higher | The agent has to author the query, GraphQL responses include nested envelopes |
| Reading the GitHub UI via `WebFetch` | very high | HTML noise, navigation chrome, render the same data with 50× the tokens |

The `--raw` shape is minimal by design: pre-filtered to what an agent
actually needs, no presentation glyphs, no totals lines, no colour
codes. Use it as the **default** for any framework-state query.
Reach for raw `gh issue list` only when you need a field that
`--raw` doesn't expose (rare).

## Goal

Invoke the `gh agentic` CLI correctly for any framework query or
action — picking the right command, the right flags (especially the
agent-only `--raw` for programmatic reads), and parsing the response
into a structured value the caller can use.

## Output Artefacts

When invoked **interactively** (human typed `/gh-agentic`):

- A clean, human-readable rendering of the chosen command's output.
  No raw TSV blobs surfaced verbatim.

When invoked **programmatically** (another skill needs framework
state):

- A structured return value: parsed `--raw` output as either a list
  of records (for `status requirements`/`features`/`pipeline`) or a
  `{ header: dict, body: string }` shape (for single-item details).

When invoked as a **reference** (loaded via another skill's `loads:`,
not actively run):

- No artefact — the consumer reads this file's body to learn the
  correct command + flags, then runs the bash directly.

## Definitions

- `skills/definitions/error-handling.md` — the severity taxonomy for
  the CLI failure modes detected in step 4 (`PIPELINE_UNHEALTHY`,
  `CLI_NOT_INSTALLED`, etc.).

## Dependencies

None as a skill. This skill shells out to the `gh agentic` CLI and
parses its output; it does not invoke any other framework skill.

## Steps

1. **Identify the query.** Either:

   - The caller passed an explicit query (another skill invoking
     this with a specific intent, e.g. "list open features",
     "framework version", "is the pipeline healthy"), or
   - The agent inferred a query from the human's natural-language
     question (e.g. *"what version are we on?"* → framework version;
     *"what's blocked?"* → pipeline-blocked filter; *"summarise
     feature 654"* → feature detail). The Common Agent Recipes
     section below maps recognised intents to commands.

2. **Pick the command and flags** to answer the query. Use the
   Command Reference and `--raw` Contract below. Hard rules:

   - For any **programmatic read** (caller will parse the result),
     ALWAYS pass `--raw`. Default human output has glyphs and
     totals lines that are not part of the contract.
   - For any **human-facing display** (the user will read it),
     OMIT `--raw`. The default human output is the polished form;
     a TSV blob is unhelpful to a human.
   - Pass `--this-repo` only when the caller explicitly wants to
     scope to the current repo.
   - Pass `--include-done` only when the caller wants closed items.
   - Pass `--verbose` (with `--raw`) only when timestamps are
     actually needed downstream — it doubles the payload.

3. **Execute the chosen command.** Capture stdout, stderr, exit
   code.

   **Detect:**
   - Exit code 127 / `command not found` → raise
     `CLI_NOT_INSTALLED` (`ERROR`). The `gh-agentic` extension is
     not installed in this environment.
   - Exit code non-zero, stderr contains "not initialised" or
     similar → raise `REPO_NOT_INITIALISED` (`ERROR`). Caller
     should suggest `gh agentic init`.
   - Exit code non-zero on `gh agentic check` → not an error of
     this skill; the check ran and reported failures. Surface them
     to the caller as the result, not as an exception.
   - Other non-zero exit → raise `CLI_FAILED` (`ERROR`) with the
     stderr in the detail.

4. **Parse the response.** For `--raw` output:

   - **List shape** (`status requirements`/`features` and pipeline
     sections) → split on `\n`, header row first, then data rows;
     emit a list of dicts keyed by the header columns.
   - **Detail shape** (single requirement/feature) → split on the
     literal `\n---\n` separator. Parse the lines before `---` as
     `key: value` pairs (allow empty values: `key:` is `""`); the
     part after is the verbatim markdown body.
   - **Pipeline shape** → split on the `# requirements` and
     `# features` section markers. Each section parses as the list
     shape above.

   For non-`--raw` output (when the human will read the result
   directly), no parsing — pass it through verbatim.

5. **Return the answer.** Two return paths:

   - When the caller is another skill (programmatic): return the
     parsed structured value.
   - When the agent is answering a human question: render a
     concise prose answer that cites the relevant fields (e.g.
     *"Framework version is v2.4.1, latest is v2.4.2 — one
     release behind."* not a TSV blob). The human asked a
     question; answer it. Do not paste the raw command output as
     the response.

---

## Command Reference

Every CLI command, fully-qualified, with declared flags. CI keeps
this in sync with the cobra tree.

### Top-level

- `gh agentic --version` — print extension version (cobra built-in).
- `gh agentic --help` (`-h`) — print top-level help (cobra built-in;
  agents should rely on this skill, not on `--help`).

The auto-generated `completion` sub-command is disabled.

### `gh agentic init`

First-time setup wizard. Resolves topology, creates or joins the
project, mounts the framework, configures pipeline infrastructure.

Inline app-install check between mount and pipeline config: if the
agentic GitHub App is missing, prompts (interactive) or prints the
install URL (headless) and continues.

Flags:
- `--force` — overwrite existing configuration.
- `--skip-app-install` — bypass the App install-state check.

### `gh agentic check`

Run the full health check (project + pipeline). Prints sections with
pass / warn / fail counts. Human output only — no `--raw` (yet).

Flags: none.

### `gh agentic repair`

Auto-fix issues reported by `check`. Non-destructive, idempotent.
Topology is always deduced by `project.Resolve` — no override flag.

Flags: none.

### `gh agentic mount`

Sync the framework at `.agents/` to the correct version. Single topology
reads `.ai-version`; federated domain reads
`AGENTIC_FRAMEWORK_VERSION` from the control plane.

Flags:
- `--yes` (`-y`) — skip the confirmation prompt.

### `gh agentic upgrade [version]`

Change the framework version for the whole project. Control-plane
only.

Flags:
- `--yes` (`-y`) — skip the confirmation prompt.
- `--list` (`-l`) — list available versions and exit.

### `gh agentic info`

Print a complete overview: extension version, repo / project /
topology, framework versions (local, remote/CP authoritative, latest
release) with sync indicators.

Flags: none.

### `gh agentic auth` (group)

Manages Claude Code credentials on domain repos. Blocked on the
federated control plane.

Subcommands:
- `gh agentic auth login` — force a new login + upload to the repo
  secret.
- `gh agentic auth refresh` — upload current local credentials to
  the repo secret.
- `gh agentic auth check` — verify local + secret are in sync.

Flags: none.

### `gh agentic project` (group)

Project-membership lifecycle.

Subcommands and flags:

- `gh agentic project create [title]`
  - `--version` — framework version to mount (default: latest).
  - `--interactive` (`-i`) — collect title + version via form.
- `gh agentic project join <owner/repo>` — CP-side: register a domain repo with
  this control plane (writes `FEDERATION.md`, links the Project, sets the target
  repo's `AGENTIC_PROJECT_ID`; **no framework mount** — domain repos are pure code).
  - `--domain` — domain the repo belongs to (required; lazy-created if new).
  - `--purpose` — the repo's purpose within its domain.
  - `--domain-purpose` — purpose of the domain (used when creating a new domain).
- `gh agentic project switch [project-name]`
  - `--list` (`-l`)
  - `--interactive` (`-i`)
- `gh agentic project unlink`
  - `--yes` (`-y`)
- `gh agentic project control-plane` — print the control-plane repo (the linked repo bearing FEDERATION.md) for this repo's project.
  - `--raw` — print only `owner/repo` on stdout; prints nothing and exits 0 for single topology (no separate control plane). Used by the CP-rooted checkout composite action to discover the control plane.

### `gh agentic status` (group)

Pipeline state queries. **All sub-commands accept `--raw`** for
agent-oriented data retrieval.

Subcommands and flags:

- `gh agentic status requirements`
  - `--raw` — agent-oriented TSV output (see `--raw` Contract below).
  - `--verbose` — include timestamps in `--raw` (no-op without).
  - `--this-repo` — narrow to current repo.
  - `--include-done` — include `done`-stage items.
- `gh agentic status requirement <number>`
  - `--raw`, `--verbose`
- `gh agentic status features`
  - same flag set as `status requirements`, plus a TASKS column
    showing `done/total`.
- `gh agentic status feature <number>`
  - `--raw`, `--verbose`
- `gh agentic status pipeline` — side-by-side requirements +
  features.
  - `--requirements` — only the requirements pipeline (mutually
    exclusive with `--features`).
  - `--features` — only the features pipeline.
  - `--horizontal` — force horizontal layout (no-op under `--raw`).
  - `--vertical` — force vertical layout (no-op under `--raw`).
  - `--include-done`, `--this-repo`, `--raw`, `--verbose`.

---

## `--raw` Contract

`--raw` is the **only** output mode an agent should use to read
project state. Stable, byte-equal across runs, minimises tokens.

### List shape (`status requirements`, `status features`)

Tab-separated. Header row first, one data row per item. Sparse cells
render as `-`. No glyphs, no colours, no totals.

```
number	stage	title	blocked_by	owning_repo
447	backlog	feat: project lifecycle management	-	eddiecarpenter/gh-agentic
```

Both list commands emit the same column set — parse once, use for
both.

### Single-item shape (`status requirement <N>`, `status feature <N>`)

Frontmatter-style header, literal `---` separator, verbatim body.
Empty values render as `key:` (no trailing space). Separator is
always emitted, even for empty body.

Requirement keys: `number`, `stage`, `title`, `owning_repo`,
`blocked_by`, `linked_features` (space-separated, no `#`).

Feature keys: `number`, `stage`, `title`, `owning_repo`,
`blocked_by`, `parent_requirement`, `branch`, `pr`,
`tasks_done_total` (e.g. `3/6`).

Body is preserved byte-for-byte — markdown headings, fenced code,
unicode all survive.

### Pipeline shape (`status pipeline`)

Two TSV sections separated by a blank line, each prefixed with a
section marker:

```
# requirements
number	stage	title	blocked_by	owning_repo
<rows>

# features
number	stage	title	blocked_by	owning_repo
<rows>
```

`pipeline --requirements --raw` emits only the `# requirements`
section. `pipeline --features --raw` is symmetric.

### `--verbose` (timestamps)

Default `--raw` omits `created_at` and `last_transitioned_at`. Pass
`--verbose` to opt in:

- **List shape:** appends two columns to the right (ISO date,
  `YYYY-MM-DD`). Sparse render as `-`.
- **Detail shape:** inserts two header lines after `owning_repo`.

`--verbose` without `--raw` is a documented no-op.

---

## Common Agent Recipes

Decision matrix for the queries agents most often have. Filter / sort
on the agent side after a single `--raw` fetch.

### "What's triggerable right now?"

```
gh agentic status features --raw
```

Filter rows where `stage == backlog`. Cross-check parent requirement
stage with `gh agentic status requirements --raw`. A feature is
triggerable when `stage == backlog` AND parent requirement is
`ready-to-implement` or beyond.

### "What's blocked?"

```
gh agentic status pipeline --raw
```

Filter both sections where `blocked_by != -`. The cell is the
`owner/repo#N` reference.

### "Summarise this requirement / feature"

```
gh agentic status requirement <N> --raw
gh agentic status feature <N> --raw
```

Read the header for stage / owning_repo / linked features. Summarise
the verbatim body below `---`. Quote header values; do not
paraphrase.

### "What stage is feature #N?"

```
gh agentic status feature <N> --raw
```

Read the `stage:` line. Never parse the human table.

### "Is the project healthy? What's broken?"

```
gh agentic check
```

Human output (no `--raw` yet). Read sections, follow `→ <remediation>`
hints, decide whether to run `gh agentic repair`.

### "What framework version are we on?"

```
gh agentic info
```

Read the Framework section: local, remote (CP authoritative), latest
release, with sync indicators.

### "Is the agentic GitHub App installed?"

No dedicated subcommand yet. The check is inline in `gh agentic init`
and `gh agentic project join`. Scrape stdout for one of:
- `GitHub App already installed on <target> — skipping install step`
- `Install the agentic GitHub App at https://github.com/apps/...
  before running the pipeline.` (headless / declined)

Bypass via `--skip-app-install`.

---

## Error Handling

- `CLI_NOT_INSTALLED` from step 4 (exit 127 / command not found) →
  severity `ERROR`; propagate. The `gh-agentic` gh extension is not
  installed in this environment.
- `REPO_NOT_INITIALISED` from step 4 (CLI ran but the repo isn't
  set up) → severity `ERROR`; propagate. The caller should suggest
  `gh agentic init`.
- `CLI_FAILED` from step 4 (exit non-zero, not one of the specific
  cases above) → severity `ERROR`; propagate with stderr in detail.
- `gh agentic check` returning a non-zero exit because checks
  failed → NOT an error of this skill; surface the result to the
  caller as data. The caller decides whether to invoke `repair` or
  escalate.
- All other errors: propagate (default).

## Rules for the Agent

- **Never run `mount <version>` on a federated domain repo** —
  version is governed by the control plane. Run `mount` with no
  argument to sync.
- **Never run `auth` commands on the federated control plane** —
  blocked by design (the CP doesn't run agents).
- **`repair` is safe to run automatically** — non-destructive and
  idempotent. Run it whenever `check` reports failures.
- **Always re-run `check` after `repair`** — confirm the repair
  succeeded.
- **Run `project join` on the control plane** — it registers a named domain
  repo (`<owner/repo> --domain`) into the federation; it does not affect the
  current repo or mount the framework.
- **Use `--raw` for every programmatic read.** Never parse the
  human table.
- **Use `--raw --verbose` only when timestamps are actually needed.**
  Default `--raw` is the cheaper token shape.
- **The `kanban` flag was removed.** Use `gh agentic status pipeline
  --requirements` or `--features`.
- **The `--json` flag was removed end-to-end.** Use `--raw`.
- **App install check is inline in `init`.** It does not block on the
  install flow. Scrape stdout for the install URL prefix
  `https://github.com/apps/` or use `--skip-app-install` when the install
  state is known-good out-of-band.
