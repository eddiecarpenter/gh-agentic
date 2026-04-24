# Recovery State

| Field               | Value                              |
|---------------------|------------------------------------|
| Feature issue       | #622                               |
| Branch              | feature/622-github-app-identity    |
| Last commit         | ba9e8d7                            |
| Total tasks         | 5                                  |
| Last updated        | 2026-04-24T00:00:00Z               |

## Completed Tasks

- #627 — Document GitHub App setup, installation, and private-key rotation
- #628 — Swap agentic-pipeline.yml to App token, drop AGENT_USER, fix mount step
- #629 — Swap remaining workflows + setup-claude-auth, fix mount step in release.yml
- #630 — Migrate Issue Session trigger from assignee to label `assigned-to-agent`

## Remaining Tasks

- #631 — Add CI test rejecting GOOSE_AGENT_PAT / AGENT_USER

## Recovery context

Interactive recovery — the workflow's `Mount AI framework` step had a preexisting regression (PR #620 submodule conversion). Pipeline-wide trigger freeze in effect until #622 ships. See the comment thread on #622.
