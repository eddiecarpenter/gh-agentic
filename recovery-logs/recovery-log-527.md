# Recovery State

| Field               | Value                                 |
|---------------------|---------------------------------------|
| Feature issue       | #527                                  |
| Branch              | feature/527-federated-org-scoped-vars |
| Last commit         | aa2a51e                               |
| Total tasks         | 7                                     |
| Last updated        | 2026-04-19T02:35:21Z                  |

## Completed Tasks

### #528 — Add ScopeFor routing helper
- Single source of truth for scope routing; `ScopeFor`, `IsSharedName`, `IsIdentityName`.

### #529 — Refuse federated topology under a user account
- `EnsureFederatedOwnerIsOrg` guard wired into project.Create, project.InitRepo, project.repairTopologyVars, cli/repair.go.

### #530 — Route variable and secret writes through ScopeFor
- Moved routing to `internal/scope/` package; routed initv2.ConfigureRepo, doctorv2.ApplyPendingPrompt, auth.uploadCredentials.

### #531 — Extend doctorv2 checks to query org scope under federated
- `checkVariable` / `checkSecret` consult org scope for shared names under federated.

### #532 — Add shadow-vars doctor check
- `checkShadowVars` + exported `FindShadowValues`. Fails with exact delete command for each shadowed shared name. Added `Data any` on `CheckResult`.

### #533 — Auto-repair shadow values via single huh confirmation
- `RepairShadowValues` drives a single injectable ConfirmFunc prompt plus batch deletes. RepairPipeline collects items into `ShadowBatch`. CLI wires `huhConfirm` after the spinner phase. Batch continues on single-item failure; No preserves the manual commands as Unrepaired.
- **Files changed:** `internal/doctorv2/repair.go`, `internal/doctorv2/repair_test.go`, `internal/cli/repair.go`

## Remaining Tasks

- [ ] #534 — Add federated init confirmation note about org-level visibility ← current
