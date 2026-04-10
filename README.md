# gh-agentic

[![Go Version](https://img.shields.io/github/go-mod/go-version/eddiecarpenter/gh-agentic)](go.mod)
[![Build](https://github.com/eddiecarpenter/gh-agentic/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/eddiecarpenter/gh-agentic/actions/workflows/build-and-test.yml)
[![Latest Release](https://img.shields.io/github/v/release/eddiecarpenter/gh-agentic)](https://github.com/eddiecarpenter/gh-agentic/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A GitHub CLI extension that bootstraps and manages agentic software delivery environments.
It replaces manual shell scripts with deterministic Go commands — creating repos, scaffolding
projects, configuring GitHub, and keeping template files in sync — so AI agents can focus
on the work that actually requires reasoning.

## Install

```bash
gh extension install eddiecarpenter/gh-agentic
```

## Commands

| Command | Description |
|---|---|
| `gh agentic bootstrap` | Bootstrap a new agentic environment (Phase 0a) |
| `gh agentic inception` | Register a new domain or tool repo (Phase 0b) |
| `gh agentic sync` | Sync `.ai/`, workflows, and recipes from the upstream template |
| `gh agentic doctor` | Check the local repo for missing or misconfigured agentic files |

## Prerequisites

- [git](https://git-scm.com)
- [GitHub CLI](https://cli.github.com) — authenticated (`gh auth login`)
- [Goose](https://github.com/block/goose)

Claude Code is recommended as the Goose provider but not required.

## Usage

### Bootstrap a new project

Run `gh agentic bootstrap` from any directory. You will be prompted for:

- **Topology** — Embedded (single repo) or Organisation (separate control plane)
- **Owner** — your personal account or an organisation
- **Project name** and **description**
- **Stack** — Go, Java, TypeScript, Python, Rust, or Other

The command creates the GitHub repo, scaffolds the project structure, configures branch
protection and labels, and prints next steps for starting a Requirements Session with
your AI agent.

### Sync the template

```bash
gh agentic sync                     # sync to the latest release
gh agentic sync --release v1.5.0    # sync to a specific release
gh agentic sync --list              # list available releases
gh agentic sync --force             # re-sync even if already up to date
```

### Check repo health

```bash
gh agentic doctor          # report missing or misconfigured files
gh agentic doctor --repair # interactively repair detected issues
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

See [`docs/PROJECT_BRIEF.md`](docs/PROJECT_BRIEF.md) for full design documentation.

## License

[MIT](LICENSE)
