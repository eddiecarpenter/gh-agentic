# concept: project command

This document defines the design of the `gh agentic project` command group — its
subcommands, guards, and the topology model it implements.

---

## Topology Model

Topology is **derived**, not configured. The agent determines topology at runtime by:

1. Reading `AGENTIC_PROJECT_ID` from the current repo's variables (repo-level only — see below)
2. Querying the GitHub ProjectV2 API for the project's linked repositories:

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

3. Comparing the linked repo against the current repo:

| Result | Topology |
|---|---|
| Linked repo == current repo | **Single** — embedded control plane; `docs/` is local |
| Linked repo != current repo | **Federated** — control plane is the linked repo; `docs/` is read from it |

Only the **control plane repo** is linked to the GitHub Project. Domain repos hold
`AGENTIC_PROJECT_ID` as a repo variable but are **not** linked. This keeps the
`repositories` query unambiguous — it always returns exactly one repo: the control plane.

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

Creates a new GitHub Project and establishes this repo as the embedded control plane.

**Guards (block if any are true):**
- `AGENTIC_PROJECT_ID` repo variable already exists — already part of a project
- Repo already appears in a project's `repositories` — already linked elsewhere

**Actions:**
- Create GitHub Project
- Link this repo to the project
- Set `AGENTIC_PROJECT_ID` as a repo-level variable
- Mount the framework (`.ai/`)
- Scaffold project labels, runner, and project board columns
- Warn if personal account (federated per-repo maintenance overhead)

---

### `project join`

Joins an existing project as a domain repo. The repo is NOT linked to the GitHub Project —
it only holds `AGENTIC_PROJECT_ID`.

**Guards:**

| Current state | Behaviour |
|---|---|
| Is embedded control plane + `docs/` has files | **Block** — migrate `docs/` to new control plane first |
| Is embedded control plane + `docs/` empty | Warn + confirm |
| Already federated member of a different project | Warn + confirm (re-affiliation) |
| Already federated member of the same project | Info only — no change |
| No affiliation | Proceed |

**Actions:**
- Set `AGENTIC_PROJECT_ID` as a repo-level variable
- Mount the framework (`.ai/`)
- For federated: read `docs/` from the control plane repo during session init

---

### `project unlink`

Removes this repo's project affiliation.

**Guards:**

| Current state | Behaviour |
|---|---|
| Is embedded control plane + `docs/` has files | **Block** — migrate `docs/` first |
| Is embedded control plane + `docs/` empty | Warn + confirm |
| Federated member | Warn + confirm (local `docs/` cache will be removed) |
| No affiliation | Nothing to do |

**Actions:**
- Delete `AGENTIC_PROJECT_ID` repo variable
- Unmount `docs/` (remove local cache if federated)

---

### `project check`

Verifies project health. Reports:
- `AGENTIC_PROJECT_ID` present and at repo level (not org level)
- Project exists and is accessible
- Linked repo is correct (control plane identification)
- Framework mounted and at expected version
- Required variables and secrets present
- Runner registered and reachable

Exits non-zero if any check fails.

---

### `project repair`

Fixes issues reported by `project check`. Interactive — confirms each repair action
before applying.

---

### `project info`

Displays current project state:
- Project name and ID
- Topology (Single / Federated — derived)
- Control plane repo
- Linked repositories
- Framework version
- Runner label

---

## `docs/` — Federation Knowledge

| Topology | Location | Writeable |
|---|---|---|
| Single (embedded) | Local `docs/` | Yes |
| Federated (domain repo) | Read from control plane repo | No (read via API) |
| Federated (control plane) | Local `docs/` | Yes |

The control plane `docs/` is the source of truth for the federation:
- `docs/federation.yaml` — member repos, project metadata
- `docs/requirements/` — cross-repo requirement documents
- `docs/decisions/` — ADRs and decision records

---

## Migration: Single → Federated

The **only supported path** for moving a single (embedded control plane) repo into
a federated project is:

1. Create a new control plane repo with `project create`
2. Manually migrate `docs/` content to the new control plane
3. Run `project join <new-project-id>` on the original repo (will unblock once `docs/` is empty or migrated)

`project join` blocks this move automatically when `docs/` has content, forcing
the migration to happen explicitly before re-affiliation.

---

## Replaces

| Old command | New command |
|---|---|
| `bootstrap` | `project create` |
| `inception` | `project join` |
| `init` | Interactive layer under `project create` (single) or `project join` (domain) |
