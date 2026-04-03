# PROJECT_BRIEF.md — gh-agentic

## What it is

A GitHub CLI extension that replaces `bootstrap.sh` and manages the lifecycle of
agentic development environments. Installed and updated via `gh extension`.

```bash
gh extension install eddiecarpenter/gh-agentic
gh extension upgrade agentic
```

## Why it exists

The original `bootstrap.sh` handed off to an AI agent (Goose or Claude) to execute
deterministic steps (create repo, scaffold project, create labels, etc.). This was
unreliable — the agent could hallucinate values (e.g. wrong Go version), produce
inconsistent output, and was hard to test.

The extension moves all deterministic steps into Go code, leaving the AI agent for
phases that genuinely require reasoning (requirements, design, development).

## Commands

| Command | Phase | Description |
|---|---|---|
| `gh agentic bootstrap` | 0a | Create and configure a new agentic environment |
| `gh agentic inception` | 0b | Register a new domain or tool repo |
| `gh agentic sync` | — | Sync `base/` from the upstream template |

## Separation of concerns

- **`gh-agentic`** — infrastructure tooling. Creates repos, scaffolds projects,
  configures GitHub. Deterministic. No AI involved.
- **`agentic-development`** — the template repo. Holds `base/AGENTS.md`, standards
  files, and reusable workflow definitions.
- **AI agent** — runs inside project repos for Phases 1+. Invoked by the human,
  not by the extension.

## Bootstrap flow (`gh agentic bootstrap`)

### Preflight checks

| Check | Level | Action if missing |
|---|---|---|
| `git` | Required | Hard stop with install URL |
| `gh` | Required | Hard stop with install URL |
| `gh auth` | Required | Hard stop — run `gh auth login` |
| `goose` | Required | Offer to install via `curl -fsSL https://github.com/block/goose/releases/latest/download/install.sh | bash` |
| `claude` | Recommended | Offer to install via `curl -fsSL https://claude.ai/install.sh | bash` |

### Form (huh)

Collected interactively using the `huh` TUI library:

1. **Topology** — Embedded (single repo) or Organisation (separate control plane)
2. **Owner** — Personal account or organisation (populated from `gh` API)
3. **Project name**
4. **Description**
5. **Stack** — Go, Java/Quarkus, Java/Spring Boot, TypeScript/Node.js, Python, Rust, Other
6. **Antora** — Y/N — whether an Antora documentation site is needed

### Steps (deterministic Go functions)

| Step | Action |
|---|---|
| 3 | `gh repo create <owner>/<repo> --template eddiecarpenter/agentic-development` + clone |
| 4 | Remove `bootstrap.sh` and `bootstrap.sh.md5` from the cloned repo |
| 5 | Scaffold project structure per stack (see `base/standards/<stack>.md`) |
| 6 | Apply branch protection + create standard labels |
| 7 | Populate `REPOS.md`, `AGENTS.local.md`, `README.md`; scaffold Antora if requested |
| 8 | `gh project create` |
| 9 | Print summary — repo URL, project URL, local clone path, next steps |

Each step shows a spinner while running and a ✔ on completion.

### Handoff

The extension prints instructions for the human to open the new repo in their
agent and start a Requirements Session (Phase 1). No agent is launched by the
extension itself.

## Technology

| Library | Purpose |
|---|---|
| `github.com/cli/go-gh/v2` | Auth (inherited from gh), GitHub API clients |
| `github.com/charmbracelet/huh` | Interactive form |
| `github.com/charmbracelet/lipgloss` | Styling — banner, summary box, colours |
| `github.com/charmbracelet/bubbles` | Spinner per step |

## Distribution

Mac-first to start. Binaries distributed via GitHub Releases using the
`gh-extension-precompile` GitHub Action.

| Platform | Install |
|---|---|
| Mac / Linux | `gh extension install eddiecarpenter/gh-agentic` |
| Windows | `gh extension install eddiecarpenter/gh-agentic` |

Upgrade: `gh extension upgrade agentic`

## Scope boundaries

**In scope:**
- `gh agentic bootstrap` (Phase 0a) — full implementation
- `gh agentic inception` (Phase 0b) — follow-on
- `gh agentic sync` (template sync) — follow-on

**Out of scope:**
- Running or orchestrating AI agent sessions
- Any development workflow (requirements, design, dev) — that is the agent's job
- Windows-specific package manager (Scoop) — added later via goreleaser
