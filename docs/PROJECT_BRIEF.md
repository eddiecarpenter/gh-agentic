# PROJECT_BRIEF.md — gh-agentic

## What it is

A GitHub CLI extension that bootstraps and manages agentic software delivery
environments. Installed and updated via `gh extension`.

```bash
gh extension install eddiecarpenter/gh-agentic
gh extension upgrade agentic
```

`gh-agentic` is the **single source of truth** for both the CLI tooling and
the AI-Native Delivery Framework files (`skills/`, `standards/`, `concepts/`,
`recipes/`). Domain repos mount the framework at `.agents/` using `gh agentic mount`.

## Why it exists

The original `bootstrap.sh` handed off to an AI agent (Goose or Claude) to execute
deterministic steps (create repo, scaffold project, create labels, etc.). This was
unreliable — the agent could hallucinate values (e.g. wrong Go version), produce
inconsistent output, and was hard to test.

The extension moves all deterministic steps into Go code, leaving the AI agent for
phases that genuinely require reasoning (requirements, design, development).

## Commands

### Commands

| Command | Description |
|---|---|
| `gh agentic init` | Interactive wizard to initialise a new agentic environment |
| `gh agentic check` | Verify project membership and pipeline readiness |
| `gh agentic repair` | Auto-fix issues reported by `check` |
| `gh agentic mount [version]` | Mount the AI-Native Delivery Framework at `.agents/` |
| `gh agentic upgrade` | Change the framework version for the whole federation (control plane only) |
| `gh agentic project` | Manage ongoing project membership — create, join, switch, unlink |
| `gh agentic info` | Show the current state of this repo's agentic setup |
| `gh agentic auth login` | Force Claude Code login and push credentials to repo secret |
| `gh agentic auth refresh` | Push current local credentials to repo secret |
| `gh agentic auth check` | Verify credentials are present and not expired |
| `gh agentic status` | Show pipeline state across requirements and features |
| `gh agentic status pipeline` | Render requirements and features as a side-by-side pipeline view |

## Separation of concerns

- **`gh-agentic`** — both infrastructure tooling and framework source. The CLI
  creates repos, scaffolds projects, configures GitHub, mounts the framework,
  and manages credentials. The same repository holds the framework files
  (`skills/`, `standards/`, `concepts/`, `recipes/`) that domain repos consume.
- **Domain repos** — mount the framework via `.agents/` and run agent sessions.
  Framework files are gitignored and populated by `gh agentic mount`.
- **AI agent** — runs inside domain repos for Phases 1+. Invoked by the human
  or by GitHub Actions workflows, not by the extension itself.

## Init flow (`gh agentic init`)

The init command is an interactive wizard.

### Preflight checks

| Check | Level | Action if missing |
|---|---|---|
| `git` | Required | Hard stop with install URL |
| `gh` | Required | Hard stop with install URL |
| `gh auth` | Required | Hard stop — run `gh auth login` |
| `claude` | Required | Hard stop — Claude Code is required |

### Init wizard

The wizard detects the current repository and collects configuration:

1. **Stack** — Go, Java/Quarkus, Java/Spring Boot, TypeScript/Node.js, Python, Rust, or Other
2. **Framework version** — which version of the framework to mount
3. **Configuration** — generates `CLAUDE.md`, `AGENTS.md`, and `LOCALRULES.md`; pins
   the framework version on the control-plane repo's `AGENTIC_FRAMEWORK_VERSION`
   variable (read through `project.Resolve`).

### Steps

| Step | Action |
|---|---|
| 1 | Detect current repo (owner, name, remote) |
| 2 | Collect configuration via interactive form |
| 3 | Mount framework at `.agents/` via tarball download |
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
| `github.com/spf13/cobra` | CLI command routing |

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
- `gh agentic init` — environment initialisation wizard
- `gh agentic mount` — framework mount and version management
- `gh agentic auth` — credential management (login, refresh, check)
- `gh agentic check` / `gh agentic repair` — health checks with grouped output and auto-repair

**Out of scope:**
- Running or orchestrating AI agent sessions
- Any development workflow (requirements, design, dev) — that is the agent's job
- Windows-specific package manager (Scoop) — added later via goreleaser
