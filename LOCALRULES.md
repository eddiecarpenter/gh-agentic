# LOCALRULES.md — Local Overrides

This file contains project-specific rules and overrides that extend or
supersede the global protocol defined in `.agents/RULEBOOK.md`.

This file is never overwritten by a template sync.

---

## Template Source

Template: eddiecarpenter/ai-native-delivery

## Project

- **Name:** gh-agentic
- **Topology:** single
- **Stack:** Go
- **Description:** GitHub CLI extension for the agentic development framework

## Repo

- **GitHub:** https://github.com/eddiecarpenter/gh-agentic
- **Module:** github.com/eddiecarpenter/gh-agentic

## Commands

| Command | Description |
|---|---|
| `gh agentic init` | Interactive wizard to initialise a new agentic environment |
| `gh agentic check` | Verify project membership and pipeline readiness |
| `gh agentic repair` | Auto-fix issues reported by `check` |
| `gh agentic mount [version]` | Mount the AI-Native Delivery Framework at `.agents/` |
| `gh agentic upgrade` | Change the framework version (control plane only) |
| `gh agentic project` | Manage project membership — create, join, switch, unlink |
| `gh agentic info` | Show the current state of this repo's agentic setup |
| `gh agentic auth` | Manage Claude credentials (login, refresh, check) |
| `gh agentic status` | Show pipeline state across requirements and features |
| `gh agentic status pipeline` | Render requirements and features as a side-by-side pipeline view |

## Key Libraries

| Library | Purpose |
|---|---|
| `github.com/cli/go-gh/v2` | GitHub API, auth inherited from gh |
| `github.com/charmbracelet/huh` | Interactive forms |
| `github.com/charmbracelet/lipgloss` | Styling |
| `github.com/charmbracelet/bubbles` | Spinner components |
| `github.com/spf13/cobra` | CLI command routing |

## Reference Implementation

**Always check `NewOpenBSS/charging-domain` before building anything new.**

This repo is the reference implementation of the agentic pipeline. If something
is missing, broken, or needs a workflow/recipe/pattern — look there first.
It is known to work end-to-end with the organisation topology.

Do not reinvent what already exists in charging-domain. Adapt it.

## Workflow File Changes — Interactive Only

**Any feature whose implementation touches `.github/workflows/agentic-pipeline.yml` (or any
other workflow file) MUST be implemented in an interactive session, not via the headless
dev-session pipeline.**

Reason: GitHub blocks any token — including fine-grained PATs and classic PATs with `workflow`
scope — from pushing workflow file changes when the push originates from a GitHub Actions
runner via `github.token`. Routing workflow changes through the pipeline causes repeated push
failures that require manual intervention. Implement workflow changes interactively (Claude Code
desktop or direct editor), commit and push with your own credentials, and open the PR manually.

This applies to all jobs in `agentic-pipeline.yml`: adding new pipeline stages, modifying
trigger conditions, adding steps, and updating job-level env vars.

## Notes

- This repo was created manually (before the bootstrap tool existed) — bootstrap.sh was not used
- `.agents/` was copied from `eddiecarpenter/ai-native-delivery` at initial setup
- GitHub Actions workflows were adapted from `NewOpenBSS/charging-domain` (known working)
- **Self-hosted runner must be registered per repo for personal-space repos** — organisation
  runners are shared across all org repos, but personal repos each need their own registration.
  Register at: `github.com/eddiecarpenter/gh-agentic/settings/actions/runners`

## Tool / Skill Sync

When the `gh agentic` CLI surface changes (new command, removed command, new/removed/renamed flag, changed `--raw` output shape), `skills/gh-agentic/SKILL.md` must be updated in the same PR. CI enforces this via `TestGhAgenticToolSkillCoversCLI` — the build fails if the skill is out of sync with the CLI.

## Release Version Sync

Whenever a new version of `gh-agentic` is released, the `AGENTIC_FRAMEWORK_VERSION` repo variable on `eddiecarpenter/gh-agentic` must be updated to match the new release tag. The CI pipeline pins both the mounted framework and the installed `gh-agentic` extension to this variable — if it drifts from the latest release, CI runs against a stale framework and a stale CLI.

Update command:

```
gh variable set AGENTIC_FRAMEWORK_VERSION --repo eddiecarpenter/gh-agentic --body "<new-version>"
```

This applies to this repo only because `gh-agentic` is its own framework source — a circular dependency unique to the control-plane-of-itself arrangement. Other domain repos get the version from the control plane via `gh agentic mount` and do not need this step.

## Migration

Domain repos moving onto the current framework identity follow these guides. Each document is the single source of truth — do not copy their contents into a repo's own `LOCALRULES.md`, reference them:

- [`concepts/migration-to-github-app.md`](concepts/migration-to-github-app.md) — required cutover doc for moving off the legacy `goose-agent` PAT identity onto the agentic GitHub App.
- [`docs/migration-agent-vars-rename.md`](docs/migration-agent-vars-rename.md) — sibling migration covering the `GOOSE_PROVIDER` / `GOOSE_MODEL` → `AGENT_PROVIDER` / `AGENT_MODEL` variable rename; typically performed alongside the App cutover (both land under parent #621).
