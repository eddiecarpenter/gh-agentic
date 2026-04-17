# concept: project command

This document defines the design of the `gh agentic project` command group — its
subcommands, guards, and the topology model it implements.

For the content and organisation of project knowledge (briefs, architecture,
the `.cp/` mount, the `docs/new/` pattern), see `knowledge-plane.md`.

---

## Topology Model

Topology is **derived**, not configured. The agent determines topology at runtime by:

1. Reading `AGENTIC_PROJECT_ID` from the current repo's variables (repo-level only — see below)
2. Reading `AGENTIC_CONTROL_PLANE` from the current repo's variables (set to `true` on exactly one repo per federation — the control plane)
3. Querying the GitHub ProjectV2 API for the project's linked repositories:

```graphql
query {
  node(id: "PVT_xxx") {
    ... on ProjectV2 {
      repositories(first: 10) {
        nodes {
          name
          nameWithOwner
          url
        }
      }
    }
  }
}
```

4. Determining topology from the combination:

| Current repo state | Topology |
|---|---|
| Only member linked to the Project and `AGENTIC_CONTROL_PLANE=true` | **Single** — embedded control plane |
| `AGENTIC_CONTROL_PLANE=true` and ≥1 other repo is linked | **Federated — control plane** |
| `AGENTIC_CONTROL_PLANE` unset and the current repo is among the linked repos | **Federated — domain** |

**Every member repo is linked to the Project** — control plane and domain repos alike.
The control plane is identified by its `AGENTIC_CONTROL_PLANE=true` marker variable,
not by being the sole linked repo. This keeps membership discoverable from the Project
itself and lets `gh agentic` identify each member's role with one variable read.

The symmetric query (check whether this repo is already linked to any project) uses:

```graphql
query {
  repository(owner: "OWNER", name: "REPO") {
    projectsV2(first: 10) {
      nodes { id title }
    }
  }
}
```

---

## AGENTIC_PROJECT_ID

- **Must be set at repo level** — never at org level
- Org-level would cause every repo in the org to inherit the variable, incorrectly
  treating every repo as a project member
- `project check` flags it as an error if detected at org level
- Set by `project create` (on the control plane repo) and `project join` (on domain repos)
- Removed by `project unlink`

## AGENTIC_CONTROL_PLANE

- **Set to `true` on exactly one repo per federation** — the control plane
- Must be set at repo level (never at org level)
- Set by `project create` on the repo being established as the control plane
- Unset / absent on every domain repo
- `project check` verifies exactly one repo in the federation carries this marker

---

## Org vs Personal Account

Federated topology works on personal accounts but carries operational cost:

| | Org | Personal |
|---|---|---|
| Variables | Set once at org level, inherited | Must be set per-repo |
| Secrets | Set once at org level, inherited | Must be set per-repo |
| Runners | Register once, shared | Must register per-repo |

`project create` warns (does not block) when federated + personal account is detected.
`project check` and `project repair` are the mechanism for catching and fixing
missing per-repo variables in personal-account federated setups.

---

## Subcommands

### `project create`

Creates a new GitHub Project and establishes this repo as the control plane.

**Guards (block if any are true):**
- `AGENTIC_PROJECT_ID` repo variable already exists — already part of a project
- Repo already appears in a project's `repositories` — already linked elsewhere

**Actions:**
- Create GitHub Project
- Link this repo to the project
- Set `AGENTIC_PROJECT_ID` as a repo-level variable
- Set `AGENTIC_CONTROL_PLANE=true` as a repo-level variable
- Mount the framework (`.ai/`)
- Scaffold project labels, runner, and project board columns
- Configure branch protection on `main` where API permissions allow (strongly
  recommended — see `knowledge-plane.md`)
- Warn if personal account (federated per-repo maintenance overhead)

---

### `project join`

Joins an existing project as a domain repo. The repo is linked to the GitHub
Project and marked as a domain (no `AGENTIC_CONTROL_PLANE` variable).

**Guards:**

| Current state | Behaviour |
|---|---|
| Is control plane + `docs/` has files | **Block** — migrate system-level content to the new control plane first |
| Is control plane + `docs/` empty | Warn + confirm |
| Already federated member of a different project | Warn + confirm (re-affiliation) |
| Already federated member of the same project | Info only — no change |
| No affiliation | Proceed |

**Actions:**
- Link this repo to the GitHub Project
- Set `AGENTIC_PROJECT_ID` as a repo-level variable
- Ensure `AGENTIC_CONTROL_PLANE` is **not** set (this is a domain repo)
- Mount the framework (`.ai/`)
- Mount the control plane knowledge into `.cp/` — sparse checkout of the CP
  repo's `docs/` at `main`. See `knowledge-plane.md` for the mount's contents
  and refresh cadence.

---

### `project sync`

Refreshes the `.cp/` mount on a domain repo. Runs `git pull origin main` inside
`.cp/` to update system-level knowledge from the control plane.

Session-init runs this automatically at the start of every federated-domain
session. `project sync` is available for explicit refresh on demand.

See `knowledge-plane.md` for the session-init behaviour and failure modes.

---

### `project unlink`

Removes this repo's project affiliation.

**Guards:**

| Current state | Behaviour |
|---|---|
| Is control plane + `docs/` has files | **Block** — migrate system-level content first |
| Is control plane + `docs/` empty | Warn + confirm |
| Federated member | Warn + confirm (`.cp/` cache will be removed) |
| No affiliation | Nothing to do |

**Actions:**
- Unlink the repo from the GitHub Project
- Delete `AGENTIC_PROJECT_ID` and, if present, `AGENTIC_CONTROL_PLANE` repo variables
- Remove `.cp/` if it exists

---

### `project check`

Verifies project health. Reports:
- `AGENTIC_PROJECT_ID` present and at repo level (not org level)
- `AGENTIC_CONTROL_PLANE` presence consistent with topology (exactly one CP across the federation)
- Project exists and is accessible
- Current repo's role (control plane or domain) matches the marker variable
- Framework mounted and at expected version
- `.cp/` present and fresh (for domain repos)
- Branch protection on CP `main` — warning only if missing (strongly recommended)
- Required variables and secrets present
- Runner registered and reachable

Exits non-zero if any hard check fails. Missing branch protection is a warning,
not a hard failure.

---

### `project repair`

Fixes issues reported by `project check`. Interactive — confirms each repair action
before applying. Includes re-establishing `.cp/` if missing or corrupted.

---

### `project info`

Displays current project state:
- Project name and ID
- Topology (Single / Federated — derived)
- Current repo's role (control plane / domain)
- All linked repositories
- Framework version
- `.cp/` freshness (for domain repos)
- Runner label

---

## Knowledge — see `knowledge-plane.md`

The content of `docs/` and the control plane knowledge mount (`.cp/`) are
defined by the knowledge plane. In brief:

| Topology | Location | Writeable |
|---|---|---|
| Single (embedded) | Local `docs/` — holds both system-level and repo-level knowledge | Yes |
| Federated (domain repo) | Local `docs/` (`BRIEF.md` + `ARCHITECTURE.md`) + `.cp/` mount (`SYSTEM_BRIEF.md` + `SYSTEM_ARCHITECTURE.md`) | `docs/` yes; `.cp/` no — write via PR to the CP repo |
| Federated (control plane) | Local `docs/` (`SYSTEM_BRIEF.md` + `SYSTEM_ARCHITECTURE.md` only) | Yes |

See `knowledge-plane.md` for the full model — naming rules, update mechanisms,
the `docs/new/` pattern for `ARCHITECTURE.md` contributions, and session-init
loading behaviour.

---

## Migration: Single → Federated

The supported path for moving a single (embedded control plane) repo into
a federated project is:

1. Create a new control plane repo with `project create`.
2. Migrate system-level content from the original repo's `docs/` into the new
   control plane's `docs/` as `SYSTEM_BRIEF.md` and `SYSTEM_ARCHITECTURE.md`.
   Leave repo-level content (`BRIEF.md`, `ARCHITECTURE.md`) in the original
   repo — it describes that repo's own responsibility within the federation.
3. Run `project join <new-project-id>` on the original repo (will unblock once
   system-level content is migrated and the original repo's `docs/` holds only
   repo-level artefacts).

`project join` blocks this move automatically when `docs/` has system-level
content, forcing the migration to happen explicitly before re-affiliation.

---

## Replaces

| Old command | New command |
|---|---|
| `bootstrap` | `project create` |
| `inception` | `project join` |
| `init` | Interactive layer under `project create` (single) or `project join` (domain) |
