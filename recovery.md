# Recovery State

| Field               | Value                                 |
|---------------------|---------------------------------------|
| Feature issue       | #527                                  |
| Branch              | feature/527-federated-org-scoped-vars |
| Last commit         | 1f76546                               |
| Total tasks         | 7                                     |
| Last updated        | 2026-04-19T02:09:12Z                  |

## Completed Tasks

### #528 — Add ScopeFor routing helper in internal/project/scope.go
- **Implemented:** Added `internal/project/scope.go` with `ScopeFor`, `IsSharedName`, `IsIdentityName`, and the canonical shared / identity name sets. Shared names route to `--org` under any federated topology variant; identity names and unknown names/topologies default to `--repo`.
- **Files changed:** `internal/project/scope.go`, `internal/project/scope_test.go`
- **Decisions:** Exported `ScopeFlagOrg` / `ScopeFlagRepo` constants so every call site uses the same literal. Unknown topology strings (including "") are treated as not-federated — write stays at `--repo` to preserve current behaviour until a later task explicitly widens scope.

## Remaining Tasks

- [ ] #529 — Refuse federated topology under a user account (hard guards) ← current
- [ ] #530 — Route variable and secret writes through ScopeFor
- [ ] #531 — Extend doctorv2 secret/variable checks to query org scope under federated
- [ ] #532 — Add shadow-vars doctor check (hard Fail) for federated repos
- [ ] #533 — Auto-repair shadow values via single huh confirmation
- [ ] #534 — Add federated init confirmation note about org-level visibility
