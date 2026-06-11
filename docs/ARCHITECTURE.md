# ARCHITECTURE.md — gh-agentic

## Overview

`gh-agentic` is a GitHub CLI extension that bootstraps and manages agentic
software delivery environments. It is a single compiled Go binary distributed via
`gh extension install`.

The extension serves two roles:

1. **CLI tooling** — commands to initialise repos, mount the framework, manage
   credentials, and run health checks.
2. **Framework source** — the canonical source for the AI-Native Delivery Framework
   files (`skills/`, `standards/`, `concepts/`, `recipes/`). Domain repos mount
   these files at `.agents/` via `gh agentic upgrade`.

---

## Package structure

```
gh-agentic/
├── cmd/
│   └── gh-agentic/
│       └── main.go              ← entrypoint, wires cobra root command
├── internal/
│   ├── cli/
│   │   ├── root.go              ← root command, version flag
│   │   ├── init.go              ← `gh agentic init` subcommand
│   │   ├── mount.go             ← `gh agentic mount` subcommand (internal; superseded by upgrade/repair for user flows)
│   │   ├── auth.go              ← `gh agentic auth` subcommand (login, refresh, check)
│   │   ├── check.go             ← `gh agentic check` subcommand
│   │   ├── repair.go            ← `gh agentic repair` subcommand
│   │   ├── upgrade.go           ← `gh agentic upgrade` subcommand
│   │   ├── project.go           ← `gh agentic project` subcommand tree
│   │   ├── info.go              ← `gh agentic info` subcommand
│   │   ├── status.go            ← `gh agentic status` subcommand
│   │   └── pipeline.go          ← `gh agentic status pipeline` subcommand
│   ├── init/                    ← init wizard logic
│   ├── mount/                   ← mount logic (first-time, remount, switch)
│   │   └── templates/           ← embedded templates for generated files
│   ├── auth/                    ← credential management (login, refresh, check)
│   ├── doctor/                  ← grouped health checks
│   ├── project/                 ← agentic-project management (create, join, switch)
│   ├── projectstatus/           ← pipeline status reporting
│   ├── scope/                   ← shared scope routing for gh variable/secret set
│   ├── frameworkcheck/          ← framework-sync helpers
│   ├── tarball/                 ← tarball download and extraction
│   ├── fsutil/                  ← filesystem utilities
│   ├── testutil/                ← shared test helpers
│   └── ui/
│       └── styles.go            ← lipgloss styles, GitHub colour palette
├── skills/                      ← framework playbooks (session procedures)
├── standards/                   ← language and framework standards
├── concepts/                    ← architectural concepts and guidance
├── recipes/                     ← goose recipe definitions
├── docs/
│   ├── PROJECT_BRIEF.md
│   └── ARCHITECTURE.md          ← this file
├── .github/workflows/
│   ├── agentic-pipeline.yml     ← domain repo wrapper workflow
│   ├── agentic-pipeline-reusable.yml ← reusable pipeline workflow
│   ├── build-and-test.yml       ← CI build and test
│   └── publish-release.yml      ← release publishing
├── RULEBOOK.md                  ← agent rulebook (active in all sessions)
├── LOCALRULES.md                ← project-specific rule overrides
├── AGENTS.md                    ← agent entrypoint (references RULEBOOK + LOCALRULES)
├── CLAUDE.md                    ← Claude Code entrypoint (references AGENTS.md)
├── go.mod
└── go.sum
```

---

## Mount model

Domain repos consume the framework via a **mount** mechanism rather than copying
template files directly.

### How it works

1. **Pinned version** — The framework version is pinned per repo by that
   repo's own `AGENTIC_FRAMEWORK_VERSION` GitHub Actions variable. Every repo
   resolves its version independently — there is no control-plane broadcast.
   The canonical resolver in `internal/project/` (`project.Resolve`, reading
   `AGENTIC_FRAMEWORK_VERSION` on the current repo) is the single code path
   that answers "what version should I mount?". (Federation does not change
   this — see the Federation section below; a federation is a scoping-time
   relationship between repos, not a runtime version-distribution mechanism.)

2. **`.agents/` directory** — The mounted framework. This directory is **gitignored**
   and populated on demand by `gh agentic upgrade` (to change version) or
   `gh agentic repair` (to resync). It is not committed. The cloned framework's
   own `.git` metadata records the exact tag — that is the local source of truth
   after the clone runs.

3. **Fetch mechanism** — `gh agentic upgrade` downloads the framework via
   `git clone --depth 1 --branch <version>` against the `eddiecarpenter/gh-agentic`
   release at the pinned version. It extracts framework files (`skills/`,
   `standards/`, `concepts/`, `recipes/`, `RULEBOOK.md`) into `.agents/`.

4. **Mount flows** — The mount command supports three flows, all driven by
   the resolver's answer to "what version is pinned?":
   - **First-time** (no `.agents/` directory yet): downloads the framework,
     generates `CLAUDE.md`, `AGENTS.md`, and wrapper workflows.
   - **Remount** (`mount` with no args, `.agents/` already present at the pinned
     version): re-downloads at the current pinned version. Used after a
     fresh clone or to repair a corrupted `.agents/`.
   - **Version switch** (`mount <new-version>` or a change to this repo's
     pinned `AGENTIC_FRAMEWORK_VERSION`): prompts for confirmation, remounts
     at the new version, updates wrapper-workflow tags.

5. **Reusable workflows** — Domain repos invoke the agentic pipeline via thin
   wrapper workflows in `.github/workflows/` that call reusable workflows
   defined in `eddiecarpenter/gh-agentic`. The reusable workflow reads
   `${{ vars.AGENTIC_FRAMEWORK_VERSION }}` to pick the framework version at
   runtime — consistent with what `project.Resolve` exposes to the CLI.

### In domain repos

```
my-domain-repo/
├── .agents/                 ← gitignored — mounted framework files
│   ├── RULEBOOK.md
│   ├── skills/
│   ├── standards/
│   ├── concepts/
│   └── recipes/
├── CLAUDE.md            ← committed — references .agents/AGENTS.md
├── AGENTS.md            ← committed — references RULEBOOK + LOCALRULES
├── LOCALRULES.md        ← committed — project-specific overrides
└── .github/workflows/
    └── agentic-pipeline.yml  ← wrapper calling reusable workflow
```

---

## Federation

Federation is a **scoping-time** concern, not a runtime one. A federation is a
multi-repo project where requirements are captured in one repo (the requirements
or umbrella repo, which holds domain knowledge) and the Features scoped from them
are created in the implementation repos where the work will actually happen. In a
single-topology project those are the same repo; in a federation they differ —
and that is the *only* difference. Everything downstream of scoping (design,
dev-session, compliance-verify, PR review) operates within a single repo and is
identical to single-topology.

> **Note — earlier model removed.** Federation was previously a runtime concern
> implemented through a control-plane role that broadcast the framework version,
> org-level shared-variable routing, a `.cp/` sparse-checkout mount, three-way
> topology inference, and `Closes owner/repo#N` text parsing for cross-repo links.
> That model was removed (Requirement #823 / Feature #835). It never ran
> end-to-end, so there is no production state to migrate. Implementation repos are
> now plain single-topology repos with no federation-specific configuration.

### `FEDERATION.md` manifest

The presence of a `FEDERATION.md` file at a repo's root is the sole signal that
the repo is a federation requirements repo. No topology variable, no role
inference — `project.IsFederationRepo` is a stat-only presence check, and a repo
without the manifest behaves as single topology (Features are always created in
the same repo as the requirement). The manifest is a small YAML document listing
the federation's target implementation repos, each with a purpose:

```yaml
repos:
  - name: owner/charging-domain
    purpose: Charging domain — rating, balance management, charging events
  - name: owner/billing-domain
    purpose: Billing domain — invoice generation, bill runs, statements
```

`project.ReadFederation` parses and validates it (`gh agentic check` surfaces any
validation error; `gh agentic info` lists the target repos). The manifest gives
the scoping agent its candidate target set and the human an orientation page.

### How federation works at scoping time

- During scoping in a manifest-bearing repo, the agent proposes a target repo for
  each Feature from the manifest's purpose descriptions; the human confirms or
  overrides. A Feature must fit entirely in one repo — a Feature spanning two is
  split into one Feature per repo at the decomposition checkpoint.
- Features are created in their target repo and wired as **cross-repo sub-issues**
  of their requirement (GitHub sub-issues work across repos within the same
  owner), so a requirement's Feature list and progress are visible natively on the
  requirement issue regardless of which repos the Features live in.
- One GitHub Project spans the whole federation. `gh agentic status` answers
  "where is everything" across all linked repos, and a requirement is closed only
  when all of its cross-repo Features are complete.
- `gh agentic check` / `repair` validate that `FEDERATION.md` and the GitHub
  Project's linked repos stay in sync.

The **placement rule** generalises this: an issue lives in the most specific repo
that fully contains its scope — Features always fit one implementation repo;
requirements live in the domain repo that contains them; cross-domain requirements
live in the umbrella repo.

---

## Self-relative paths

Framework files (`skills/`, `standards/`, `concepts/`) use **self-relative paths**
that resolve correctly in two contexts:

- **At the gh-agentic root** — when working directly in this repository, paths
  like `skills/dev-session.md` resolve relative to the repo root where these
  directories live.
- **When mounted as `.agents/` in domain repos** — the same paths resolve relative
  to the `.agents/` mount point. References within framework files (e.g.
  `@RULEBOOK.md`, `standards/go.md`) work because they are relative to the
  directory containing the referencing file.

This means framework files never use absolute paths or paths that assume a
specific repo structure. A reference like `concepts/delivery-philosophy.md`
works identically whether the file is at `/gh-agentic/concepts/` or at
`/my-repo/.agents/concepts/`.

---

## Credential management

The `gh agentic auth` command manages Claude Code credentials for CI runners.

### Auth subcommands

| Subcommand | Description |
|---|---|
| `auth login` | Forces a Claude Code login via `claude -p "Say Hi"`, reads credentials from the platform-appropriate store, and pushes them to the repo secret |
| `auth refresh` | Reads current local credentials and pushes them to the repo secret (no login prompt) |
| `auth check` | Verifies credentials are present and not expired |

### Credential stores

Credentials are read from the platform-appropriate location:

- **macOS** — macOS keychain, service name `"Claude Code-credentials"`
- **Linux / other** — `~/.claude/.credentials.json`

Credentials are pushed to the GitHub repo as an encrypted secret, making them
available to CI runners without manual configuration.

---

## Commands

| Command | Description |
|---|---|
| `gh agentic init` | Interactive wizard to initialise a new agentic environment |
| `gh agentic check` | Verify project membership and pipeline readiness |
| `gh agentic repair` | Auto-fix issues reported by `check` |
| `gh agentic upgrade [version]` | Install or change this repo's pinned framework version at `.agents/` |
| `gh agentic project` | Manage ongoing project membership — create, join, switch, unlink |
| `gh agentic info` | Show the current state of this repo's agentic setup |
| `gh agentic auth` | Manage Claude credentials used by the agent pipeline (login, refresh, check) |
| `gh agentic status` | Show pipeline state across requirements and features |
| `gh agentic status pipeline` | Render requirements and features as a side-by-side pipeline view |

---

## Layering rules

```
cmd/gh-agentic/main.go
    └── internal/cli/        ← cobra commands only — no business logic
            └── internal/*/  ← all logic lives here
                    └── go-gh, huh, lipgloss, bubbles
```

- `internal/cli/` contains only cobra command definitions and flag parsing.
  All logic is delegated to the relevant `internal/` package.
- `internal/init/`, `internal/mount/`, `internal/auth/`, `internal/doctor/`
  own their respective business logic. They have no knowledge of cobra.
- `internal/project/` owns agentic-project management — create, join, switch,
  unlink — and the shared check/repair helpers that the cobra commands compose.
- `internal/ui/` owns the shared colour palette and lipgloss styles. No other
  package defines styles inline.
- `internal/tarball/` provides shared tarball download and extraction used by
  both mount and init.
- `internal/fsutil/` provides filesystem utilities shared across packages.
- `internal/testutil/` provides shared test helpers.
- `internal/scope/` provides the shared `ScopeFor` routing used by the lower-
  level packages (auth, init) and re-exported via `internal/project/`.

---

## Key dependencies

| Library | Why |
|---|---|
| `github.com/cli/go-gh/v2` | GitHub API clients pre-authenticated from `gh` credentials. Eliminates all token handling. |
| `github.com/charmbracelet/huh` | Declarative TUI forms — used in init wizard for configuration collection. |
| `github.com/charmbracelet/lipgloss` | Terminal styling — GitHub colour palette, borders, summary boxes. |
| `github.com/charmbracelet/bubbles` | Spinner component shown while each execution step runs. |
| `github.com/spf13/cobra` | CLI command routing — root command and subcommands. |

---

## GitHub authentication

Authentication is fully delegated to `gh`. The extension inherits the
authenticated user's credentials automatically via `go-gh`. No PAT handling,
no `gh auth login` checks beyond verifying `gh auth status` in preflight.

---

## Extension distribution

Binaries are distributed via GitHub Releases using the
`gh-extension-precompile` GitHub Action. `gh extension install` downloads
the appropriate binary for the user's platform automatically.

Upgrade: `gh extension upgrade agentic`
