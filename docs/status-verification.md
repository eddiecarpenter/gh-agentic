# `gh agentic status` — Manual Verification Checklist

This document lists the scenarios that are worth confirming manually against a
live `gh agentic` project before cutting a release. The automated tests in
`internal/cli/status_integration_test.go` cover every documented behaviour
against in-memory fixtures; this checklist exists so a human can spot-check
that the live integration (GraphQL queries, terminal rendering, colour
support) behaves the same way.

Run each command and compare to the expected shape. Anything that diverges
should either be logged as a bug or added as a regression test before
release.

## Setup

1. Point a shell at a gh-agentic control-plane repo that has an
   `AGENTIC_PROJECT_ID` variable set.
2. Confirm `gh auth status` reports a logged-in user with access to the
   project board.
3. Install the `gh-agentic` extension built from this branch.

## List commands

- [ ] `gh agentic status requirements` — compact table (REQUIREMENT / STAGE /
      TITLE), totals line, no REPO column when all items are local.
- [ ] `gh agentic status requirements --include-done` — adds closed items,
      totals increment.
- [ ] `gh agentic status requirements --json` — valid JSON envelope
      `{items, totals}`; parseable via `| jq`.
- [ ] `gh agentic status requirements --this-repo` — identical to default in a
      single-repo topology; narrowed in a federated one.
- [ ] `gh agentic status features` — as above, but features.
- [ ] `gh agentic status features --this-repo` in a federated project — cross
      repo entries drop out.

## Detail commands

- [ ] `gh agentic status requirement <N>` for an open requirement — body
      first, structured fields after, linked features block.
- [ ] `gh agentic status requirement 9999` → `Error: issue #9999 not found in
      <repo>` with non-zero exit.
- [ ] `gh agentic status requirement <feature-number>` → "#N is a feature,
      not a requirement" with the suggested command.
- [ ] `gh agentic status feature <N>` for the current feature — parent
      requirement, branch one-liner, PR one-liner, task checklist.
- [ ] `gh agentic status feature <N> --json` — valid JSON, `parent_requirement`
      / `branch` / `pr` are `null` when absent; `tasks` is always an array.

## Pipeline

- [ ] `gh agentic status pipeline --requirements` — vertical stage-grouped
      view, heading, section counts, `(none)` for empty columns.
- [ ] `gh agentic status pipeline --features --include-done` — adds `done`
      column to the right.
- [ ] `gh agentic status pipeline --features --horizontal` on a wide terminal
      (≥ 120 cols) — side-by-side columns, unicode box-drawing on UTF-8
      locales.
- [ ] `gh agentic status pipeline --features --horizontal` on a narrow terminal
      (< 120 cols) — clean error naming required vs current widths.
- [ ] `LANG=C gh agentic status feature <N>` — task checklist glyphs switch
      to `[x]` / `[ ]` ASCII fallback.
- [ ] `gh agentic status pipeline --features --json` — JSON wins; no pipeline
      headings in output.
- [ ] `gh agentic status requirements --kanban` (legacy flag) — migration
      error pointing at `gh agentic status pipeline --requirements`.
- [ ] `gh agentic status pipeline` with a card title containing `→` on a
      252-col terminal — every row has the same rune count and every `│`
      separator sits at the same visual column across top border, content
      rows, and bottom border.

## Error paths

- [ ] In a repo without `AGENTIC_PROJECT_ID`: any status sub-command →
      `Error: AGENTIC_PROJECT_ID is not set for this repository.` block with
      fix instructions.
- [ ] Revoke the project's permission on your token / set the variable to a
      bogus node ID: any status sub-command → "Error: agentic project ... is
      not reachable" with `gh auth status` hint.
- [ ] Pull the network cable (or simulate): error text distinguishes network
      / auth / rate-limit / 5xx via `projectstatus.ClassifyAPIError`.
- [ ] `gh agentic status` with no sub-command → help output listing four
      leaf sub-commands, exit 0, does not hang on stdin.

## Blocked-by

- [ ] A requirement or feature with a GitHub native dependency (tracked
      issues) renders `[blocked by <repo>#N]` inline in list views.
- [ ] A requirement or feature with a `Blocked-by: owner/repo#N` line in its
      body — but no native tracking — also renders the annotation.
- [ ] An item with neither renders no annotation.

## gh agentic check

- [ ] `gh agentic check` surfaces a "Project reachability" group on every
      topology. Pass shows the project title. Fail (missing variable) shows
      the `gh agentic project join` remediation. Fail (unreachable) suggests
      `gh auth status`.

## Post-verification

If any item above surfaces unexpected behaviour, capture the exact command
and output, open a bug issue, and add an automated test covering the
regression before shipping.
