# ARCHITECTURE.md — gh-agentic

## Overview

`gh-agentic` is a GitHub CLI extension that bootstraps and manages agentic
software delivery environments. It is a single compiled Go binary distributed via
`gh extension install`.

The extension serves two roles:

1. **CLI tooling** — commands to initialise repos, mount the framework, manage
   credentials, and run health checks.
2. **Framework source** — the canonical source for the AI-Native Delivery Framework
   files (`skills/`, `standards/`, `concepts/`, `recipes/`). A control plane mounts
   these files at `.agents/` via `gh agentic upgrade`; domain repos carry no mount.

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
│   │   ├── auth.go              ← `gh agentic auth` subcommand (login, refresh, check)
│   │   ├── check.go             ← `gh agentic check` subcommand
│   │   ├── repair.go            ← `gh agentic repair` subcommand
│   │   ├── upgrade.go           ← `gh agentic upgrade` subcommand
│   │   ├── project.go           ← `gh agentic project` subcommand tree
│   │   ├── info.go              ← `gh agentic info` subcommand
│   │   ├── framework_source.go  ← framework-source (self-mount) guard
│   │   ├── status*.go           ← `gh agentic status` subcommands + raw renderers
│   │   └── pipeline.go / pipeline_cmd.go ← `gh agentic status pipeline`
│   ├── init/                    ← init wizard logic
│   ├── mount/                   ← submodule mount logic (first-time, version switch, legacy migration)
│   │   └── templates/           ← embedded templates for generated files
│   ├── auth/                    ← credential management (login, refresh, check)
│   ├── doctor/                  ← grouped health checks (check + repair)
│   ├── project/                 ← agentic-project management + context/version resolver
│   ├── projectstatus/           ← pipeline status reporting
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
│   ├── agentic-pipeline.yml     ← pipeline stages (reusable via workflow_call; also this repo's own pipeline)
│   ├── add-issue-to-project.yml ← adds new issues to the GitHub Project
│   ├── build-and-test.yml       ← CI build and test
│   ├── sonarcloud.yml           ← SonarQube deep analysis on PRs
│   ├── release.yml              ← release-notes generation (reusable via workflow_call)
│   └── publish-release.yml      ← release binary publishing on tag push
├── RULEBOOK.md                  ← agent rulebook (active in all sessions)
├── LOCALRULES.md                ← project-specific rule overrides
├── AGENTS.md                    ← agent entrypoint (references RULEBOOK + LOCALRULES)
├── CLAUDE.md                    ← Claude Code entrypoint (references AGENTS.md)
├── go.mod
└── go.sum
```

---

## Mount model

The framework is mounted **only on the control plane** — the repo that holds the
agentic project's knowledge (`.agents`, docs, requirements, scoping). Domain repos
are pure code and carry no framework mount. This is the headline of the
control-plane-centralized model (#870): only the control plane is ever
version-pinned, upgraded, or repaired, so federation maintenance is a single-repo
operation and domain repos cannot drift.

### How it works

1. **Pinned version** — the control plane pins its framework version via its own
   `AGENTIC_FRAMEWORK_VERSION` GitHub Actions variable (read by `project.Resolve`).
   A single-topology repo is its own control plane and pins its version the same
   way. Domain repos have no framework to version.

2. **`.agents/` directory** — the mounted framework (`skills/`, `standards/`,
   `concepts/`, `recipes/`, `RULEBOOK.md`), installed on the control plane as a
   **tracked git submodule** pointing at `eddiecarpenter/gh-agentic` at the pinned
   version tag. On a runner the submodule is populated by `submodules: recursive`
   on checkout.

3. **Install / upgrade** — `gh agentic upgrade` re-points `.agents/` at a version
   tag via the underlying `git submodule` operations (`internal/mount/`). Only the
   control plane mounts, so only the control plane is upgraded. Legacy gitignored
   shallow-clone mounts are auto-migrated to the submodule by
   `gh agentic upgrade` / `gh agentic repair`.

4. **Pipeline checkout** — the agentic pipeline runs **on the control plane** (see
   Federation). Each execution phase checks the control plane out as the read-only
   knowledge root (`.agents` + docs) and clones the feature's target domain repo
   into a uniform `./project` directory; the recipe runs against `./project` with
   the framework resolved from the control-plane mount.

### On the control plane

```
control-plane-repo/
├── .agents/                 ← tracked submodule → eddiecarpenter/gh-agentic@vX.Y.Z
│   ├── RULEBOOK.md / skills/ / standards/ / concepts/ / recipes/
├── FEDERATION.yaml            ← domain-grouped manifest (domains → repos)
├── docs/                    ← SYSTEM_BRIEF, SYSTEM_ARCHITECTURE, docs/domains/<domain>/
├── CLAUDE.md / AGENTS.md / LOCALRULES.md   ← committed agent entry files
└── .github/workflows/
    └── agentic-pipeline.yml  ← the pipeline runs here, on the control plane
```

### Domain repos

A domain repo is **pure code** — no `.agents`, no docs, no pipeline workflow.
It is registered with the control plane (added to the GitHub Project and
`FEDERATION.yaml`, and given an `AGENTIC_PROJECT_ID` variable) via
`gh agentic project join` run on the control plane (#874), but nothing agentic is
installed into it.

---

## Federation

Federation centralizes on the **control plane (CP)** (#870). The control plane
holds the framework, the docs, the requirements, the scoping, the feature issues,
and the execution pipeline; domain repos hold only code. A single-topology project
is its own control plane; a federation is a control plane plus one or more
pure-code domain repos. The headline win: only the control plane carries anything
agentic, so federation maintenance collapses from N repos to one and domain repos
cannot drift.

> **Note — pre-pivot model superseded.** Federation was previously a
> *scoping-time-only* concern: Features were created in their target domain repo
> and wired as **cross-repo** sub-issues, and domain repos mounted the framework.
> Decision #869 reversed this — the pipeline runs on the control plane, feature
> issues live on the control plane targeted at a repo by a field, and domain repos
> are pure code. Migrating an existing federation is documented in
> `docs/migration-cp-centralized.md`.

### `FEDERATION.yaml` manifest

The presence of `FEDERATION.yaml` at a repo's root signals that the repo is a
federation control plane (`project.IsFederationRepo` is a stat-only check; a repo
without it is single topology). The manifest is **domain-grouped** (#871) —
domains, each with a purpose and the repos that implement it (a domain may span
one or many repos):

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

`project.ReadFederation` parses and validates it; an empty `domains:` list is
valid (a control plane with no domains registered yet). `gh agentic check`
validates the manifest stays in sync with the Project's linked repos; `gh agentic
info` lists the domains and their repos.

### How federation works

- **Create a control plane** — `gh agentic init` → federated (or `gh agentic
  project create`) establishes the GitHub Project, scaffolds an empty
  `FEDERATION.yaml` plus the federated-tier system docs (`docs/SYSTEM_BRIEF.md`,
  `docs/SYSTEM_ARCHITECTURE.md`), and mounts the framework (#875).
- **Register a domain repo** — `gh agentic project join <owner/repo> --domain
  <name>`, run on the control plane, adds the repo to `FEDERATION.yaml` under the
  named domain (lazy-creating the domain), links it to the Project, and sets its
  `AGENTIC_PROJECT_ID` — with **no framework mount** (#874).
- **Feature issues live on the control plane**, each carrying a "Target repo"
  ProjectV2 field naming the domain repo it targets, and wired as **same-repo**
  sub-issues of their requirement (#872; reverses the #825 cross-repo model).
- **The pipeline runs on the control plane** — each execution phase checks the
  control plane out as the read-only knowledge root and clones the feature's
  target repo into the uniform `./project` directory (`$AGENTIC_CP_ROOT` /
  `$AGENTIC_PROJECT_DIR`). Execution phases are read-only documentation consumers
  (#873, designed).
- **One GitHub Project** spans the whole federation; `gh agentic status` answers
  "where is everything", and `gh agentic check` / `repair` keep `FEDERATION.yaml`
  and the Project's linked repos in sync.

The two-tier knowledge plane (carried into #870) keeps system-level docs at the
control-plane root and domain docs under `docs/domains/<domain>/`.

---

## Self-relative paths

Framework files (`skills/`, `standards/`, `concepts/`) use **self-relative paths**
that resolve correctly in two contexts:

- **At the gh-agentic root** — when working directly in this repository, paths
  like `skills/dev-session.md` resolve relative to the repo root where these
  directories live.
- **When mounted as `.agents/` on a control plane** — the same paths resolve
  relative to the `.agents/` mount point. References within framework files (e.g.
  `@RULEBOOK.md`, `standards/go.md`) work because they are relative to the
  directory containing the referencing file.

This means framework files never use absolute paths or paths that assume a
specific repo structure. A reference like `concepts/delivery-philosophy.md`
works identically whether the file is at `/gh-agentic/concepts/` or at
`/my-control-plane/.agents/concepts/`.

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
  unlink — plus the context/version resolver (`project.Resolve`,
  `IsFederationRepo`, `ReadFederation`) and the shared check/repair helpers
  that the cobra commands compose.
- `internal/ui/` owns the shared colour palette and lipgloss styles. No other
  package defines styles inline.
- `internal/fsutil/` provides filesystem utilities shared across packages.
- `internal/testutil/` provides shared test helpers.

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
