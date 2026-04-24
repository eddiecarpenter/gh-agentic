---
name: gh-agentic-tool
description: Authoritative command reference for the gh agentic CLI extension — every command in the cobra tree with every declared flag, the --raw output contract for agent-oriented data retrieval, and a decision matrix for common agent questions. Use whenever the agent needs to interact with the agentic framework from the command line.
category: Reference
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# gh-agentic Tool

> **This skill is verified against the CLI by `TestGhAgenticToolSkillCoversCLI` —
> if it is out of sync the build fails.** Every command path and every declared
> flag listed below is covered by that test. When you change the CLI, update
> this skill in the same PR.

## Purpose

Reference skill for the `gh agentic` CLI extension. Documents every command,
every declared flag, the agent-oriented `--raw` output contract, and a
decision matrix for the questions agents most often ask the framework.

A loaded copy of this skill is the agent's single source of truth — there
should be no need to invoke `--help` on any command.

## When to Invoke

- Whenever a `gh agentic` command is needed
- Whenever an agent needs to retrieve project state programmatically
- When diagnosing or repairing project / pipeline health

---

## Global

- `gh agentic --version` — print the extension version (cobra built-in)
- `gh agentic --help` (`-h`) — print top-level help (cobra built-in; agents
  should rely on this skill instead)

The auto-generated `completion` sub-command is disabled — gh extensions
invoked via `gh agentic` do not support tab completion.

---

## Command Reference

Every command's fully-qualified path and every declared flag is listed.
Where a flag has a short alias, the long form is shown first followed by
`(-x)`.

### `gh agentic init`

First-time setup wizard. Resolves topology (single vs. federated), creates
or joins the project, mounts the framework, and configures pipeline
infrastructure (variables, secrets, wrapper workflows).

Between the framework mount and the pipeline-configuration step, the
wizard checks whether the agentic GitHub App is installed on the target.
If installed, it logs a skip message and continues. If missing and the
session is interactive, it prompts the user to open the install page in a
browser. If missing and the session is headless (CI / non-TTY), it prints
the install URL and continues without blocking. `--skip-app-install`
bypasses the check entirely.

Flags:
- `--force` — overwrite existing configuration on a repo that is already
  initialised.
- `--skip-app-install` — bypass the agentic GitHub App install-state check
  and install guidance.

### `gh agentic check`

Run the full health check covering both project membership and pipeline
readiness. Prints a Project section then a Pipeline section, with pass /
warn / fail counts at the end.

Flags: none.

### `gh agentic repair`

Auto-fix issues reported by `check`. Repairs include framework not
mounted, missing project board views, topology variable writes (CP-side
`AGENTIC_FRAMEWORK_VERSION`, clearing stray values on domain repos),
`.ai/` missing from `.gitignore`, and workflow version-tag drift.
Variable / secret values that need human input are surfaced via huh
prompts.

Topology is always deduced by `project.Resolve` — from the project
owner, linked-graph, and local `AGENTIC_FRAMEWORK_VERSION`. There is no
override flag and no interactive topology prompt; each repo's repair
fixes only its own state. When a federated-domain repo detects that the
control plane has missing state, repair terminates with a pointed
instruction to run `gh agentic repair` on the control plane repo.

Flags: none.

### `gh agentic mount`

Sync the AI-Native Delivery Framework at `.ai/` to the correct version.
Single topology reads `.ai-version`; federated domain repos read
`AGENTIC_FRAMEWORK_VERSION` from the control plane and update the local
`.ai-version` to match. Federated domain repos additionally get a
read-only `.cp/` mount of the control plane's `docs/` tree.

Flags:
- `--yes` (`-y`) — skip the confirmation prompt when the version is
  changing (for scripts).

**Version resolution:** `project.Resolve` is the single canonical source. On
a single-topology or federated-CP repo, the pinned version comes from
`AGENTIC_FRAMEWORK_VERSION` on the repo itself (falling back to the clone's
`.ai/.git` metadata, and then to the latest release). On a federated domain
repo, the pinned version is read from `AGENTIC_FRAMEWORK_VERSION` on the
control-plane repo so every domain stays in lock-step with the CP.

### `gh agentic upgrade [version]`

Change the framework version for the whole agentic project. Valid only on
the control plane. Specifying an older version downgrades the federation.

Flags:
- `--yes` (`-y`) — skip the confirmation prompt.
- `--list` (`-l`) — list available framework versions and exit.

### `gh agentic info`

Print a complete overview of the current environment: extension version
and installation date, repo / project / topology, and three framework
versions (local, remote / control-plane authoritative, latest available
release) with sync indicators.

Flags: none.

### `gh agentic auth`

Group command for managing Claude Code credentials on domain repos.
Bare invocation prints help. Auth commands are blocked on the federated
control plane (which does not run agents).

Flags: none on the group itself.

### `gh agentic auth login`

Force a new Claude Code login and upload refreshed credentials to the
`CLAUDE_CREDENTIALS_JSON` repo secret.

Flags: none.

### `gh agentic auth refresh`

Upload current local credentials to the repo secret without triggering a
new login.

Flags: none.

### `gh agentic auth check`

Verify that local credentials are valid and the repo secret is set, then
report whether they are in sync.

Flags: none.

### `gh agentic project`

Group command for project-membership lifecycle. Bare invocation prints
help.

Flags: none on the group itself.

### `gh agentic project create [title]`

Create a new GitHub Project board and establish this repo as the
federated control plane.

Flags:
- `--version` — framework version to mount (default: latest).
- `--interactive` (`-i`) — collect title and version via a form.

### `gh agentic project join [project-name]`

Bring this repo into an existing project as a domain repo.

Before the join variable is written, the command checks whether the
agentic GitHub App is installed on the target. Organisation owners check
at the org level (one install covers every repo under the org); personal-
account owners check at the repo level. If missing and the session is
interactive, the user is prompted to open the install page. If missing
and the session is headless (CI / non-TTY), the install URL is printed
and the command continues. `--skip-app-install` bypasses the check.

Flags:
- `--list` (`-l`) — list available projects and exit.
- `--interactive` (`-i`) — select project interactively.
- `--skip-app-install` — bypass the agentic GitHub App install-state
  check and install guidance.

### `gh agentic project switch [project-name]`

Move this repo to a different agentic project. Requires the repo to
already be initialised.

Flags:
- `--list` (`-l`) — list available projects and exit.
- `--interactive` (`-i`) — select project interactively.

### `gh agentic project unlink`

Remove this repo's project affiliation. The GitHub Project board itself
is not deleted; the framework mount at `.ai/` is left in place.

Flags:
- `--yes` (`-y`) — skip the confirmation prompt.

### `gh agentic status`

Group command for pipeline state. Bare invocation prints help. All
sub-commands accept `--raw` for an agent-oriented payload.

Flags: none on the group itself.

### `gh agentic status requirements`

List open requirements with stage. Default output is a compact table.

Flags:
- `--raw` — emit agent-oriented raw output (tab-separated for lists,
  frontmatter + markdown for details) and suppress human output.
- `--verbose` — include timestamps in `--raw` output (no-op without
  `--raw`).
- `--this-repo` — narrow the view to the current repository only.
- `--include-done` — include items in the `done` stage.

### `gh agentic status requirement <number>`

Show detail for a single requirement: number, title, stage, dates,
body, linked features, and blocked annotation.

Flags:
- `--raw` — emit frontmatter header, `---` separator, and verbatim
  markdown body; suppress human output.
- `--verbose` — include timestamps in `--raw` output (no-op without
  `--raw`).

### `gh agentic status features`

List open features with stage. Same shape as `requirements` plus a TASKS
column showing `done/total`.

Flags:
- `--raw` — emit agent-oriented raw output (tab-separated for lists,
  frontmatter + markdown for details) and suppress human output.
- `--verbose` — include timestamps in `--raw` output (no-op without
  `--raw`).
- `--this-repo` — narrow the view to the current repository only.
- `--include-done` — include items in the `done` stage.

### `gh agentic status feature <number>`

Show detail for a single feature: number, title, stage, dates, body,
parent requirement, tasks, branch state, PR state.

Flags:
- `--raw` — emit frontmatter header, `---` separator, and verbatim
  markdown body; suppress human output.
- `--verbose` — include timestamps in `--raw` output (no-op without
  `--raw`).

### `gh agentic status pipeline`

Render requirements and features together as a side-by-side pipeline
view — the first-class way to answer "where are we?" at a glance. Bare
invocation renders both pipelines stacked.

Flags:
- `--requirements` — render only the requirements pipeline (mutually
  exclusive with `--features`).
- `--features` — render only the features pipeline (mutually exclusive
  with `--requirements`).
- `--horizontal` — force horizontal layout regardless of terminal width
  (no-op under `--raw`).
- `--vertical` — force vertical layout regardless of terminal width
  (no-op under `--raw`).
- `--include-done` — include items in the `done` stage.
- `--this-repo` — narrow the view to the current repository only.
- `--raw` — emit agent-oriented raw output (tab-separated sections per
  selector); `--horizontal` / `--vertical` are no-ops under `--raw`.
- `--verbose` — include timestamps in `--raw` output (no-op without
  `--raw`).

---

## `--raw` Contract

`--raw` is the **only** output mode an agent should use to read project
state. Its shape is stable, byte-equal across runs for a fixed input,
and minimises token cost.

### When to use it

- Every programmatic data retrieval the agent performs.
- Whenever the agent needs to filter, sort, or pick a single field out
  of the response.

### List shape (`status requirements`, `status features`)

Tab-separated. Header row first, one data row per item. Sparse cells
render as `-`. No presentation glyphs, no colours, no borders, no
totals line.

```
number	stage	title	blocked_by	owning_repo
447	backlog	feat: project lifecycle management	-	eddiecarpenter/gh-agentic
457	scoping	feat: single-pane pipeline status view	-	eddiecarpenter/gh-agentic
467	backlog	feat: skill-publishing	-	eddiecarpenter/gh-agentic
```

Both list commands emit the same column set — agents that parse one
parse both with the same code path.

### Single-item shape (`status requirement <N>`, `status feature <N>`)

Frontmatter-style header, the literal line `---` as separator, then the
verbatim issue body. Empty values render as `key:` (no trailing space).
The `---` separator is always emitted, even for an empty body — agents
can split on it without checking for body presence.

Requirement header keys:
`number`, `stage`, `title`, `owning_repo`, `blocked_by`, `linked_features`
(space-separated feature numbers, no `#` prefix; empty when none).

Feature header keys:
`number`, `stage`, `title`, `owning_repo`, `blocked_by`,
`parent_requirement`, `branch`, `pr`, `tasks_done_total` (e.g. `3/6`).

Example:

```
number: 569
stage: scheduled
title: Centralised project context resolution
owning_repo: eddiecarpenter/gh-agentic
blocked_by:
linked_features: 571 572
---
## Business need

Today, `gh agentic init` reads the project context from many places. We need
a single chokepoint.
```

The body is preserved byte-for-byte — markdown headings, fenced code
blocks, and unicode characters (e.g. `→`) all survive without escaping.

### Pipeline shape (`status pipeline`)

Bare invocation emits two TSV sections separated by a single blank line,
each prefixed with a section marker line:

```
# requirements
number	stage	title	blocked_by	owning_repo
<requirement rows>

# features
number	stage	title	blocked_by	owning_repo
<feature rows>
```

`pipeline --requirements --raw` emits only the `# requirements` section
(no `# features` marker). `pipeline --features --raw` is the symmetric
case. Per-section row shape is identical to the list commands —
guaranteed by the same renderer.

### `--verbose` (timestamps)

By default `--raw` omits `created_at` and `last_transitioned_at` to keep
the agent token cost low. Pass `--verbose` to opt in:

- **List shape:** appends `created_at` and `last_transitioned_at`
  columns to the right of the header and to every data row (ISO-8601
  date, `YYYY-MM-DD`). Sparse timestamps render as `-`.
- **Detail shape:** inserts `created_at` and `last_transitioned_at`
  header lines after `owning_repo`. Empty values render as `key:`.

Column / line count grows by exactly 2 in both shapes.

`--verbose` without `--raw` is a documented no-op — human output is
unchanged.

### Agent rules

- **Prefer `--raw` over parsing human text output.** The default human
  table has glyphs, colours, and totals lines that are not part of the
  contract; only `--raw` is stable.
- **Never render `--raw` verbatim back to the human.** The human asked a
  question — answer it in prose, citing the relevant rows. Emitting a
  TSV blob defeats the point of having an agent in the loop.

---

## Common Agent Questions

Decision matrix with the exact recipes to run. Filter / sort logic
should happen on the agent side after a single `--raw` fetch.

### "What's triggerable right now?"

```
gh agentic status features --raw
```

Filter rows where `stage == backlog`. Cross-check the parent
requirement's stage with:

```
gh agentic status requirements --raw
```

A feature is triggerable when its own stage is `backlog` **and** its
parent requirement is `scheduled` or beyond. Read the requirement's
parent via `gh agentic status feature <N> --raw` and the
`parent_requirement:` line.

### "What's blocked?"

```
gh agentic status pipeline --raw
```

Filter rows in both sections where `blocked_by != -`. The `blocked_by`
cell is the `owner/repo#N` reference of the blocking issue.

### "Summarise this requirement / feature"

```
gh agentic status requirement <N> --raw
```

or

```
gh agentic status feature <N> --raw
```

Read the header for stage / owning_repo / linked features. Summarise
the verbatim markdown below the `---` separator. Do not paraphrase the
header fields — quote them.

### "What stage is feature #N?"

```
gh agentic status feature <N> --raw
```

Read the `stage:` line. Do not run the human-mode command and parse
the table — the table layout is not part of the contract.

### "Is the project healthy? What's broken?"

```
gh agentic check
```

This is human output — there is no `--raw` for `check` (yet). Read the
Project / Pipeline sections, follow the `→ <remediation>` hints, and
decide whether to run `gh agentic repair`.

### "What framework version are we on?"

```
gh agentic info
```

Read the Framework section. The three lines are local, remote (control
plane authoritative), and latest available — with sync indicators.

### "Is the agentic GitHub App installed on this repo / org?"

There is no dedicated subcommand for this yet — the check is performed
inline as part of `gh agentic init` and `gh agentic project join`. Read
the command output for one of these three lines:

- `GitHub App already installed on <target> — skipping install step` —
  the App is present and matches the expected slug. No action required.
- `Install the agentic GitHub App at https://github.com/apps/... before
  running the pipeline.` — missing, headless session (CI / non-TTY). The
  command continued without blocking; the human or the operator must
  click the URL before the pipeline runs.
- `Install the agentic GitHub App at https://github.com/apps/... when
  ready.` — missing, interactive session where the user declined the
  prompt. Same remediation.

To bypass the step entirely (useful for CI smoke tests or when the
install state is known-good out-of-band), pass `--skip-app-install`.

---

## Rules for the Agent

- **Never run `mount <version>` on a federated domain repo** — version is
  governed by the control plane. Run `mount` with no argument to sync.
- **Never run `auth` commands on the federated control plane** — they are
  blocked by design (the control plane does not run Claude agents).
- **`repair` is safe to run automatically** — it is non-destructive and
  idempotent. Run it whenever `check` reports failures.
- **Always re-run `check` after `repair`** — confirm the repair succeeded
  before proceeding.
- **Do not run `project join` on an uninitialised repo** — run
  `gh agentic init` first.
- **Use `--raw` for every programmatic read.** Never parse the human
  table.
- **Use `--raw --verbose` only when you actually need timestamps.**
  Default `--raw` is the cheaper token shape; pay for `--verbose` only
  when the additional fields will be used.
- **The `kanban` flag on list commands has been removed.** If you see it
  referenced in older code or docs, update to `gh agentic status pipeline
  --requirements` or `--features`.
- **The `--json` flag has been removed end-to-end.** Cobra now responds
  `unknown flag: --json` on every status command. Use `--raw` instead.
- **App install check runs inline in `init` and `project join`.** Neither
  command blocks on the install flow — installed, declined, and headless
  all let the command continue. Scrape the output lines above (or the
  install URL prefix `https://github.com/apps/`) to detect the branch
  that executed. Use `--skip-app-install` when running in automation
  that has out-of-band confirmation the App is present.
