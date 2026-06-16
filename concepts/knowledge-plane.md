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

### Single topology

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

Domain repos in a federation are single-topology repos. They carry their own
`docs/BRIEF.md` and `docs/ARCHITECTURE.md` describing their scope within the
federation. No special mount or marker variable is required on a domain repo.

### Federation topology

A federation requirements repo declares its target domain repos via `FEDERATION.md`
at the repo root. The control plane owns system-level knowledge; domain repos own
their own.

**Federation requirements (control plane) repo:**
```
control-plane-repo/
├── .agents/
├── FEDERATION.md                     # declares all domain repos + their purpose
└── docs/
    ├── SYSTEM_BRIEF.md               # system orientation
    └── SYSTEM_ARCHITECTURE.md        # seams between repos
```

**Domain repo (single topology):**
```
domain-repo/
├── .agents/                              # framework mount, read-only
├── docs/
│   ├── BRIEF.md                      # this domain's scope
│   ├── ARCHITECTURE.md               # this domain's internals
│   └── new/                          # staging for architectural contributions
└── (code)
```

Domain repos are plain single-topology repos. They do not carry any special
filesystem mount or marker variable. The federation is declared from the control plane
outward, not from domain repos inward.

#### Domain documentation tier (#871)

Under the domain-grouped `FEDERATION.md` schema, a *domain* is one or more
repos, and a domain's documentation lives centrally on the control plane —
not scattered across its member repos. The knowledge plane has exactly two
tiers, both homed on the control plane:

- **Federated / system tier** — the shared architecture and principles, at the
  control-plane `docs/` root (e.g. `SYSTEM_BRIEF.md`, `SYSTEM_ARCHITECTURE.md`).
  Never decentralised into domain repos.
- **Domain tier** — each domain's documentation under `docs/domains/<domain>/`,
  where `<domain>` is the domain's slug from `FEDERATION.md`. A domain spanning
  several repos therefore has a single documentation home.

There is no separate per-service documentation tier: member repos carry code,
not design docs. A domain whose repo list has one entry is just a single-repo
domain — its docs still live at `docs/domains/<domain>/`.

`gh agentic check` emits a *soft* warning (never a failure) when a manifest
domain has no `docs/domains/<domain>/` directory yet, since domain docs are
authored incrementally.

> The broader realignment of this concept to the control-plane-centralized
> execution model (pipeline-on-CP, pure-code domain repos) is tracked under
> requirement #870 / Feature #876; this section introduces the domain
> documentation tier and the `docs/domains/<domain>/` convention.

---

## Naming — scope-explicit

Different names at different scopes make the rule unambiguous:

> **Domain knowledge = `SYSTEM_BRIEF` + `SYSTEM_ARCHITECTURE` + `BRIEF` + `ARCHITECTURE`**

A skill in a domain repo loads up to four files. In federation mode the agent
reads the control plane's `docs/SYSTEM_BRIEF.md` and `docs/SYSTEM_ARCHITECTURE.md`
via the GitHub API on demand (no mount required — these reads are rare). In single
topology only the two unqualified files exist.

### What each file holds

| File | Answers | Scope |
|---|---|---|
| `SYSTEM_BRIEF.md` | Why the system as a whole exists | System (CP only) |
| `SYSTEM_ARCHITECTURE.md` | The seams between repos, cross-cutting concerns | System (CP only) |
| `BRIEF.md` | Why this repo exists within the system | Repo |
| `ARCHITECTURE.md` | How this repo works internally | Repo |

A system-level brief or architecture should *never* describe how a single
repo works internally. A repo-level brief or architecture should *never*
describe seams owned by another repo.

---

## Membership

In federation topology, **every member repo is linked to the GitHub Project**.
The control plane repo is identified by `FEDERATION.md` at its root — no
marker variable is required.

Discovery:
1. Query the Project's linked repositories — returns all N members
2. Find the repo whose root contains `FEDERATION.md` — that is the control plane
3. The rest are domain repos (single topology)

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

### Session-init behaviour

For every session, session-init:

1. Detects topology from `FEDERATION.md` presence (single or federation).
2. In federation mode, reads `FEDERATION.md` to identify target domain repos.
3. Loads `docs/BRIEF.md` and `docs/ARCHITECTURE.md` from the local repo.
4. For federation topology: reads `docs/SYSTEM_BRIEF.md` and
   `docs/SYSTEM_ARCHITECTURE.md` from the control plane repo via the GitHub
   API on demand (no local mount required).
5. Proceeds with the session.

Domain repos (single topology) load only their own `docs/` — no cross-repo
knowledge reads are required for ordinary single-topology sessions.

### Cross-reads — the rare direction

Scoping a cross-domain requirement on the CP occasionally needs to read an
affected domain's `BRIEF.md` and `ARCHITECTURE.md`. These reads are rare (only
during cross-domain scoping) and the content is read-once-and-discarded. No
mount is installed in that direction — the CP reads via the GitHub API
(`gh api repos/<owner>/<domain>/contents/docs/BRIEF.md` or equivalent) on
demand.

---

## Branch protection on the control plane

Branch protection on CP `main` prevents unreviewed content from reaching
domain agents via session-init reads.

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

- **Requirements** live as Issues on the **control plane** repo.
- **Features also live on the control plane**, as same-repo sub-issues of their
  parent Requirement. A federated Feature records the single implementation repo
  it targets in a "Target repo" ProjectV2 field (the owner is always the
  control-plane owner; the field holds the bare repo name); the pipeline reads
  that field to clone the target. This reverses the earlier model (#825), where
  Features were created in their target domain repo via cross-repo sub-issues —
  domain repos are now pure code with no pipeline of their own, so a Feature
  issue placed there would have no workflow to fire.
- **Features are always single-target.** A feature maps to a single branch in a
  single implementation repo. A feature whose work spans two repos is split at
  scoping into one Feature per repo, each with its own target. Ordering is
  expressed with `blocked-by: #N` on the dependent feature (same-repo issue
  references on the control plane); the human holds the design trigger on a
  dependent feature until its blocker closes.

This is why no filesystem `docs/requirements/` directory exists. The
motivation shared across repos lives in the Requirement issue's body on the CP,
referenced by every child Feature.

**CP-event triggering contract.** Because Features live on the control plane, the
pipeline fires on **control-plane issue events**: applying a trigger label (e.g.
`in-design`, `in-development`) to a Feature on the control plane starts the
corresponding phase, which reads the Feature's "Target repo" field to know which
repo to clone into `./project`. This CP-event triggering and the checkout it
drives are implemented by the pipeline workflow (Feature #873); requirement
scoping's job is to place the Feature on the control plane with its "Target repo"
field set, so the trigger has a well-formed issue to key off.

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

Three artefacts from the framework's previous federation model have been removed:

- **`docs/federation.yaml`** — replaced by GitHub Project linkage and later by
  `FEDERATION.md`. Membership is now declared in `FEDERATION.md` at the control
  plane repo root.
- **`docs/requirements/`** — replaced by GitHub Issues. Requirements are
  Issues; there is no parallel filesystem artefact.
- **Control plane knowledge mount** — the sparse-checkout of `docs/` from the
  control plane repo, previously installed by `project join` and refreshed by
  `project sync`. Domain repos are now plain single-topology repos; system-level
  knowledge is read on demand via the GitHub API rather than through a local
  filesystem mount.

A reader encountering any of these in old documentation or a legacy repo should
treat them as obsolete and remove them when touching the surrounding content.

---

## Cross-references

- `project-command.md` — the control plane and `gh agentic project` commands.
- `feature-switches.md` — how features reach users; orthogonal to the
  knowledge plane but often referenced from the same scoping sessions.
- `delivery-philosophy.md` — the pipeline's scope and the
  deploy/release/enable distinction.
- `RULEBOOK.md` Contract Rules — the narrow `decision`-label usage.
