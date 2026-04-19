# Recovery State

| Field               | Value                                 |
|---------------------|---------------------------------------|
| Feature issue       | #527                                  |
| Branch              | feature/527-federated-org-scoped-vars |
| Last commit         | 9354609                               |
| Total tasks         | 7                                     |
| Last updated        | 2026-04-19T02:17:35Z                  |

## Completed Tasks

### #528 — Add ScopeFor routing helper in internal/project/scope.go
- **Implemented:** Added `internal/project/scope.go` with `ScopeFor`, `IsSharedName`, `IsIdentityName`, and the canonical shared / identity name sets. Shared names route to `--org` under any federated topology variant; identity names and unknown names/topologies default to `--repo`.
- **Files changed:** `internal/project/scope.go`, `internal/project/scope_test.go`
- **Decisions:** Exported `ScopeFlagOrg` / `ScopeFlagRepo` constants so every call site uses the same literal. Unknown topology strings (including "") are treated as not-federated — write stays at `--repo` to preserve current behaviour until a later task explicitly widens scope.

### #529 — Refuse federated topology under a user account
- **Implemented:** Added `EnsureFederatedOwnerIsOrg` in `internal/project/guards.go` with the verbatim error message. Wired the guard into every federated entry point (project.Create, project.InitRepo, project.repairTopologyVars, cli/repair.go `--topology federated`). Extended `CreateConfig` with an optional `Topology` field so single-topology init can opt out.
- **Files changed:** `internal/project/guards.go`, `internal/project/guards_test.go`, `internal/project/create.go`, `internal/project/create_test.go`, `internal/project/init.go`, `internal/project/init_test.go` (new), `internal/project/repair.go`, `internal/project/repair_test.go`, `internal/cli/repair.go`
- **Decisions:** Guard is case-tolerant (normalises to lower-case) because initv2 emits "Federated" not "federated"; the stricter `isFederatedTopology` helper shared with `ScopeFor` remains case-sensitive to preserve task #528's strict routing contract. `Create` now writes the caller-supplied topology marker rather than hard-coding "federated" — `initSingle` opts in via `Topology: "single"` and drops its redundant post-Create override.

## Remaining Tasks

- [ ] #530 — Route variable and secret writes through ScopeFor ← current
- [ ] #531 — Extend doctorv2 secret/variable checks to query org scope under federated
- [ ] #532 — Add shadow-vars doctor check (hard Fail) for federated repos
- [ ] #533 — Auto-repair shadow values via single huh confirmation
- [ ] #534 — Add federated init confirmation note about org-level visibility
