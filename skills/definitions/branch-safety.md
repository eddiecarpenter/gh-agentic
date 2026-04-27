# Branch Safety

The "refuse-on-main" guard. Skills that mutate the local working
tree (write files, run `git add`, delete branches, etc.) MUST refuse
to operate when the active branch is `main` (or `master`) so an
accidental commit cannot land directly on the integration branch.

This rule is unconditional and matches RULEBOOK.md's "Never commit
or make changes on `main`" rule. It is enforced at the skill's
entry, before any state mutation.

## The check

Run as the first action after the skill's entry banner, before any
file write, git operation, or GitHub mutation:

```bash
git branch --show-current
```

Branch on the result:

- **Empty / detached HEAD** — surface a `WARN` and exit cleanly.
  The skill cannot determine the branch; refusing is the safe
  default.
- **`main` or `master`** — raise the skill's branch-safety error
  code (typically `ON_MAIN_BRANCH`) with severity `ERROR` and exit
  cleanly. Render the remediation template below to the human.
- **Anything else** — capture the branch name as `<branch>` and
  continue.

## Remediation template

When the check fails, render to the human:

```
This skill refuses to run on main. Switch to a branch first
(e.g. `git checkout -b <suggested-prefix>-<short-context>`), then
re-invoke this skill.
```

`<suggested-prefix>` is skill-specific — examples in the live
framework:

- `solution-architecture` → `chore/architecture-update`
- `foreground-recovery` → `chore/recovery-<timestamp>`

The skill MAY also recommend a different branching base (e.g.,
the human's current feature branch) when context makes it
appropriate — the prefix above is the default fallback.

## Where this guard applies

Any skill that:

- Writes files in the working tree (e.g., creating or editing
  `docs/ARCHITECTURE.md`, scaffolding source files).
- Runs `git add`, `git commit`, or any other history-mutating
  command in the working tree.
- Deletes local branches, removes worktree files, or otherwise
  modifies the local repo state.
- Performs interactive recovery operations whose remediation
  could touch local state.

It does NOT apply to:

- Pure-read skills (`gh agentic status` queries, repo inspection).
- Skills that operate entirely against GitHub via `gh` (creating
  issues, posting comments) without touching the local working
  tree.
- Skills that *create* a feature branch as their first action
  (e.g., `dev-session` checks out `feature/<N>-<slug>` before any
  local edits — it never edits files while on `main`).

Consumer skills today: `solution-architecture`, `foreground-recovery`.
Future consumers: any new skill that opens an interactive session
involving local file edits.

## Why this is a separate guard

The check is mechanically trivial (one git command, one branch),
but the failure mode it prevents is severe: a stray commit on
`main` that bypasses the framework's branch/PR/review pipeline.
Centralising the rule and its remediation template in one
definition means a future change (e.g., adding `develop` to the
refuse list, or extending the error message) lands once rather
than per skill.
