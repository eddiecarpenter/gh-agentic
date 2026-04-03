# ARCHITECTURE.md — gh-agentic

## Overview

`gh-agentic` is a GitHub CLI extension that bootstraps and manages agentic
development environments. It is a single compiled Go binary distributed via
`gh extension install`.

---

## Package structure

```
gh-agentic/
├── cmd/
│   └── gh-agentic/
│       └── main.go          ← entrypoint, wires cobra root command
├── internal/
│   ├── cli/
│   │   ├── root.go          ← root command, version flag
│   │   ├── bootstrap.go     ← `gh agentic bootstrap` subcommand
│   │   ├── inception.go     ← `gh agentic inception` subcommand
│   │   └── sync.go          ← `gh agentic sync` subcommand
│   ├── bootstrap/
│   │   ├── preflight.go     ← environment checks, tool installation
│   │   ├── form.go          ← huh TUI form, BootstrapConfig struct
│   │   ├── steps.go         ← steps 3-9 as individual Go functions
│   │   └── runner.go        ← orchestrates steps, spinner per step
│   ├── inception/
│   │   ├── form.go          ← inception form
│   │   └── steps.go         ← inception execution steps
│   ├── sync/
│   │   └── sync.go          ← template sync logic
│   └── ui/
│       └── styles.go        ← lipgloss styles, GitHub colour palette
├── docs/
│   ├── PROJECT_BRIEF.md
│   ├── ARCHITECTURE.md      ← this file
│   └── TUI_DESIGN.md
├── prototype.sh             ← gum-based UX prototype (reference only)
├── go.mod
└── go.sum
```

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
- `internal/bootstrap/`, `internal/inception/`, `internal/sync/` own their
  respective business logic. They have no knowledge of cobra.
- `internal/ui/` owns the shared colour palette and lipgloss styles. No other
  package defines styles inline.

---

## Key dependencies

| Library | Why |
|---|---|
| `github.com/cli/go-gh/v2` | GitHub API clients pre-authenticated from `gh` credentials. Eliminates all token handling. |
| `github.com/charmbracelet/huh` | Declarative TUI forms — topology, owner, project details. See `docs/TUI_DESIGN.md`. |
| `github.com/charmbracelet/lipgloss` | Terminal styling — GitHub colour palette, borders, summary boxes. |
| `github.com/charmbracelet/bubbles` | Spinner component shown while each execution step runs. |
| `github.com/spf13/cobra` | CLI command routing — `gh agentic bootstrap`, `inception`, `sync`. |

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

---

## Goose launch

At the end of `gh agentic bootstrap`, the user is offered the option to
launch Goose CLI in the new repo:

```bash
cd <clone-path> && goose session --recipe requirements
```

Desktop launch is explicitly not supported — opening a desktop AI client with
a new workspace path while it is already running kills the current active
session (confirmed on macOS). CLI is predictable and works on all platforms
including SSH sessions. See `docs/TUI_DESIGN.md` — Design Decisions.
