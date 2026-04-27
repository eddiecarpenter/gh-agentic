# Concurrency Beacon

A label on a GitHub issue that marks "a session is currently
working on this entity". Skills that perform multi-step work on a
single entity use a beacon to prevent two sessions from racing —
e.g., a workflow re-firing while a prior run is still active, or
a human invoking interactively while automation is running.

The pattern is: **claim** the beacon on entry, **release** it on
exit, **best-effort release** on every error path.

A skill that loads this definition supplies its own concrete
beacon label name (`design-in-progress`, `development-in-progress`,
`issue-in-progress`, etc.) and the specific steps where claim and
release fire.

## Claim semantics

The first step that mutates state for the entity (typically very
early in the skill) MUST claim the beacon:

```
apply-label(repo=<active-repo>, issue=<N>,
            add=["<beacon-name>"], remove=[])
```

Before the apply, probe whether the beacon is already set:

```bash
gh issue view <N> --repo <active-repo> --json labels \
  --jq '[.labels[].name] | index("<beacon-name>")'
```

Branch on the result:

- **Beacon already set** in headless mode → another session is in
  flight (or recently was). Exit cleanly with a one-line note:
  "Another session is active; this run is a no-op." Do NOT remove
  the beacon — it belongs to the other session. Do NOT proceed.
- **Beacon already set** in interactive mode → another session
  *may* be in flight (the prior session may also have crashed
  leaving the beacon stuck). Surface a warning to the human and
  prompt:
  ```
  ⚠ <beacon-name> is set on this entity. Another session may be
     actively working on it. Continuing will likely cause conflicts.
  ```
  Then offer "Continue anyway" / "Cancel". If the human picks
  Continue, fall through to the apply step (the apply is a no-op
  if the beacon is already set; the slot is now claimed by *this*
  session for release purposes).
- **Beacon not set** → apply it. On apply failure, raise the
  skill's invalid-state error and exit before any further work.
  The beacon is the lock; without it, single-writer semantics are
  not guaranteed.

From the moment the beacon is claimed, every exit path (success,
parked, error, cancel) MUST attempt to release it.

## Release semantics

The skill's normal exit path removes the beacon as the first
action of its closeout step:

```
apply-label(repo=<active-repo>, issue=<N>,
            add=[], remove=["<beacon-name>"])
```

A failure here is surfaced as a `WARN` and does not block exit —
the substantive work is complete; a stuck beacon is a presentation
issue the human can clear by hand or via `foreground-recovery`.

## Slot-release rule (universal — applies to every error path)

Every error path AND every cancel path that fires AFTER the beacon
was claimed MUST attempt to remove the beacon before exit, on a
best-effort basis. If the removal itself fails, surface it as a
`WARN` and exit anyway — the original error is what matters; a
stuck beacon is a secondary concern.

In the consuming skill's `## Error Handling` section, surface the
rule explicitly so future maintainers see the obligation:

```markdown
**Slot-release rule (universal).** Every error path AND every
graceful exit AFTER step <N> (the slot was claimed) MUST attempt
to remove `<beacon-name>` before exit, on a best-effort basis.
If the removal itself fails, surface it as a `WARN` and exit
anyway — the original error is what matters.
```

## Why a beacon and not just relying on workflow concurrency

GitHub Actions has its own `concurrency:` group mechanism, but it
does not cover:

- A workflow run + an interactive human session against the same
  entity.
- Two runs against the same entity from different triggers (label
  apply vs comment vs PR review).
- Recovery: a beacon that *should* be set isn't (so a re-fire is
  the wrong choice), or a beacon is *stuck* set after a crash (so
  a re-fire is the right choice but the workflow won't fire it).

The label-based beacon is observable and inspectable from any
session, lives on the same entity as the rest of the state, and
can be cleared manually. It is the defensive complement to
workflow-level concurrency, not a replacement.

## Where this pattern applies

Any skill that:

- Performs multi-step work tied to a single GitHub entity
  (typically a Feature or Requirement).
- Has at least one mutation that, if duplicated by a concurrent
  run, would produce conflicting or duplicated artefacts.
- Can be invoked from more than one trigger (workflow + human,
  multiple workflow events).

Consumer skills today: `dev-session` (`development-in-progress`),
`feature-design` (`design-in-progress`), `issue-session`
(`issue-in-progress`), `foreground-recovery` (operates on stuck
beacons rather than claiming one — special case).

## Naming convention

Beacon labels use the pattern `<phase>-in-progress`:

- `design-in-progress` — `feature-design` is active.
- `development-in-progress` — `dev-session` is active.
- `issue-in-progress` — `issue-session` is active.

Future skills introducing a new phase MUST use this pattern. The
canonical list of beacon labels lives in
`internal/project/assets/project-template.json` so `gh agentic
init` provisions them on new projects.
