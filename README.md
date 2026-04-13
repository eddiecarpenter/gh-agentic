# gh-agentic

[![Build](https://github.com/eddiecarpenter/gh-agentic/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/eddiecarpenter/gh-agentic/actions/workflows/build-and-test.yml)
[![Latest Release](https://img.shields.io/github/v/release/eddiecarpenter/gh-agentic)](https://github.com/eddiecarpenter/gh-agentic/releases/latest)
[![Quality Gate](https://sonarcloud.io/api/project_badges/measure?project=eddiecarpenter_gh-agentic&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=eddiecarpenter_gh-agentic)
[![Go Version](https://img.shields.io/github/go-mod/go-version/eddiecarpenter/gh-agentic)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A GitHub CLI extension that bootstraps and manages agentic software delivery environments.
It replaces manual shell scripts with deterministic Go commands — creating repos, scaffolding
projects, configuring GitHub, mounting the AI-Native Delivery Framework, and managing
credentials — so AI agents can focus on the work that actually requires reasoning.

## Install

```bash
gh extension install eddiecarpenter/gh-agentic
```

## Prerequisites

- [git](https://git-scm.com)
- [GitHub CLI](https://cli.github.com) — authenticated (`gh auth login`)
- [Claude Code](https://claude.ai/code) — required for agent sessions and credential management

### Platform requirements for credential management

- **macOS** — credentials are stored in the macOS keychain (`"Claude Code-credentials"`)
- **Linux** — credentials are stored at `~/.claude/.credentials.json`

## Getting started (v2)

### 1. Clone or create a repo

```bash
git clone git@github.com:<owner>/<repo>.git
cd <repo>
```

### 2. Initialise the agentic environment

```bash
gh agentic -v2 init
```

The init wizard detects the current repo, collects configuration (stack, framework
version), mounts the framework, generates agent entry files, and configures GitHub
secrets and variables.

### 3. Verify the setup

```bash
gh agentic -v2 doctor-v2
```

All checks should pass. If any fail, follow the remediation commands in the output.

### 4. Start working

Open the repo in your AI agent and begin a Requirements Session. The agent reads
`CLAUDE.md` and `AGENTS.md` to load the framework rules and playbooks.

## Commands

### v2 commands (current)

| Command | Description |
|---|---|
| `gh agentic -v2 init` | Interactive wizard to initialise a new agentic environment |
| `gh agentic -v2 mount [version]` | Mount the AI-Native Delivery Framework at `.ai/` |
| `gh agentic -v2 auth login` | Force Claude Code login and push credentials to repo secret |
| `gh agentic -v2 auth refresh` | Push current local credentials to repo secret |
| `gh agentic -v2 auth check` | Verify credentials are present and not expired |
| `gh agentic -v2 doctor-v2` | Health check with grouped output |

### v1 commands (deprecated)

> **Legacy notice:** The following v1 commands are deprecated and will be removed
> in a future release. Use the v2 equivalents above.

| Command | Replacement |
|---|---|
| `gh agentic bootstrap` | `gh agentic -v2 init` |
| `gh agentic inception` | `gh agentic -v2 init` |
| `gh agentic sync` | `gh agentic -v2 mount` |
| `gh agentic doctor` | `gh agentic -v2 doctor-v2` |

## Mount

The mount command downloads the AI-Native Delivery Framework and installs it at
`.ai/` in the current repo. The `.ai/` directory is gitignored — it is populated
on demand and not committed.

```bash
gh agentic -v2 mount v2.0.0    # first-time mount at a specific version
gh agentic -v2 mount            # remount at current .ai-version
gh agentic -v2 mount v2.1.0    # switch to a new version (prompts for confirmation)
```

The pinned version is stored in `.ai-version` (committed to the repo).

## Auth

The auth command manages Claude Code credentials for CI runners.

```bash
gh agentic -v2 auth login      # force login and push credentials
gh agentic -v2 auth refresh    # push current local credentials to repo secret
gh agentic -v2 auth check      # verify credentials are present and not expired
```

## Upgrade

```bash
gh extension upgrade agentic
```

## Development

```bash
git clone git@github.com:eddiecarpenter/gh-agentic.git
cd gh-agentic
go build ./...
go test ./...
```

See [`docs/PROJECT_BRIEF.md`](docs/PROJECT_BRIEF.md) for full design documentation
and [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the package structure and
mount model.

## License

[MIT](LICENSE)
