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

## Getting started

### 1. Clone or create a repo

```bash
git clone git@github.com:<owner>/<repo>.git
cd <repo>
```

### 2. Initialise the agentic environment

```bash
gh agentic init
```

The init wizard detects the current repo, collects configuration (stack, framework
version), mounts the framework, generates agent entry files, and configures GitHub
secrets and variables.

### 3. Verify the setup

```bash
gh agentic check
```

All checks should pass. If any fail, run `gh agentic repair` to auto-fix the ones
that can be fixed, and follow the remediation commands for the rest.

### 4. Start working

Open the repo in your AI agent and begin a Requirements Session. The agent reads
`CLAUDE.md` and `AGENTS.md` to load the framework rules and playbooks.

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
| `gh agentic auth login` | Force Claude Code login and push credentials to repo secret |
| `gh agentic auth refresh` | Push current local credentials to repo secret |
| `gh agentic auth check` | Verify credentials are present and not expired |
| `gh agentic status` | Show pipeline state across requirements and features |
| `gh agentic status pipeline` | Render requirements and features as a side-by-side pipeline view |

## Mount

The mount command downloads the AI-Native Delivery Framework and installs it at
`.ai/` in the current repo. The `.ai/` directory is gitignored — it is populated
on demand and not committed.

```bash
gh agentic mount v2.0.0    # first-time mount at a specific version
gh agentic mount            # remount at current .ai-version
gh agentic mount v2.1.0    # switch to a new version (prompts for confirmation)
```

The pinned version is stored in `.ai-version` (committed to the repo).

## Auth

The auth command manages Claude Code credentials for CI runners.

```bash
gh agentic auth login      # force login and push credentials
gh agentic auth refresh    # push current local credentials to repo secret
gh agentic auth check      # verify credentials are present and not expired
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
