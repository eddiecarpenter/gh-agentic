# Recovery State

| Field               | Value                                 |
|---------------------|---------------------------------------|
| Feature issue       | #527                                  |
| Branch              | feature/527-federated-org-scoped-vars |
| Last commit         | 567d220                               |
| Total tasks         | 7                                     |
| Last updated        | 2026-04-19T02:31:36Z                  |

## Completed Tasks

### #528 — Add ScopeFor routing helper in internal/project/scope.go
- **Implemented:** `ScopeFor`, `IsSharedName`, `IsIdentityName`, shared / identity name sets.
- **Files changed:** `internal/project/scope.go`, `internal/project/scope_test.go` (later moved to `internal/scope/`)

### #529 — Refuse federated topology under a user account
- **Implemented:** `EnsureFederatedOwnerIsOrg` guard wired into project.Create, project.InitRepo, project.repairTopologyVars, cli/repair.go `--topology federated`.

### #530 — Route variable and secret writes through ScopeFor
- **Implemented:** Moved routing logic to a new package `internal/scope/`. Routed the three write sites (initv2.ConfigureRepo, doctorv2.ApplyPendingPrompt, auth.uploadCredentials) through `scope.ScopeFor`. `PendingPrompt` now carries Topology + Owner.

### #531 — Extend doctorv2 secret/variable checks to query org scope under federated
- **Implemented:** `checkVariable` and `checkSecret` consult the org scope for shared names under federated; identity names and single topology stay repo-only.

### #532 — Add shadow-vars doctor check (hard Fail) for federated repos
- **Implemented:** `checkShadowVars` + exported `FindShadowValues` in `internal/doctorv2/checks.go`. Issues four list queries under federated, reports each shared name present at both scopes as a hard Fail with the exact delete command. Registered in the topology-aware check registry. Added `Data any` to `CheckResult` so the summary carries `[]ShadowValue` for the upcoming repair task.
- **Files changed:** `internal/doctorv2/checks.go`, `internal/doctorv2/groups.go`, `internal/doctorv2/checks_test.go`
- **Decisions:** Deterministic ordering via a stable slice of shared names; variables and secrets detected independently; single topology issues zero gh queries.

## Remaining Tasks

- [ ] #533 — Auto-repair shadow values via single huh confirmation ← current
- [ ] #534 — Add federated init confirmation note about org-level visibility
