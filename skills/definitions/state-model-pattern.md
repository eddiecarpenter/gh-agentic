# State Model Pattern

Skills that perform a sequence of GitHub-side mutations whose
recoverability differs use this pattern to make the cancellation
and failure semantics explicit. The agent must always know which
state it is in so it can choose the correct cancel or failure
behaviour.

A skill that loads this definition supplies its own concrete
**transition table** (the per-skill list of T0/T1/T2/T3/...
mutations and their recoverability) and refers to the universal
**cancel rules** and **failure-during-transition rules** described
below.

## The transition table

Each consuming skill provides a table of this shape, near the top
of its `## Steps` section:

```markdown
| Transition | Where | Effect | Skill-recoverable? |
|---|---|---|---|
| **T0 → T1** | step <N> | <one-line description of what mutates>     | <Yes / Partial / No — with rationale> |
| **T1 → T2** | step <N> | <one-line description>                      | <Yes / Partial / No — with rationale> |
| ...        | ...      | ...                                          | ...                                     |
```

Conventions:

- **T0** is the pre-entry state (no mutations yet); the skill is at
  T0 before any work runs.
- Number the transitions in the order they fire during a normal
  run. Skip numbers if a transition is conditional.
- The **Where** column points at the specific step that fires the
  transition.
- The **Skill-recoverable?** column has three permitted values:
  - **Yes** — the skill can cleanly revert this transition on
    cancel/failure.
  - **Partial** — the skill can revert some artefacts but not
    others; the partial state must be surfaced.
  - **No — point of no return** — once this fires, the skill
    cannot revert. The human picks up.

## Cancel rules by state (universal)

Every consuming skill applies these rules unless it explicitly
overrides one in a "Skill-specific cancel override" subsection.

- **Before T1** (no mutations yet) → clean exit. No revert is
  needed because no GitHub state has changed.
- **At T_k** where T_k is **Yes**-recoverable → revert the
  T_k mutation (and any later mutations, in reverse order) and
  exit. Surface to the human which mutations were reverted.
- **At T_k** where T_k is **Partial**-recoverable → revert what
  the skill can; surface the residual partial state explicitly
  (which artefacts remain, what state they are in) so the human
  can decide whether to complete manually or run a repair flow.
- **At T_k** where T_k is **No — point of no return** → the
  skill MUST NOT attempt to revert. Surface the partial state
  in the exit block; recommend manual cleanup or completion.
  Do NOT attempt a "best-effort" rollback that would leave
  orphans pointing at reverted parents.

The "No — point of no return" tier MUST be surfaced as a warning
to the human at the confirmation gate immediately before it fires:

> ⚠ Once you confirm, <effect of T_k>. Cancellation after this
> point cannot <revert action> automatically; the session would
> exit with <residual state> in place.

## Failure-during-transition rules (universal)

When a transition fails partway through (e.g., the label apply
succeeds but the status set fails), the resulting state is a
mismatch — the skill MUST raise the appropriate error code (per
its Error Handling section), exit, and rely on the next session's
pre-entry check to detect and surface the inconsistency.

- **T0 → T1 fails partway** → raise the skill's transition error.
  Surface which side of the mutation succeeded and which failed.
  Recommend `gh agentic repair`.
- **T_k → T_{k+1} partial** where multiple artefacts are created
  (e.g., creating N Feature issues) → raise the issue-creation
  error. Do NOT continue past the partial set; partial work on a
  partial set tends to make recovery harder, not easier. Surface
  which artefacts succeeded and which failed; recommend keeping
  the successful ones (they are valid pipeline state) and either
  re-running the skill (which the orphan-re-entry check will
  detect) or closing the orphans manually.
- **A late transition fails** (the final label/status flip in a
  closeout) → raise the transition error. The earlier artefacts
  are in their intended states; the skill is stuck on the final
  flip. Recommend `gh agentic repair` and a manual re-run of the
  final transition only.

## Orphan re-entry detection

When a skill is re-invoked on an entity that is mid-transition (a
prior session was interrupted between T_k and T_{k+1}), the skill's
pre-entry check (typically the first step that reads the entity's
state) MUST detect the partial state and branch:

- **Pre-T2 leftover** (only T1 fired, no irreversible artefacts
  yet) → offer the human "continue from current state" or "revert
  to T0".
- **T2-or-later leftover** (irreversible artefacts already exist)
  → the skill cannot cleanly resume (would duplicate work) and
  cannot cleanly revert (would orphan the artefacts). Surface
  the existing artefacts and recommend manual completion or
  cleanup. Exit cleanly without changes.

## Where this pattern applies

Any skill whose `## Steps` section performs more than one
GitHub-side mutation in sequence and where any of those mutations
is non-trivially recoverable. Pure-read skills do not need this
pattern; single-mutation skills (e.g., `apply-label`,
`set-issue-status`) do not need it either — there is nothing to
revert mid-flow.

Consumer skills today: `requirement-scoping`, `feature-design`.
Future consumers: any skill that creates issues, posts comments,
and transitions labels in sequence (PR-merge cascade workflow,
multi-Feature parallel scoping, etc.).
