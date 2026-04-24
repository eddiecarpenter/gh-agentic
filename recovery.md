# Recovery State

| Field               | Value                                     |
|---------------------|-------------------------------------------|
| Feature issue       | #625                                      |
| Branch              | feature/625-cli-app-install-helper        |
| Last commit         | 5666e9b                                   |
| Total tasks         | 5                                         |
| Last updated        | 2026-04-24T04:35:00Z                      |

## Completed Tasks

### #641 — Add GitHub App installation detection API
- **Implemented:** New `internal/githubapp` package with `Checker` type, `CheckRepoInstallation` and `CheckOrgInstallation` methods, and an injectable `RESTClient` interface. 200/404/wrong-slug/5xx/network-error paths all unit-tested via fake client.
- **Files changed:** `internal/githubapp/installation.go`, `internal/githubapp/installation_test.go`
- **Decisions:** Used `DoWithContext` (not `Get`) to propagate `context.Context` per Go standards. `DefaultAppSlug = "gh-agentic-app"` constant — can be overridden at construction for tests.

## Remaining Tasks

- [ ] #642 — Add install URL builder, browser opener, and IsInteractive helper ← current
- [ ] #643 — Wire GitHub App install detection into gh agentic init
- [ ] #644 — Wire GitHub App install detection into gh agentic project join
- [ ] #645 — Update skills/gh-agentic-tool.md to document App install behaviour
