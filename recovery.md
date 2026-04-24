# Recovery State

| Field               | Value                                     |
|---------------------|-------------------------------------------|
| Feature issue       | #625                                      |
| Branch              | feature/625-cli-app-install-helper        |
| Last commit         | 1dcaa58                                   |
| Total tasks         | 5                                         |
| Last updated        | 2026-04-24T05:05:00Z                      |

## Completed Tasks

### #641 — Add GitHub App installation detection API
- **Implemented:** `internal/githubapp.Checker` with `CheckRepoInstallation`, `CheckOrgInstallation`; injectable `RESTClient` interface.
- **Files changed:** `internal/githubapp/installation.go`, `internal/githubapp/installation_test.go`
- **Decisions:** Context-propagating via `DoWithContext`; `DefaultAppSlug = "gh-agentic-app"`.

### #642 — Add install URL builder, browser opener, and IsInteractive helper
- **Implemented:** `InstallURL`, `ui.OpenURL`/`OpenURLFunc`, `ui.IsInteractive`/`ui.IsCI`; refactored `busy.go:busySuppressed` to delegate to `IsInteractive`.
- **Files changed:** `internal/githubapp/installurl.go` (+ test), `internal/ui/browser.go` (+ test), `internal/ui/tty.go` (+ test), `internal/ui/busy.go`, `go.mod`, `go.sum`
- **Decisions:** Indirect deps added transitively via go-gh browser. `IsCI` strict — only `GITHUB_ACTIONS=true` / `CI=true`.

### #643 — Wire GitHub App install detection into gh agentic init
- **Implemented:** `githubapp.Flow` + `EnsureInstalled` four-path flow (installed / interactive-accept / interactive-decline / headless). Hooked into `wizard.Run()` between mount and ConfigureRepo. `--skip-app-install` flag and production flow wiring in `cli/init.go`. `skills/gh-agentic-tool.md` updated for init.
- **Files changed:** `internal/githubapp/flow.go`, `internal/githubapp/flow_test.go`, `internal/init/wizard.go`, `internal/init/appinstall_test.go`, `internal/cli/init.go`, `internal/cli/init_test.go`, `skills/gh-agentic-tool.md`
- **Decisions:** Federated topology routes to `TargetOrg` so one install covers all domain repos under the org. Browser-open failures log a fallback and continue — not fatal.

## Remaining Tasks

- [ ] #644 — Wire GitHub App install detection into gh agentic project join ← current
- [ ] #645 — Update skills/gh-agentic-tool.md to document App install behaviour
