# LOCALRULES.md — Local Overrides

This file contains project-specific rules and overrides that extend or
supersede the global protocol defined in `.ai/RULEBOOK.md`.

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
| `gh agentic mount [version]` | Mount the AI-Native Delivery Framework at `.ai/` |
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

## Notes

- This repo was created manually (before the bootstrap tool existed) — bootstrap.sh was not used
- `.ai/` was copied from `eddiecarpenter/ai-native-delivery` at initial setup
- GitHub Actions workflows were adapted from `NewOpenBSS/charging-domain` (known working)
- **Self-hosted runner must be registered per repo for personal-space repos** — organisation
  runners are shared across all org repos, but personal repos each need their own registration.
  Register at: `github.com/eddiecarpenter/gh-agentic/settings/actions/runners`
