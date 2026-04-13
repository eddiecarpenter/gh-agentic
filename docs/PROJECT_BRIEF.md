# PROJECT_BRIEF.md — gh-agentic

## What it is

A GitHub CLI extension that bootstraps and manages agentic software delivery
environments. Installed and updated via `gh extension`.

```bash
gh extension install eddiecarpenter/gh-agentic
gh extension upgrade agentic
```

In v2, `gh-agentic` is the **single source of truth** for both the CLI tooling and
the AI-Native Delivery Framework files (`skills/`, `standards/`, `concepts/`,
`recipes/`). Domain repos mount the framework at `.ai/` using `gh agentic -v2 mount`,
rather than copying from a separate template repository.

## Why it exists

The original `bootstrap.sh` handed off to an AI agent (Goose or Claude) to execute
deterministic steps (create repo, scaffold project, create labels, etc.). This was
unreliable — the agent could hallucinate values (e.g. wrong Go version), produce
inconsistent output, and was hard to test.

The extension moves all deterministic steps into Go code, leaving the AI agent for
phases that genuinely require reasoning (requirements, design, development).

## Commands

### v2 commands (current)

| Command | Description |
|---|---|
| `gh agentic -v2 init` | Interactive wizard to initialise a new agentic environment (replaces bootstrap + inception) |
| `gh agentic -v2 mount [version]` | Mount the AI-Native Delivery Framework at `.ai/` (replaces sync) |
| `gh agentic -v2 auth login` | Force Claude Code login and push credentials to repo secret |
| `gh agentic -v2 auth refresh` | Push current local credentials to repo secret |
| `gh agentic -v2 auth check` | Verify credentials are present and not expired |
| `gh agentic -v2 doctor-v2` | Health check with grouped output |

### v1 commands (deprecated)

| Command | Replacement | Status |
|---|---|---|
| `gh agentic bootstrap` | `gh agentic -v2 init` | Deprecated — will be removed |
| `gh agentic inception` | `gh agentic -v2 init` | Deprecated — will be removed |
| `gh agentic sync` | `gh agentic -v2 mount` | Deprecated — will be removed |

## Separation of concerns

- **`gh-agentic`** — both infrastructure tooling and framework source. The CLI
  creates repos, scaffolds projects, configures GitHub, mounts the framework,
  and manages credentials. The same repository holds the framework files
  (`skills/`, `standards/`, `concepts/`, `recipes/`) that domain repos consume.
- **Domain repos** — mount the framework via `.ai/` and run agent sessions.
  Framework files are gitignored and populated by `gh agentic -v2 mount`.
- **AI agent** — runs inside domain repos for Phases 1+. Invoked by the human
  or by GitHub Actions workflows, not by the extension itself.

## v2 Init flow (`gh agentic -v2 init`)

The init command is an interactive wizard that replaces both `bootstrap` and
`inception` from v1.

### Preflight checks

| Check | Level | Action if missing |
|---|---|---|
| `git` | Required | Hard stop with install URL |
| `gh` | Required | Hard stop with install URL |
| `gh auth` | Required | Hard stop — run `gh auth login` |
| `claude` | Required | Hard stop — Claude Code is required for v2 |

### Init wizard

The wizard detects the current repository and collects configuration:

1. **Stack** — Go, Java/Quarkus, Java/Spring Boot, TypeScript/Node.js, Python, Rust, or Other
2. **Framework version** — which version of the framework to mount
3. **Configuration** — generates `CLAUDE.md`, `AGENTS.md`, `LOCALRULES.md`, `.ai-version`

### Steps

| Step | Action |
|---|---|
| 1 | Detect current repo (owner, name, remote) |
| 2 | Collect configuration via interactive form |
| 3 | Mount framework at `.ai/` via tarball download |
| 4 | Generate agent entry files (`CLAUDE.md`, `AGENTS.md`, `LOCALRULES.md`) |
| 5 | Configure GitHub repo variables and secrets |
| 6 | Print summary — next steps for starting a session |

### Handoff

The extension prints instructions for the human to open the repo in their
agent and start a Requirements Session (Phase 1). No agent is launched by the
extension itself.

## Technology

| Library | Purpose |
|---|---|
| `github.com/cli/go-gh/v2` | Auth (inherited from gh), GitHub API clients |
| `github.com/charmbracelet/huh` | Interactive form (init wizard) |
| `github.com/charmbracelet/lipgloss` | Styling — banner, summary box, colours |
| `github.com/charmbracelet/bubbles` | Spinner per step |
| `github.com/spf13/cobra` | CLI command routing with `-v2` persistent flag |

## Distribution

Binaries are distributed via GitHub Releases using the
`gh-extension-precompile` GitHub Action.

| Platform | Install |
|---|---|
| Mac / Linux | `gh extension install eddiecarpenter/gh-agentic` |
| Windows | `gh extension install eddiecarpenter/gh-agentic` |

Upgrade: `gh extension upgrade agentic`

## Scope boundaries

**In scope:**
- `gh agentic -v2 init` — environment initialisation wizard
- `gh agentic -v2 mount` — framework mount and version management
- `gh agentic -v2 auth` — credential management (login, refresh, check)
- `gh agentic -v2 doctor-v2` — health checks with grouped output

**Out of scope:**
- Running or orchestrating AI agent sessions
- Any development workflow (requirements, design, dev) — that is the agent's job
- Windows-specific package manager (Scoop) — added later via goreleaser
