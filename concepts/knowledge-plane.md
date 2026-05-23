# concept: knowledge plane

This document defines the **knowledge plane** — where project knowledge lives
and how it reaches the AI at the right moment in every phase.

The knowledge plane is the counterpart to the **control plane**. Where the
control plane orchestrates the pipeline (issues, labels, project board, runners),
the knowledge plane describes the *content* the pipeline operates over:
briefs, architecture, and how those surface into every session.

Read `project-command.md` for the control plane model. This document is the
companion for knowledge.

---

## Guiding principle

**Each repo is self-contained as far as its own knowledge goes.** A repo's
`docs/` fully describes its responsibility. The control plane describes
only what spans repos — the seams, the cross-cutting concerns, the
system-level orientation. The rule is:

> The CP describes what spans repos. A repo describes what it owns.
> Never describe the same fact twice.

---

## Topology and layout

The knowledge plane takes two shapes, matching the two topologies.

### Single topology (embedded control plane)

The repo is its own control plane. All knowledge is local.

```
repo/
├── .agents/                              # framework mount, read-only
├── docs/
│   ├── BRIEF.md                      # why this exists
│   ├── ARCHITECTURE.md               # how it works
│   └── new/                          # staging for architectural contributions
└── (code)
```

In single topology `docs/BRIEF.md` and `docs/ARCHITECTURE.md` carry both
system-level and repo-level scope — the repo *is* the project.

### Federated topology

Each domain repo owns its own knowledge. The control plane owns only
system-level knowledge.

**Domain repo:**
```
domain-repo/
├── .agents/                              # framework mount, read-only, gitignored
├── .cp/                              # control plane knowledge mount, read-only, gitignored
│   ├── SYSTEM_BRIEF.md
│   └── SYSTEM_ARCHITECTURE.md
├── docs/
│   ├── BRIEF.md                      # this domain's scope
│   ├── ARCHITECTURE.md               # this domain's internals
│   └── new/                          # staging for architectural contributions
└── (code)
```

**Control plane repo:**
```
control-plane-repo/
├── .agents/
└── docs/
    ├── SYSTEM_BRIEF.md               # system orientation
    └── SYSTEM_ARCHITECTURE.md        # seams between repos
```

The control plane holds no application code. Its job is orchestration and
system-level knowledge.

---

## Naming — scope-explicit

Different names at different scopes make the rule unambiguous:

> **Domain knowledge = `SYSTEM_BRIEF` + `SYSTEM_ARCHITECTURE` + `BRIEF` + `ARCHITECTURE`**

A skill in a domain repo loads four files in federated mode. An AI loading
`.cp/SYSTEM_BRIEF.md` knows immediately it is reading system-level context; an
AI loading `docs/BRIEF.md` knows it is reading repo-scoped context. In single
topology only the two unqualified files exist.

### What each file holds

| File | Answers | Scope |
|---|---|---|
| `SYSTEM_BRIEF.md` | Why the system as a whole exists | System |
| `SYSTEM_ARCHITECTURE.md` | The seams between repos, cross-cutting concerns | System |
| `BRIEF.md` | Why this repo exists within the system | Repo |
| `ARCHITECTURE.md` | How this repo works internally | Repo |

A system-level brief or architecture should *never* describe how a single
repo works internally. A repo-level brief or architecture should *never*
describe seams owned by another repo.

---

## Membership

In federated topology, **every member repo is linked to the GitHub Project**.
A repo-level variable marks the control plane:

```
AGENTIC_CONTROL_PLANE=true
```

Set on exactly one repo per federation (the CP). All other members have
`AGENTIC_PROJECT_ID` set but not `AGENTIC_CONTROL_PLANE`.

Discovery:
1. Query the Project's linked repositories — returns all N members
2. Filter for the repo whose `AGENTIC_CONTROL_PLANE` variable is `true`
3. The rest are domain repos

The GitHub Project is the single source of truth for membership. A deleted
repo is automatically absent from the query; a newly-added repo is automatically
present once it joins the Project.

---

## Update mechanisms

The knowledge plane contains four distinct kinds of artefact. Each has a
different change frequency, and each uses the mechanism that matches.

| Artefact | Frequency | Mechanism | Who drives |
|---|---|---|---|
| `docs/BRIEF.md` | Rare — strategic pivots | Direct PR | Human |
| `docs/ARCHITECTURE.md` | Frequent — per feature | `docs/new/` + merge-triggered AI integration | Dev session writes; workflow integrates |
| CP `docs/SYSTEM_BRIEF.md` | Rare | Direct PR on CP | Human |
| CP `docs/SYSTEM_ARCHITECTURE.md` | Rare | Direct PR on CP | Human |

The AI integration machinery exists **exactly where frequency justifies it**:
per-repo `ARCHITECTURE.md` receiving contributions from many features in
parallel. Everywhere else, rarity makes a direct human-driven PR honest and
sufficient.

### Cross-domain architectural changes

When a domain feature implies an update to `SYSTEM_ARCHITECTURE.md`, that
update is a **separate human-driven PR on the CP repo**. The PR description
references the triggering feature by cross-repo reference (e.g.
`triggered by owner/repo#N`).

There is no cross-repo automation for CP knowledge updates. Cross-domain
architectural changes are rare and important — they deserve human attention
and a dedicated review, not a side-effect of a domain feature merging.

---

## The `docs/new/` pattern

The merge-conflict problem that drove requirements and features to become
GitHub Issues would return if every feature branch edited `ARCHITECTURE.md`
directly. The `docs/new/` pattern keeps the file-based artefact while avoiding
the parallel-branch collision.

### On a feature branch

When a feature contributes architectural knowledge, the dev session writes
a single file:

```
docs/new/feature-<N>.md
```

The file is named after the feature issue number, so parallel feature branches
never collide on the filename. The file is part of the feature's PR diff —
reviewable, challengeable, revisable alongside the code it describes.

Dev sessions are **forbidden from modifying `docs/ARCHITECTURE.md` directly**.
The shipped architecture is read-only on feature branches. The feature's
contribution lives in `docs/new/feature-<N>.md` until the feature merges.

### On merge to main

A merge-triggered GitHub Actions workflow runs:

```yaml
on:
  push:
    branches: [main]
    paths:
      - 'docs/new/**.md'

concurrency:
  group: architecture-integration-${{ github.repository }}
  cancel-in-progress: false
```

Two guarantees come from the configuration:

- **Path filter:** the workflow only runs when a merge actually introduced a
  `docs/new/*.md` file. Features with no architectural impact do not pay
  any workflow cost.
- **Concurrency with `cancel-in-progress: false`:** rapid consecutive merges
  queue behind each other rather than racing. No run is cancelled; each
  processes what it sees and exits.

### Flow

Each workflow run executes this sequence:

1. Check out `main` at latest commit.
2. Snapshot the current list of files under `docs/new/`.
3. If the snapshot is empty → log "nothing to integrate" and exit 0. **No AI
   invocation occurs.**
4. Invoke the AI integration recipe with the explicit snapshotted file list as
   input, alongside the current `docs/ARCHITECTURE.md`.
5. The AI produces an updated `docs/ARCHITECTURE.md` that integrates the new
   knowledge at the right structural location — not a chronological appendix.
6. Commit the integrated `ARCHITECTURE.md` and delete exactly the files
   listed in the snapshot from step 2. (Files that arrived *during* the run
   are left in place — they are another run's input.)
7. Push.

### The invariant

**In steady state, `docs/new/` is empty on `main`.** A transient window
between a feature's merge and the workflow's completion is acceptable — seconds
to minutes, bounded by the workflow's runtime. Files lingering beyond that
window signal a failure (workflow did not run, AI integration errored,
manual push bypassed the workflow) and should be surfaced by monitoring.

### Why integration, not append

An append-only consolidation would produce a chronological log — a history of
what each feature added. The `git log` already holds that history. The value of
`ARCHITECTURE.md` is as a **coherent current-state description**, not a diary.
The AI's job is to place each new fact where it structurally belongs, preserving
existing content, so the doc remains readable front-to-back.

### Edge cases

- **Two features merge within seconds.** Run #1 sees both files by the time it
  snapshots (step 2) and processes both. Run #2 starts (queued), finds an empty
  directory, and exits gracefully.
- **A third feature merges mid-run.** Run #1 only deletes what it snapshotted,
  leaving the new arrival for run #2 or later.
- **AI call fails.** The workflow errors; files remain in `docs/new/`; the
  next qualifying merge retries. Persistent failure surfaces via the stale-
  pending monitor.
- **Feature has no architectural impact.** No `docs/new/` file is created.
  The path filter prevents the workflow from triggering at all.

### Self-trigger

The workflow's own commit includes deletions under `docs/new/**.md`, which the
same path filter would match. Re-triggering would burn workflow minutes and
churn the concurrency queue with no work to do.

Mitigation is an implementation-time choice between:

- `[skip ci]` (or an equivalent marker) in the bot commit message, which
  GitHub Actions respects for `push` and `pull_request` events, or
- An `if: github.actor != 'github-actions[bot]'` guard on the job.

The `if:` guard is more robust across future maintenance edits (it does not
depend on commit-message convention), at the cost of spinning up a runner only
to exit. `[skip ci]` skips the runner entirely but depends on commit-message
discipline.

---

## Loading mechanism

Skills that shape design or code load knowledge at session start. The
mechanism mirrors the `.agents/` mount used by the framework itself.

### `.cp/` — the control plane knowledge mount

| Property | `.agents/` | `.cp/` |
|---|---|---|
| Gitignored in domain repo | Yes | Yes |
| Read-only in domain repo | Yes — never edit in place | Yes — never edit in place |
| Source | `eddiecarpenter/gh-agentic` at a pinned version | The control plane repo, `main` branch |
| Populated by | `gh agentic upgrade <version>` | `gh agentic project join` (initial) and `gh agentic project sync` (refresh) |
| Refresh cadence | Manual — deliberate upgrade | Automatic on session-init (`git pull`) |
| Writes go where | PRs to `gh-agentic` | PRs to the control plane repo |
| Reflects | A chosen framework version | Currently-reviewed CP state |

`.agents/` is pinned because framework upgrades are a decision. `.cp/` tracks CP
main because knowledge updates should propagate everywhere as soon as reviewed.

### Contents of `.cp/`

A sparse checkout of the CP repo containing only `docs/`:

```
.cp/
├── SYSTEM_BRIEF.md
└── SYSTEM_ARCHITECTURE.md
```

The sparse checkout keeps the mount small and the refresh fast regardless of
what else the CP repo accumulates over time.

### Session-init behaviour

For every federated-topology session, session-init:

1. Detects topology (federated or single).
2. In federated mode, checks that `.cp/` exists. Missing `.cp/` is a hard
   failure — `gh agentic project join` should have established it.
3. Runs `git pull origin main` inside `.cp/`.
   - Success → continue.
   - Network failure → warn, proceed with the existing `.cp/` state. Sessions
     remain usable offline with slightly-stale CP knowledge.
   - Merge conflict (unexpected under the read-only discipline) → hard fail
     and surface for repair.
4. Proceeds with the session.

### Cross-reads — the rare direction

Scoping a cross-domain requirement on the CP occasionally needs to read an
affected domain's `BRIEF.md` and `ARCHITECTURE.md`. These reads are rare (only
during cross-domain scoping) and the content is read-once-and-discarded. No
mount is installed in that direction — the CP reads via the GitHub API
(`gh api repos/<owner>/<domain>/contents/docs/BRIEF.md` or equivalent) on
demand.

---

## Branch protection on the control plane

Branch protection on CP `main` is the safeguard that makes "always mount main"
safe. Without it, any direct push to CP main would propagate unreviewed content
to every domain repo on the next sync.

- `gh agentic project check` flags CP main without branch protection as a
  warning.
- `gh agentic project create` sets up branch protection as part of scaffolding
  where API permissions allow. Where they do not, the recommendation is
  surfaced prominently.

Branch protection is not a hard prerequisite — personal-account repos sometimes
cannot configure it — but it is strongly nudged, and its absence is a visible
gap in `project check` output.

---

## Requirements and features — where they live

Requirements and features are GitHub Issues, not files. The knowledge plane
does not store them.

- **Cross-domain requirements** live as Issues on the **control plane** repo.
  The scoping session produces **one Feature per affected domain**, each
  Feature living in its own domain repo, each linked to the parent
  Requirement via `Closes part of owner/cp-repo#N`.
- **Domain-scoped requirements** live as Issues on the domain repo itself.
- **Features are always per-repo.** A feature maps to a single branch in a
  single repo. Cross-domain ordering is expressed with
  `blocked-by: owner/repo#N` on the dependent feature; the human holds
  `in-design` on a dependent feature until its blocker closes.

This is why no filesystem `docs/requirements/` directory exists. The
motivation shared across repos lives in the Requirement issue's body on the CP,
referenced by every child Feature.

---

## Decisions — no standalone concept

Generic architectural decision records (ADRs) are not a knowledge-plane
artefact. In an AI-native workflow the AI rebuilds context from the current
state each session; it does not consult a historical decision log. In
practice decisions were absorbed into architecture docs with no loss of
useful information.

The **narrow** use of the `decision` label — for contract-change audit
trails, as defined in `RULEBOOK.md` — is retained. That use preserves the
"which consumers were checked and why" evidence the contract rules require.
It is not an ADR practice; it is a mandatory audit artefact for a
specific, rule-bound situation.

No `docs/decisions/` directory exists in either the CP or domain repos.

---

## Legacy relics

Two artefacts from the framework's file-based era have been removed from the
knowledge plane:

- **`docs/federation.yaml`** — replaced by GitHub Project linkage and the
  `AGENTIC_CONTROL_PLANE` marker variable. Membership is now a GitHub-enforced
  property, not a synced file.
- **`docs/requirements/`** — replaced by GitHub Issues. Requirements are
  Issues; there is no parallel filesystem artefact.

A reader encountering either in old documentation or a legacy repo should
treat them as obsolete and remove them when touching the surrounding
content.

---

## Cross-references

- `project-command.md` — the control plane and `gh agentic project` commands.
- `feature-switches.md` — how features reach users; orthogonal to the
  knowledge plane but often referenced from the same scoping sessions.
- `delivery-philosophy.md` — the pipeline's scope and the
  deploy/release/enable distinction.
- `RULEBOOK.md` Contract Rules — the narrow `decision`-label usage.
