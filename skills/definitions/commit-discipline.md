# commit-discipline.md — Universal commit discipline

Any agent-driven, code-touching skill that produces a commit on a
feature branch follows this discipline. It is loaded as a definition
by `dev-session` (per-task), `pr-review-session` (per-review-batch),
and any future skill that lands code on a Feature branch.

This document is the single source of truth. Skills do not duplicate
the rules — they reference this file by path in their `loads:` block
and inherit the discipline.

The discipline has five non-negotiable parts.

## 1. Reuse outcome (mandatory)

Before writing any new function, type, module, or schema, the agent
must record one of three outcomes for the change:

- `reuse — as-is` — an existing symbol in the codebase already covers
  the need. No new code; reference the existing symbol.
- `reuse — refactor — <one-line>` — an existing symbol nearly covers
  the need; extend or generalise it. The one-line note explains what
  was extended and how.
- `reuse — opt-out — <reason>` — genuinely new code; existing code is
  unsuitable for the recorded reason. The reason must be specific
  ("the existing handler is sync; this needs to stream"), not vague
  ("nothing existed").

*"I didn't look"* is never a permitted outcome. The agent MUST inspect
the existing codebase before writing new code.

The outcome is recorded as a `Reuse:` trailer on the commit (see §4).

## 2. Tests are first-class

- **Existing tests must pass** after the changes. Tests must be
  *executed* — writing tests without running them does not count.
- **New code requires new tests** covering at minimum a success case,
  a failure case, and an edge case (boundary, concurrency, unusual
  input).
- **Modified functionality requires updated tests.** When a behaviour
  changes, its tests change with it — not after.
- **No commit lands with failing tests.** If tests fail, fix them
  before committing. There is no retry cap; keep working until they
  pass. If genuinely stuck (no forward progress on consecutive
  attempts, environment-shaped error, ambiguity in the work), the
  calling skill raises its own `*_BLOCKED` or `TEST_FAILED_PERSISTENT`
  error and exits — it does not commit failing tests.

## 3. Build must pass

The project's build must pass cleanly before the commit. Same rule as
tests: fix-before-commit, no cap on attempts, raise and exit if stuck.

## 4. Commit format

Conventional-commits prefix; descriptive subject; optional body;
mandatory `Reuse:` trailer.

```
<type>: <descriptive subject> [— <context tag>]

[optional body — bullet points, rationale, links]

Reuse: <outcome from §1>
```

`<type>` is `feat`, `fix`, `refactor`, `docs`, `test`, `chore` per
conventional-commits.

`<context tag>` is the calling skill's responsibility. Examples:

- `dev-session` per-task: `— task <K> of <M> (#<feature>)`
- `pr-review-session` per-batch: `— PR #<N>`

The `Reuse:` trailer is the LAST line of the commit message.

After committing, verify the commit shape:

```bash
git log -1 --format='%s%n%b'
```

The subject must match the format; the trailer must start with
`Reuse:`. If either is wrong, amend the commit before proceeding.
This is the one place `git commit --amend` is permitted — the commit
just landed, no published history.

## 5. Push after every commit

Push immediately after each commit. No batching across commits.

```bash
git push origin "$BRANCH"
```

Verify:
```bash
git log -1 origin/$BRANCH --format='%H'
```
matches the local HEAD.

On push failure (network, permissions, non-fast-forward), the calling
skill raises its own error code and exits. The local commit is preserved;
a re-run can re-push.

The "push after every commit" rule applies regardless of whether the
calling skill produces one commit per invocation or many. The unit is
"a commit landed locally" → "push it before doing anything else."

## What this discipline does NOT cover

- Branch creation, checkout, or rebasing — owned by the calling skill.
- Issue closing or label transitions — owned by the calling skill.
- The semantics of *which* commits to make and in *what order* — owned
  by the calling skill.

This file is the contract for the commit itself. Everything around it
(when, why, on which branch, with what effect on issue state) is the
calling skill's playbook.
