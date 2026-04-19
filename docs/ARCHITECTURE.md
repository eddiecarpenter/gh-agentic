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
   these files at `.ai/` via `gh agentic mount`.

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
│   │   ├── mount.go             ← `gh agentic mount` subcommand
│   │   ├── auth.go              ← `gh agentic auth` subcommand (login, refresh, check)
│   │   ├── check.go             ← `gh agentic check` subcommand
│   │   ├── repair.go            ← `gh agentic repair` subcommand
│   │   ├── upgrade.go           ← `gh agentic upgrade` subcommand
│   │   ├── project.go           ← `gh agentic project` subcommand tree
│   │   ├── info.go              ← `gh agentic info` subcommand
│   │   ├── status.go            ← `gh agentic status` subcommand
│   │   └── kanban.go            ← `gh agentic kanban` subcommand
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
│   ├── ARCHITECTURE.md          ← this file
│   └── TUI_DESIGN.md            ← init wizard UX reference
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

1. **`.ai-version`** — A plain-text file at the repo root containing the pinned
   framework version tag (e.g. `v2.0.0`). This file is committed to the repo.

2. **`.ai/` directory** — The mounted framework. This directory is **gitignored**
   and populated on demand by `gh agentic mount`. It is not committed.

3. **Fetch mechanism** — `gh agentic mount` downloads the framework as a
   tarball from the `eddiecarpenter/gh-agentic` release at the pinned version
   using `git clone --depth 1`. It extracts framework files (`skills/`,
   `standards/`, `concepts/`, `recipes/`, `RULEBOOK.md`) into `.ai/`.

4. **Mount flows** — The mount command supports three flows:
   - **First-time** (`mount <version>` with no `.ai-version`): downloads the
     framework, generates `CLAUDE.md`, `AGENTS.md`, and wrapper workflows, writes
     `.ai-version`.
   - **Remount** (`mount` with no args): re-downloads at the current `.ai-version`.
     Used after a fresh clone or to repair a corrupted `.ai/`.
   - **Version switch** (`mount <new-version>` with existing `.ai-version`):
     prompts for confirmation, updates `.ai-version`, and remounts.

5. **Reusable workflows** — Domain repos invoke the agentic pipeline via thin
   wrapper workflows in `.github/workflows/` that call reusable workflows
   defined in `eddiecarpenter/gh-agentic`. This avoids duplicating workflow
   definitions across repos.

### In domain repos

```
my-domain-repo/
├── .ai-version          ← committed — pins framework version
├── .ai/                 ← gitignored — mounted framework files
│   ├── RULEBOOK.md
│   ├── skills/
│   ├── standards/
│   ├── concepts/
│   └── recipes/
├── CLAUDE.md            ← committed — references .ai/AGENTS.md
├── AGENTS.md            ← committed — references RULEBOOK + LOCALRULES
├── LOCALRULES.md        ← committed — project-specific overrides
└── .github/workflows/
    └── agentic-pipeline.yml  ← wrapper calling reusable workflow
```

---

## Self-relative paths

Framework files (`skills/`, `standards/`, `concepts/`) use **self-relative paths**
that resolve correctly in two contexts:

- **At the gh-agentic root** — when working directly in this repository, paths
  like `skills/dev-session.md` resolve relative to the repo root where these
  directories live.
- **When mounted as `.ai/` in domain repos** — the same paths resolve relative
  to the `.ai/` mount point. References within framework files (e.g.
  `@RULEBOOK.md`, `standards/go.md`) work because they are relative to the
  directory containing the referencing file.

This means framework files never use absolute paths or paths that assume a
specific repo structure. A reference like `concepts/delivery-philosophy.md`
works identically whether the file is at `/gh-agentic/concepts/` or at
`/my-repo/.ai/concepts/`.

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
| `gh agentic mount [version]` | Mount the AI-Native Delivery Framework at `.ai/` |
| `gh agentic upgrade` | Change the framework version for the whole federation (control plane only) |
| `gh agentic project` | Manage ongoing project membership — create, join, switch, unlink |
| `gh agentic info` | Show the current state of this repo's agentic setup |
| `gh agentic auth` | Manage Claude credentials used by the agent pipeline (login, refresh, check) |
| `gh agentic status` | Show pipeline state across requirements and features |
| `gh agentic kanban` | Render requirements and features as a kanban view |

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
