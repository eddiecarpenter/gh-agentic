# cp-execution-context.md — Control-plane execution context for pipeline phases

This document defines the runtime context every **execution phase**
(feature-design, dev-session, compliance-verify, pr-review-session)
operates in under the control-plane-centralized model (#870, #873).

It is the authoritative contract referenced by those skills. It makes
concrete the read-only doc-consumer rule (carried from #855) and the
"feature issues live on the control plane" keystone (#872) for the
agent at execution time.

---

## The two anchors

The pipeline runs **on the control plane**. Each execution phase checks
the control plane out as the workspace root (the knowledge layer) and
clones the feature's **target repo** into `./project` (the code). Two
environment variables anchor the layout — both are exported by the
pipeline before the recipe runs:

| Anchor | Points at | Holds |
|---|---|---|
| `AGENTIC_CP_ROOT` | the workspace root | the framework mount (`.agents`), all project docs, `AGENTS.md` / `RULEBOOK.md` / `LOCALRULES.md`, `FEDERATION.md` — and is the repo where **issues live** |
| `AGENTIC_PROJECT_DIR` | `./project` | the **code** — the target repo at the feature/PR branch. This is the agent's **current working directory** |

In a single-topology project the target *is* the control plane, so both
anchors resolve to the same repo checked out twice — the rules below
still hold unchanged.

**Legacy fallback.** When neither anchor is set (a pre-#873 single-repo
checkout), the working directory is both the code and the knowledge
root; every "`$AGENTIC_CP_ROOT/...`" path below falls back to the
current directory, and "the control plane repo" falls back to the
current repo. A skill MUST check for the anchor and fall back rather
than assume it is set.

---

## Where each operation goes

The agent's cwd is the **code** (`./project`), but the knowledge and the
issues live on the **control plane**. Route every operation accordingly:

| Operation | Target | How |
|---|---|---|
| Read a doc / brief / architecture | Control plane | read under `$AGENTIC_CP_ROOT/docs` (see *Reading docs*) |
| Read the rulebook / skills / standards | Control plane | `$AGENTIC_CP_ROOT/.agents` (the recipe loads `$AGENTIC_CP_ROOT/AGENTS.md`) |
| Feature / Requirement / Task issue read or write (view, label, comment, sub-issue, status) | Control plane | `gh ... --repo <CP>` where `<CP>` is the `owner/repo` of `$AGENTIC_CP_ROOT` |
| Code edits, branches, commits, `git push` | Project | operate in cwd (`./project` / `$AGENTIC_PROJECT_DIR`) |
| Pull-request create / read / comment / review | Project | `gh pr ... --repo <target>` (the PR lives where the code lives) |

Resolve the two `owner/repo` strings once, at the top of the phase:

```bash
CP_REPO=$(git -C "${AGENTIC_CP_ROOT:-.}" remote get-url origin \
  | sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##')
TARGET_REPO=$(git -C "${AGENTIC_PROJECT_DIR:-.}" remote get-url origin \
  | sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##')
```

Then pass `--repo "${CP_REPO}"` to every `gh issue` operation on the
Feature, and `--repo "${TARGET_REPO}"` to every `gh pr` operation.

> A cross-repo "Closes" reference (`Closes <CP>#<N>`) is required when a
> PR in the target repo implements a Feature issue on the control plane;
> GitHub's same-repo auto-close does not fire across repos. The
> `feature-complete` pipeline stage closes the Feature explicitly, so the
> reference is for human traceability, not automation.

---

## Reading docs — two tiers, control-plane-homed

All project documentation lives on the control plane (#871). Resolve doc
reads against `$AGENTIC_CP_ROOT/docs`, never against the project code
(which carries no docs):

- **System tier** — `$AGENTIC_CP_ROOT/docs/SYSTEM_BRIEF.md` and
  `$AGENTIC_CP_ROOT/docs/SYSTEM_ARCHITECTURE.md` (federation only).
- **Domain tier** — `$AGENTIC_CP_ROOT/docs/domains/<domain>/BRIEF.md`
  and `.../ARCHITECTURE.md`, where `<domain>` is the target repo's
  domain from `$AGENTIC_CP_ROOT/FEDERATION.md`.
- **Single topology** — `$AGENTIC_CP_ROOT/docs/BRIEF.md` and
  `.../ARCHITECTURE.md` (the unqualified pair; the repo *is* the project).

See `concepts/knowledge-plane.md` for the full naming model.

---

## Read-only on the control plane

The control-plane checkout is a **read-only knowledge root**, pinned at
`main`. An execution phase **MUST NOT write to it**:

- No commits to `$AGENTIC_CP_ROOT`. No `docs/new/` write-back to the
  control plane — the per-feature architecture-integration write-back is
  **not** performed by the pipeline. Architecture updates to the control
  plane are separate human-driven PRs (`concepts/knowledge-plane.md`).
- All file writes the phase makes — code, tests, design products under
  `docs/design/feature-<N>/` — land in `$AGENTIC_PROJECT_DIR` (the
  project repo), never in the control-plane root.

The only durable effects an execution phase has on the control plane are
**GitHub issue operations** (labels, comments, status, sub-issues) via
`gh ... --repo <CP>` — never filesystem writes.

---

## Headless doc-insufficiency → halt and signal

When an execution phase runs **headless** (no human in the loop) and
finds the control-plane documentation insufficient to proceed correctly
— missing architecture for the area being changed, an unscoped
dependency, a contradiction it cannot resolve — it **halts** rather than
guessing:

1. Make **no** code commit and **no** control-plane commit.
2. Apply the kickback label on the Feature issue (`--repo <CP>`):
   `needs-scoping` when the gap is a scoping/requirements gap;
   `needs-human-review` when the work cannot continue without a human
   decision. Remove the current in-flight phase label.
3. Post a comment on the Feature naming the gap precisely — which
   document, which fact was missing, what decision is needed.
4. Exit. A red/halted phase is the visible signal; a silent guess is the
   failure mode this rule exists to prevent.

In an **interactive** phase the agent surfaces the same gap to the human
and waits, rather than applying a kickback label.
