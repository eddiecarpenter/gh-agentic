# concept: project command

This document defines the design of the `gh agentic project` command group — its
subcommands, guards, and the topology model it implements.

For the content and organisation of project knowledge (briefs, architecture,
and the `docs/` pattern), see `knowledge-plane.md`.

---

## Topology Model

Topology is **derived from FEDERATION.md presence**, not from runtime variables.

The rule is simple and binary:

| `FEDERATION.md` at repo root | Topology |
|---|---|
| Present (valid YAML, non-empty `repos` list) | **Federation** — this repo is the federation requirements repo |
| Absent | **Single** — standalone or domain repo |

`gh agentic` calls `project.IsFederationRepo(root)` — a single `os.Stat` check on
`FEDERATION.md` — to classify any repo. No variable reads, no GraphQL queries, and
no runtime environment are required. The topology is deterministic from the working tree.

### FEDERATION.md format

When a repo is the federation requirements repository, it declares the target
domain repos in `FEDERATION.md` at the repo root. The file is valid YAML with
the following structure:

```yaml
repos:
  - name: owner/domain-repo-1
    purpose: Short description of this repo's role in the federation
  - name: owner/domain-repo-2
    purpose: Another domain's purpose
```

Validation rules (enforced by `project.ReadFederation`):
- File must not be empty
- `repos` list must contain at least one entry
- Each entry requires `name` (in `owner/repo` format) and `purpose`
- Duplicate names are rejected

Domain repos (listed in `FEDERATION.md`) are plain single-topology repos — they
do not carry any special marker variable or manifest. The federation is declared
from the requirements repo outward.

---

## AGENTIC_PROJECT_ID

- **Must be set at repo level** — never at org level
- Org-level would cause every repo in the org to inherit the variable, incorrectly
  treating every repo as a project member
- Set by `project create` (on the control plane repo) and `project join` (on domain repos)
- Removed by `project unlink`

---

## Org vs Personal Account

Federation topology requires a GitHub Organisation because `EnsureFederatedOwnerIsOrg`
blocks federation setup on user-owned repos. All variables and secrets are --repo scoped
(Feature #824 removed org-scope routing entirely), so there is no per-repo variable
maintenance overhead as there was under the old model.

`project create` warns (does not block) when single + personal account is detected.

---

## Subcommands

### `project create`

Creates a new GitHub Project and establishes this repo as the control plane.

**Guards (block if any are true):**
- `AGENTIC_PROJECT_ID` repo variable already exists — already part of a project
- Repo already appears in a project's `repositories` — already linked elsewhere
- Federation topology + user owner (blocked by `EnsureFederatedOwnerIsOrg`)

**Actions:**
- Create GitHub Project
- Link this repo to the project
- Set `AGENTIC_PROJECT_ID` as a repo-level variable
- Set `AGENTIC_FRAMEWORK_VERSION` as a repo-level variable
- Mount the framework (`.agents/`)
- Scaffold project labels, runner, and project board columns
- Warn if personal account

---

### `project join`

Joins an existing project as a domain repo. The repo is linked to the GitHub
Project.

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
- Mount the framework (`.agents/`)

Domain repos joined via this command are single-topology repos — no special
filesystem mount or marker variable is required. They can read the federation requirements from
the control plane repo's `FEDERATION.md` via `gh agentic info` or the API.

---

### `project unlink`

Removes this repo's project affiliation.

**Guards:**

| Current state | Behaviour |
|---|---|
| Is control plane + `docs/` has files | **Block** — migrate system-level content first |
| Is control plane + `docs/` empty | Warn + confirm |
| Federated member | Warn + confirm |
| No affiliation | Nothing to do |

**Actions:**
- Unlink the repo from the GitHub Project
- Delete `AGENTIC_PROJECT_ID` repo variable

---

### `project check`

Verifies project health. Reports:
- `AGENTIC_PROJECT_ID` present and configured
- Project exists and is accessible
- Framework mounted and at expected version
- FEDERATION.md valid (when present)
- Legacy federation config present (warns when pre-Feature-#824 topology variables or directory layout are still present)
- Required variables and secrets present
- Runner registered and reachable

Exits non-zero if any hard check fails. Missing branch protection is a warning,
not a hard failure.

---

### `project repair`

Fixes issues reported by `project check`. Interactive — confirms each repair action
before applying.

---

### `project info`

Displays current project state:
- Project name and ID
- Topology (Single / Federation — derived from `FEDERATION.md` presence)
- All federation target repos (when `FEDERATION.md` is present)
- Framework version
- Runner label

---

## Knowledge — see `knowledge-plane.md`

The content of `docs/` is defined by the knowledge plane. In brief:

| Topology | Location | Writeable |
|---|---|---|
| Single (standalone or domain repo) | Local `docs/` — holds both system-level and repo-level knowledge | Yes |
| Federation (requirements repo) | Local `docs/` + `FEDERATION.md` | Yes |

Domain repos in a federation carry their own `docs/BRIEF.md` and
`docs/ARCHITECTURE.md`. System-level knowledge is maintained in the control
plane's `docs/` and accessed by domain agents via the GitHub API on demand
(no directory mount required).

See `knowledge-plane.md` for the full model — naming rules, the `docs/new/`
pattern, and session-init loading behaviour.

---

## Migration: Single → Federated

The supported path for moving a single (embedded control plane) repo into
a federated project is:

1. Create a new control plane repo with `project create`.
2. Add `FEDERATION.md` to the control plane repo listing all domain repos
   (including the original repo).
3. Migrate system-level content from the original repo's `docs/` into the new
   control plane's `docs/` as `SYSTEM_BRIEF.md` and `SYSTEM_ARCHITECTURE.md`.
   Leave repo-level content (`BRIEF.md`, `ARCHITECTURE.md`) in the original
   repo.
4. Run `project join <new-project-id>` on the original repo.

---

## Replaces

| Old command | New command |
|---|---|
| `bootstrap` | `project create` |
| `inception` | `project join` |
| `init` | Interactive layer under `project create` (single) or `project join` (domain) |
