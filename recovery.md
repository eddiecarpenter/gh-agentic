# Recovery State

| Field               | Value                                     |
|---------------------|-------------------------------------------|
| Feature issue       | #625                                      |
| Branch              | feature/625-cli-app-install-helper        |
| Last commit         | e53c946                                   |
| Total tasks         | 5                                         |
| Last updated        | 2026-04-24T04:50:00Z                      |

## Completed Tasks

### #641 — Add GitHub App installation detection API
- **Implemented:** `internal/githubapp.Checker` with `CheckRepoInstallation`, `CheckOrgInstallation`; injectable `RESTClient` interface for tests.
- **Files changed:** `internal/githubapp/installation.go`, `internal/githubapp/installation_test.go`
- **Decisions:** Used `DoWithContext` to propagate `context.Context`. `DefaultAppSlug = "gh-agentic-app"`.

### #642 — Add install URL builder, browser opener, and IsInteractive helper
- **Implemented:** `InstallURL(slug,targetType,targetName)`, `ui.OpenURL` (injectable var), `ui.IsInteractive` / `ui.IsCI`, and refactored `busy.go:busySuppressed` to delegate TTY detection to `IsInteractive`.
- **Files changed:** `internal/githubapp/installurl.go`, `internal/githubapp/installurl_test.go`, `internal/ui/browser.go`, `internal/ui/browser_test.go`, `internal/ui/tty.go`, `internal/ui/tty_test.go`, `internal/ui/busy.go`, `go.mod`, `go.sum`
- **Decisions:** Two indirect deps added transitively via go-gh browser (`cli/browser`, `google/shlex`). `IsCI` is strict — only `GITHUB_ACTIONS=true` or `CI=true`.

## Remaining Tasks

- [ ] #643 — Wire GitHub App install detection into gh agentic init ← current
- [ ] #644 — Wire GitHub App install detection into gh agentic project join
- [ ] #645 — Update skills/gh-agentic-tool.md to document App install behaviour
