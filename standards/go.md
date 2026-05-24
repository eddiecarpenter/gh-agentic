# Go — Language and Framework Standards

Apply these rules when working in any Go package in this repository.

---

## Project Initialisation

Run these commands to scaffold a new Go project. Do not create files by hand.

```bash
go mod init github.com/<owner>/<repo-name>
mkdir -p cmd/<repo-name> internal
cat > cmd/<repo-name>/main.go << 'EOF'
package main

func main() {}
EOF
git add go.mod cmd/ internal/
```

- `go mod init` writes the correct installed Go version into `go.mod` automatically — never edit `go.mod` by hand.
- Commit: `chore: scaffold Go project structure`

---

## Build Verification

After any change to Go source, imports, or dependencies — run in this order:

```bash
gofmt -w ./...
go mod tidy
go build ./...
go test ./...
```

Never claim an implementation is complete without all four passing.

---

## Verification Gate (build + test)

The build+test pass is the **mandatory gate** at two specific
points in the pipeline. The same two commands run in both places:

```bash
go build ./...
go test ./...
```

Both must exit zero. Any non-zero exit — compilation failure,
failing test, vet failure surfaced through `go test`, race-detector
data race report under `go test -race`, etc. — **fails** the gate.

### Stack-eligibility pre-check (manifest presence)

The Go gate only applies to a repository when its **manifest file
is present**:

```bash
test -f go.mod
```

If `go.mod` is **not** present at the repo root (i.e. this isn't a
Go module), the Go gate is **not eligible** for this repository.
Compliance and the dev session SKIP the gate with a WARN ("Go
manifest `go.mod` not present — Go gate not applicable to this
repo"); the verdict is **not a fail**.

This handles the legitimate case where a multi-stack repo (e.g.
Go + TypeScript) gets a Feature whose diff touches only Go-extension
files in a directory that is NOT a Go module — those files are not
verifiable as a Go project and the gate should not pretend
otherwise.

### Dev Session — last step before exit

After the final task commit and before the workflow applies
`in-verification`, the dev session **MUST** run the gate when
eligible (manifest present). On gate failure the dev session does
NOT exit cleanly — it loops back to fix the breakage and re-runs
the gate until it passes. Pushing a broken build or a failing test
suite and signalling completion is forbidden.

The dev session's exit block must state, for each touched stack,
whether the gate ran and what its result was (PASS / FAIL / SKIPPED).
An exit block that omits the gate result is itself a protocol
violation.

### Compliance Verify — first step before any other check

The compliance verifier **MUST** run the gate (when eligible)
before evaluating acceptance criteria, static analysis, or any
other check. On gate failure the verifier emits an immediate FAIL
verdict and short-circuits — ACs cannot be PASS while the build is
broken or the tests fail, regardless of what code inspection
suggests.

The gate's run-and-result is the first item in the compliance
report. Subsequent sections (static analysis, AC table) appear only
when the gate passed or was skipped per the rules below.

### When the toolchain is unavailable

If `go.mod` IS present but `go` is **not** on the runner's PATH,
the gate is treated as **SKIPPED with a WARN** — not as PASS, not
as FAIL, not as BLOCKED. The recipe records:

- a `compliance-warn:v1` comment noting that the Go gate was skipped
  because the toolchain isn't installed on the runner, with the
  exact `which go` / probe output
- a recommendation to install the Go toolchain on the runner image
  (via `actions/setup-go@v5` or an apt step) so the gate can actually
  run on the next cycle
- AC verdicts for build / test fields are marked **WARN — skipped**
  rather than PASS or FAIL

Compliance still produces an overall verdict, runs the static
analysis section, and evaluates the AC table. The PR is permitted
to open. CI ( `build-and-test.yml` or equivalent) is the
authoritative backstop for actually running the tests once the PR
is open.

This is a deliberate softening of an earlier rule (v2.8.15) that
required BLOCKED on toolchain absence. The softening is the right
trade-off when the alternative is permanent stuck-state for a
real Feature whose runner image cannot be changed in the moment.
The intent is preserved by the "WARN, never PASS by inspection"
rule below.

### What is still forbidden

- **PASS-by-inspection is still forbidden.** Compliance MUST NOT
  emit a PASS verdict for the gate based on diff inspection alone.
  The gate is either PASS (commands ran and exited zero), FAIL
  (commands ran and exited non-zero), or SKIPPED-with-WARN
  (commands could not run). No fourth state.
- **FAIL-by-inspection is still forbidden.** Compliance MUST NOT
  emit FAIL based on a CI run from a closed PR, sibling branch, or
  any commit other than the current branch HEAD. Same trap, opposite
  direction.

---

## Coding Standards

- **Context propagation**: Every I/O function (DB, HTTP, Kafka) accepts `context.Context` as first parameter and propagates it. Never use `context.Background()` inside business logic — only at entry points.
- **Nil safety**: Check all pointer, interface, slice, and map returns before use.
- **Panics**: Never use `panic` in business logic. Handlers must recover from unexpected panics.
- **Interface design**: Define interfaces at the point of consumption. Keep them small and focused. Accept interfaces, return concrete types.
- **Struct initialisation**: Always use named fields — positional initialisation is prohibited.
- **Constants**: Numeric literals and strings with business meaning must be named constants. Timeouts, retry counts, and thresholds come from YAML config.
- **Time**: Never call `time.Now()` inside business logic — inject as a parameter. Store/publish UTC. Use `.Equal()`, `.Before()`, `.After()` for comparison.
- **Financial values**: All financial values use `github.com/shopspring/decimal` — no float types. Read precision from `DecimalDigits int32` config field — never hardcode.
- **Sensitive data**: Never log subscriber identifiers, balances, or transaction amounts. Never return internal errors or stack traces to API callers. Credentials in YAML config only.
- **Concurrency**: Protect shared mutable state with `sync.Mutex`, `sync.RWMutex`, atomics, or channels. Every goroutine must terminate via context or stop channel. Run `go test -race ./...` for concurrent code.

---

## Error Handling

- All domain errors use typed error structs with a `Code` type and constructor functions — reference: `internal/chargeengine/ocserrors/errors.go`
- Never use `fmt.Errorf` or `errors.New` for domain errors
- Error codes must be meaningful stable identifiers: `"UNKNOWN_SUBSCRIBER"`, `"OUT_OF_FUNDS"`
- Use `errors.As` for type assertions — never string comparison
- `fmt.Errorf` is permitted only for wrapping infrastructure errors (DB, network, I/O)

---

## Testing

**Commands:**
```bash
go test ./...                    # all tests
go test -race ./...              # required for concurrent code
go test ./internal/quota/...     # specific package
go test -run TestName ./...      # specific test
```

**Requirements:**
- Every Go source file with functions must have an accompanying `_test.go` file
- Files that only declare structs, constants, types, or interfaces are exempt
- Tests must run and pass — writing without running does not satisfy this rule
- Unit tests must NOT require external services (PostgreSQL, Kafka)

**Table-driven tests** — required for functions with multiple input/output combinations:
```go
tests := []struct {
    name     string
    input    SomeType
    expected SomeResult
}{
    {name: "zero value returns default", ...},
    {name: "negative amount returns error", ...},
}
for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) { ... })
}
```

**Test naming:** `TestFunctionName_Scenario_ExpectedBehaviour`
e.g. `TestDebitQuota_InsufficientBalance_ReturnsOutOfFunds`

---

## Architecture Boundaries

- Transport handlers must be thin — delegate all logic to services
- No business logic in HTTP or Diameter handlers
- All database access through repository interfaces in `internal/store/`
- Kafka consumers must delegate to services — no business logic in consumers
- New applications must follow structural patterns of existing applications
- Configuration from YAML only — no environment variables in application code

---

## Dependency Management

- Prefer libraries already used in the project over introducing new ones
- Verify new module paths on `pkg.go.dev` before adding — do not assume import paths
- If internet access is unavailable, state explicitly that verification was not performed
- Never modify files marked `// Code generated ... DO NOT EDIT` — re-run the generator

---

## Contract Structures — Go-Specific Rules

For the full contract framework and approval rules, see `.agents/RULEBOOK.md` — "Contract Rules".

**Kafka event structs** in `internal/events/` are identified by:
- Structs with a field typed as `*EventType` (e.g. `WholesaleContractEventType`)
- Structs referenced in consumer `handleRecord` switch statements
- Any struct with JSON tags that is `json.Unmarshal`-ed from a Kafka record

**Database-serialised structs** are identified by:
- Structs stored via `json.Marshal` into a `pgtype.JSONB` column
- Any struct referenced in a sqlc query as a JSON column type

**Never add internal domain IDs to event structs.** Generate any internal identifier inside the service layer after consuming the event.

**`internal/events/` is read-only** for AI agents unless the task explicitly states "modify the event schema" and a human has approved it.

---

## Documentation

- All public functions and methods must have a Go doc comment
- Comments must describe what and why — not restate the code

---

## Static Analysis

The compliance-verify skill reads this section to execute the correct toolchain
when verifying a Go Feature. Run these tools in order against the full module tree.

### Native tools — commands

| Tool | Command | Notes |
|---|---|---|
| Go vet | `go vet ./...` | Always available with the Go toolchain |
| golangci-lint | `golangci-lint run ./...` | Skip if absent (`which golangci-lint` fails) |
| govulncheck | `govulncheck ./...` | Skip if absent (`which govulncheck` fails) |

### Native tools — severity mapping

| Tool | Finding type | Compliance severity |
|---|---|---|
| `go vet` | any finding | CRITICAL |
| `golangci-lint` | gosec / G-series (security rules) | CRITICAL |
| `golangci-lint` | errcheck, staticcheck SA-series (bug rules) | MAJOR |
| `golangci-lint` | gofmt, goimports, style rules | MINOR |
| `govulncheck` | vulnerability in packages touched by the diff | CRITICAL |
| `govulncheck` | vulnerability in transitive dependencies only | MAJOR |

### Race condition gate

Packages containing goroutines, channels, or shared mutable state must also pass:

```bash
go test -race ./...
```

Any data-race report → CRITICAL finding.

---

## Compliance & Quality

The compliance-verify skill reads this section to determine what to enforce when
verifying a Go Feature's implementation. Rules here are machine-parseable
constraints — they supplement (not replace) the guidance in the sections above.

### Test Quality Expectations

Coverage numbers alone are not sufficient. The compliance verifier additionally
enforces:

- Tests must assert on the content of return values, not merely their non-nil-ness.
  A test that only checks `if err != nil` without inspecting the error type or value
  does not satisfy the error-path coverage requirement.
- Table-driven tests are required for functions with two or more input/output
  combinations (see the Testing section above). A test file that exercises
  multi-case logic through repeated identical `t.Run` blocks rather than a
  `tests := []struct{}` table fails the quality check.
- At least 50% of test lines must exercise non-trivial logic paths — trivial
  getter/setter tests are insufficient. Coverage inflated purely by testing simple
  field access does not satisfy the 80% threshold in spirit.

### Go-Specific Enforcement Rules

1. **Accompanying test file** — every Go source file that declares at least one
   function must have a corresponding `_test.go` file. Files that only declare
   types, constants, or interfaces are exempt. Files without a companion test file
   fail the compliance check.

2. **No content-free assertions** — test code must not assert only on non-nil
   returns without also verifying the returned value's meaningful content (error
   code, field value, slice length, etc.). `assert.NotNil(t, err)` without a
   follow-up `assert.Equal(t, expectedCode, err.Code)` (or equivalent) is a
   failing pattern.

3. **No artificial coverage inflation** — coverage must not be padded with
   trivial getter/setter tests or tests that do nothing but call a constructor
   and assert the struct is non-nil. At least 50% of test lines must touch
   conditional branches, error paths, or non-trivial computation.

4. **Race-condition gate** — packages containing goroutines, channels, or shared
   mutable state must pass `go test -race ./...` without data-race reports. A
   package in this category that is not run with `-race` fails the check.
