# concept: project command

This document defines the design of the `gh agentic project` command group — its
subcommands, guards, and the topology model it implements.

For the content and organisation of project knowledge (briefs, architecture,
and the `docs/` pattern), see `knowledge-plane.md`.

---

## Topology Model

Topology is **derived from FEDERATION.yaml presence**, not from runtime variables.

The rule is simple and binary:

| `FEDERATION.yaml` at repo root | Topology |
|---|---|
| Present (valid YAML, domain-grouped `domains` list — may be empty) | **Federation** — this repo is the control plane |
| Absent | **Single** — standalone repo (its own control plane), or a pure-code domain repo |

`gh agentic` calls `project.IsFederationRepo(root)` — a single `os.Stat` check on
`FEDERATION.yaml` — to classify any repo. No variable reads, no GraphQL queries, and
no runtime environment are required. The topology is deterministic from the working tree.

### FEDERATION.yaml format

The control plane declares its domains — and the repos that implement them — in
`FEDERATION.yaml` at the repo root. The file is valid YAML, **domain-grouped**
(#871): a list of domains, each with a purpose and the repos under it.

```yaml
domains:
  - name: charging
    purpose: Rating, balance management, charging events
    repos:
      - name: owner/charging-rating
        purpose: Rating engine
      - name: owner/charging-balance
        purpose: Balance management
```

Validation rules (enforced by `project.ReadFederation`):
- An empty `domains:` list is valid — a control plane with no domains registered yet
- Each domain requires `name` and `purpose`
- Each repo entry requires `name` (in `owner/repo` format) and `purpose`
- Duplicate repo names are rejected
- The flat `repos:` schema is no longer accepted (hard cut in #871)

Domain repos (listed in `FEDERATION.yaml`) are **pure code** — no `.agents` mount,
no docs, no pipeline workflow. They carry only an `AGENTIC_PROJECT_ID` repo
variable, set when registered. The federation — including which domain each repo
belongs to — is declared entirely from the control plane's `FEDERATION.yaml`; no
per-repo marker or domain variable is required.

---

## AGENTIC_PROJECT_ID

- **Must be set at repo level** — never at org level
- Org-level would cause every repo in the org to inherit the variable, incorrectly
  treating every repo as a project member
- Set by `project create` (on the control plane repo) and `project join` (run on the
  control plane, sets the variable on the target domain repo)
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

Registers a pure-code domain repo with the federation. **Run on the control
plane** (#874), not in the domain repo:

```
gh agentic project join <owner/repo> --domain <name> [--purpose <text>] [--domain-purpose <text>]
```

It adds the target repo to the control plane's `FEDERATION.yaml` under the named
domain (lazy-creating the domain if it does not exist yet), links the repo to the
GitHub Project, and sets the target repo's `AGENTIC_PROJECT_ID`. It does **not**
mount the framework into the target repo — domain repos stay pure code.

**Guards:**

| Current state | Behaviour |
|---|---|
| Not run on a control plane (no local `FEDERATION.yaml`) | **Block** — join is a control-plane operation |
| Target repo already a member of a different project | Warn + confirm (re-affiliation) |
| Target repo already registered in this federation | Info only — no change |
| Otherwise | Proceed |

**Actions:**
- Add the target repo to `FEDERATION.yaml` under `--domain` (via `AddRepo` + `WriteFederation`)
- Link the target repo to the GitHub Project
- Set `AGENTIC_PROJECT_ID` on the target repo (no framework mount)

The domain a repo belongs to is read from the control plane's `FEDERATION.yaml` —
no per-repo domain variable. Agents in a domain repo read system-level knowledge
from the control plane via the GitHub API on demand.

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
- FEDERATION.yaml valid (when present)
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
- Topology (Single / Federation — derived from `FEDERATION.yaml` presence)
- All domains and their repos (when `FEDERATION.yaml` is present)
- Framework version
- Runner label

---

## Knowledge — see `knowledge-plane.md`

The content of `docs/` is defined by the knowledge plane. In brief:

| Topology | Location | Writeable |
|---|---|---|
| Single (standalone repo, its own control plane) | Local `docs/` — holds both system-level and repo-level knowledge | Yes |
| Federation (control plane) | Local `docs/` (system-level + `docs/domains/<domain>/`) + `FEDERATION.yaml` | Yes |

In a federation, all knowledge — system-level and per-domain — lives on the
control plane: system docs at the root (`SYSTEM_BRIEF.md`,
`SYSTEM_ARCHITECTURE.md`) and domain docs under `docs/domains/<domain>/`. Domain
repos are pure code and carry no docs; agents access control-plane knowledge via
the GitHub API on demand.

See `knowledge-plane.md` for the full model — naming rules, the `docs/new/`
pattern, and session-init loading behaviour.

---

## Migration: Single → Federated

The supported path for moving a single (embedded control plane) repo into
a federated project is:

1. Create a new control plane repo with `gh agentic init` → federated (or
   `project create` with federated topology), which scaffolds an empty
   `FEDERATION.yaml` plus the federated-tier system docs (#875).
2. Migrate system-level content from the original repo's `docs/` into the new
   control plane's `docs/` as `SYSTEM_BRIEF.md` and `SYSTEM_ARCHITECTURE.md`.
3. On the control plane, run `gh agentic project join <owner/repo> --domain <name>`
   for the original repo (and any other domain repos) to register each under a
   domain and link it to the Project (#874).

Migrating an existing federation that predates the control-plane-centralized
model — relocating features onto the control plane and removing stale domain-repo
mounts — is documented in `../docs/migration-cp-centralized.md`.

---

## Replaces

| Old command | New command |
|---|---|
| `bootstrap` | `project create` |
| `inception` | `project join` |
| `init` | Interactive layer under `project create` (single) or `project join` (domain) |
