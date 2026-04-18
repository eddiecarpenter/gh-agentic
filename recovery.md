# Recovery State

| Field               | Value                                            |
|---------------------|--------------------------------------------------|
| Feature issue       | #492                                             |
| Branch              | feature/492-gh-agentic-status                    |
| Last commit         | 02a522d                                          |
| Total tasks         | 11                                               |
| Last updated        | 2026-04-18T10:45:00Z                             |

## Completed Tasks

### #494 — Scaffold gh agentic status command group and four sub-command stubs
- **Implemented:** Added Cobra command group `gh agentic status` with four leaf
  sub-commands (requirements, requirement <N>, features, feature <N>) as stubs
  returning `errStatusNotImplemented`. All downstream flags (`--json`, `--kanban`,
  `--horizontal`, `--this-repo`, `--include-done`) declared. Wired into root.
- **Files changed:** internal/cli/status.go (new), internal/cli/status_test.go (new),
  internal/cli/root.go.
- **Decisions:** Stubs return a shared sentinel error `errStatusNotImplemented` so
  test assertions and later replacements stay simple. List vs detail flag sets split
  into two helper registrars (`registerStatusListFlags`, `registerStatusDetailFlags`).

## Remaining Tasks

- [ ] #495 — Build internal/projectstatus package — types, GraphQL queries, typed errors ← current
- [ ] #496 — Implement 'gh agentic status requirements' list command
- [ ] #497 — Implement 'gh agentic status requirement <N>' detail command
- [ ] #498 — Implement 'gh agentic status features' list command
- [ ] #499 — Implement 'gh agentic status feature <N>' detail command
- [ ] #500 — Implement --kanban renderer (vertical + --horizontal) with --json precedence
- [ ] #501 — Wire blocked-by dependency detection
- [ ] #502 — Extend 'gh agentic check' to verify AGENTIC_PROJECT_ID reachability
- [ ] #503 — Add centralised error renderer for status commands
- [ ] #504 — Lock JSON schema fixtures and add end-to-end integration tests
