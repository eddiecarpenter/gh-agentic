# Recovery State

| Field               | Value                                 |
|---------------------|---------------------------------------|
| Feature issue       | #527                                  |
| Branch              | feature/527-federated-org-scoped-vars |
| Last commit         | 202859d                               |
| Total tasks         | 7                                     |
| Last updated        | 2026-04-19T02:28:37Z                  |

## Completed Tasks

### #528 — Add ScopeFor routing helper in internal/project/scope.go
- **Implemented:** `ScopeFor`, `IsSharedName`, `IsIdentityName`, shared / identity name sets.
- **Files changed:** `internal/project/scope.go`, `internal/project/scope_test.go` (later moved to `internal/scope/`)
- **Decisions:** Exported scope flag constants and the name-classification helpers.

### #529 — Refuse federated topology under a user account
- **Implemented:** `EnsureFederatedOwnerIsOrg` guard wired into project.Create, project.InitRepo, project.repairTopologyVars, cli/repair.go `--topology federated`.
- **Decisions:** Guard is case-tolerant; `CreateConfig.Topology` field added so single-topology init can opt out.

### #530 — Route variable and secret writes through ScopeFor
- **Implemented:** Moved routing logic to a new dependency-free package `internal/scope/`. Routed the three write sites (initv2.ConfigureRepo, doctorv2.ApplyPendingPrompt, auth.uploadCredentials) through `scope.ScopeFor`. `PendingPrompt` now carries Topology + Owner.
- **Decisions:** `internal/project/scope.go` is a thin re-export to preserve task #528's API location and avoid the `project ↔ initv2` cycle.

### #531 — Extend doctorv2 secret/variable checks to query org scope under federated
- **Implemented:** `checkVariable` and `checkSecret` consult the org scope for shared names under any federated topology variant; identity names and single topology stay repo-only. Remediation hints reference the authoritative scope.
- **Files changed:** `internal/doctorv2/checks.go`, `internal/doctorv2/checks_test.go`
- **Decisions:** Helpers `shouldConsultOrg`, `containsVariableName`, `containsSecretName`, and `remediationSet` localise the scope-aware check logic. Repo-list error is treated as soft-warning only when the org fallback also could not confirm.

## Remaining Tasks

- [ ] #532 — Add shadow-vars doctor check (hard Fail) for federated repos ← current
- [ ] #533 — Auto-repair shadow values via single huh confirmation
- [ ] #534 — Add federated init confirmation note about org-level visibility
