# Recovery State

| Field               | Value                                 |
|---------------------|---------------------------------------|
| Feature issue       | #527                                  |
| Branch              | feature/527-federated-org-scoped-vars |
| Last commit         | 36cbab6                               |
| Total tasks         | 7                                     |
| Last updated        | 2026-04-19T02:25:44Z                  |

## Completed Tasks

### #528 — Add ScopeFor routing helper in internal/project/scope.go
- **Implemented:** `ScopeFor`, `IsSharedName`, `IsIdentityName`, and the canonical shared / identity name sets. Shared names route to `--org` under any federated topology variant; identity names and unknown names/topologies default to `--repo`.
- **Files changed:** `internal/project/scope.go`, `internal/project/scope_test.go` (later relocated to `internal/scope/`)
- **Decisions:** Exported scope flag constants and the name-classification helpers.

### #529 — Refuse federated topology under a user account
- **Implemented:** `EnsureFederatedOwnerIsOrg` with the verbatim error message. Guard wired into project.Create, project.InitRepo, project.repairTopologyVars, cli/repair.go `--topology federated`. `CreateConfig` gained an optional Topology field so single-topology init opts out.
- **Files changed:** `internal/project/guards.go`, `internal/project/guards_test.go`, `internal/project/create.go`, `internal/project/create_test.go`, `internal/project/init.go`, `internal/project/init_test.go`, `internal/project/repair.go`, `internal/project/repair_test.go`, `internal/cli/repair.go`
- **Decisions:** Guard is case-tolerant (ToLower); ScopeFor's strict matcher kept case-sensitive. Create now writes the caller-supplied topology marker rather than hard-coding "federated".

### #530 — Route variable and secret writes through ScopeFor
- **Implemented:** Moved the scope logic to a new dependency-free package `internal/scope/` so initv2, auth, and project can import it without creating cycles. `internal/project/scope.go` is now a thin re-export. Routed the three write sites (initv2.ConfigureRepo, doctorv2.ApplyPendingPrompt, auth.uploadCredentials) through `scope.ScopeFor`. Extended `PendingPrompt` with Topology + Owner so RepairPipeline can carry them onto ApplyPendingPrompt. Tests assert captured gh invocations under both federated and single topology for every named var/secret.
- **Files changed:** `internal/scope/scope.go` (new), `internal/scope/scope_test.go` (moved), `internal/project/scope.go` (re-exports), `internal/project/scope_test.go` (slim re-export test), `internal/project/guards.go` (uses `scope.IsFederatedTopology`), `internal/initv2/wizard.go`, `internal/initv2/wizard_test.go`, `internal/doctorv2/repair.go`, `internal/doctorv2/repair_test.go`, `internal/auth/auth.go`, `internal/auth/auth_test.go`
- **Decisions:** New `internal/scope/` package avoids the import cycle that would form if `project` also imported `initv2` (it does via `project/init.go`) AND `initv2` imported `project`. Re-exports preserve task #528's public API location. `uploadCredentials` derives a synthetic topology ("federated" for OwnerTypeOrg, "single" otherwise) so a single ScopeFor call replaces the manual ownerType branching.

## Remaining Tasks

- [ ] #531 — Extend doctorv2 secret/variable checks to query org scope under federated ← current
- [ ] #532 — Add shadow-vars doctor check (hard Fail) for federated repos
- [ ] #533 — Auto-repair shadow values via single huh confirmation
- [ ] #534 — Add federated init confirmation note about org-level visibility
