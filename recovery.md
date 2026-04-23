# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #622                               |
| Branch              | feature/622-github-app-identity    |
| Last commit         | 4bb5a99                            |
| Total tasks         | 5                                  |
| Last updated        | 2026-04-24T00:00:00Z               |

## Completed Tasks

- #627 — Document GitHub App setup, installation, and private-key rotation

## Remaining Tasks

- #628 — Swap agentic-pipeline.yml to App token, drop AGENT_USER, fix mount step (submodule regression from PR #620)
- #629 — Swap remaining workflows + setup-claude-auth, fix mount step in release.yml
- #630 — Migrate Issue Session trigger from assignee to label `assigned-to-agent`
- #631 — Add CI test rejecting GOOSE_AGENT_PAT / AGENT_USER

## Recovery context

This Feature is being driven interactively from a recovery worktree because the workflow's `Mount AI framework` step has a preexisting regression (does not handle the `.ai` / `.agentic-tools` submodules introduced by PR #620). Pipeline-wide trigger freeze is in effect until #622 ships. See the comment thread on #622 for full context.
