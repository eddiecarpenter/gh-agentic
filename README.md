# gh-agentic

A GitHub CLI extension that bootstraps and manages agentic software delivery environments.

## Install

```bash
gh extension install eddiecarpenter/gh-agentic
```

## Commands

```bash
gh agentic bootstrap    # Bootstrap a new agentic environment
gh agentic inception    # Register a new domain or tool repo
gh agentic sync         # Sync base/ from the upstream template
```

## Prerequisites

- [git](https://git-scm.com)
- [GitHub CLI](https://cli.github.com) — authenticated (`gh auth login`)
- [Goose](https://github.com/block/goose)

Claude Code is recommended as the Goose provider but not required.

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

See `docs/PROJECT_BRIEF.md` for full design documentation.
