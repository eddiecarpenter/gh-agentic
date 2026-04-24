# Recovery State

| Field               | Value                                     |
|---------------------|-------------------------------------------|
| Feature issue       | #625                                      |
| Branch              | feature/625-cli-app-install-helper        |
| Last commit         | 12da1a3                                   |
| Total tasks         | 5                                         |
| Last updated        | 2026-04-24T05:20:00Z                      |

## Completed Tasks

### #641 — Add GitHub App installation detection API
- **Implemented:** `internal/githubapp.Checker` with repo+org install checks; injectable `RESTClient` interface.
- **Files changed:** `internal/githubapp/installation.go` (+ test).
- **Decisions:** Context-propagating via `DoWithContext`; `DefaultAppSlug = "gh-agentic-app"`.

### #642 — Add install URL builder, browser opener, and IsInteractive helper
- **Implemented:** `InstallURL`, `ui.OpenURL`/`OpenURLFunc`, `ui.IsInteractive`/`ui.IsCI`; refactored `busy.go:busySuppressed` to delegate to `IsInteractive`.
- **Files changed:** `internal/githubapp/installurl.go`, `internal/ui/browser.go`, `internal/ui/tty.go`, `internal/ui/busy.go` (+ tests), `go.mod`, `go.sum`.
- **Decisions:** Indirect deps via go-gh browser; `IsCI` strict on `GITHUB_ACTIONS=true`/`CI=true`.

### #643 — Wire GitHub App install detection into gh agentic init
- **Implemented:** `githubapp.Flow` + `EnsureInstalled` four-path flow; hooked into `wizard.Run()` between mount and ConfigureRepo; `--skip-app-install` flag and production flow wiring in `cli/init.go`; `skills/gh-agentic-tool.md` `init` section updated.
- **Files changed:** `internal/githubapp/flow.go`, `internal/init/wizard.go`, `internal/init/appinstall_test.go`, `internal/cli/init.go`, `internal/cli/init_test.go`, `skills/gh-agentic-tool.md`.
- **Decisions:** Federated topology routes to `TargetOrg`. Browser-open failures log fallback and continue.

### #644 — Wire GitHub App install detection into gh agentic project join
- **Implemented:** inline install step + `--skip-app-install` flag in `gh agentic project join`. `Organization` owners → org endpoint; `User` owners → repo endpoint. Production `projectAppInstallFlow` is a package-level var tests override. `skills/gh-agentic-tool.md` `project join` section updated.
- **Files changed:** `internal/cli/project.go`, `internal/cli/project_appinstall_test.go`, `skills/gh-agentic-tool.md`.
- **Decisions:** DetectOwnerType failures fall back to the repo-level endpoint conservatively.

## Remaining Tasks

- [ ] #645 — Update skills/gh-agentic-tool.md to document App install behaviour ← current
