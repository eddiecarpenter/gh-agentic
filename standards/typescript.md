# TypeScript — Language and Framework Standards

Apply these rules when working in any TypeScript package in this repository.

---

## Project Initialisation

```bash
npm init -y
npm install --save-dev typescript @types/node
npx tsc --init
```

`tsconfig.json` must enable strict mode — never disable it:

```json
{
  "compilerOptions": {
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  }
}
```

---

## Build Verification

After any change to TypeScript source, imports, or dependencies — run in this order:

```bash
npx prettier --check .
npx tsc --noEmit
npm test
```

Never claim an implementation is complete without all three passing.

To auto-fix formatting before committing:

```bash
npx prettier --write .
```

---

## Verification Gate (build + test)

The build+test pass is the **mandatory gate** at two specific
points in the pipeline. The same three commands run in both places:

```bash
npx tsc --noEmit
npm test
npm run build
```

`tsc --noEmit` catches type errors without producing emitted files;
`npm test` runs the project's test command (project must define a
`test` script in `package.json`); `npm run build` exercises the
actual production-build path (`vite build`, `tsc -b`, or whatever
`scripts.build` resolves to) — a project that type-checks and tests
clean but fails to bundle is not shippable.

All three must exit zero. Any non-zero exit — type error, failing
test, bundler error, missing `build`/`test` script — **fails** the
gate.

If `package.json` has no `test` script (genuinely no tests in the
project — rare, but possible for a CLI wrapper or a fresh scaffold),
the gate is satisfied when `tsc --noEmit` and `npm run build` exit
zero AND the standards file's "Testing" section has confirmed the
project's testing strategy with the human. An empty
`scripts.test` left at the npm-init default (`"test": "echo \"Error:
no test specified\" && exit 1"`) is NOT acceptable — it must be a
real command or removed.

### Stack-eligibility pre-check (manifest presence)

The TypeScript gate only applies to a directory that **contains a
`package.json`**:

```bash
test -f package.json   # at the repo root, or in the closest
                       # ancestor of every changed TS/JS file
```

If no `package.json` is present (the repo isn't an npm project, or
the changed files live outside any npm-rooted subtree), the
TypeScript gate is **not eligible**. Compliance and the dev session
SKIP it with a WARN ("TypeScript manifest `package.json` not present
— TypeScript gate not applicable to this scope"); the verdict is
**not a fail**.

For monorepos with multiple `package.json` files, each gate run is
scoped to the directory containing the closest enclosing
`package.json` of the changed files.

### Dev Session — last step before exit

After the final task commit and before the workflow applies
`in-verification`, the dev session **MUST** run the gate when
eligible. On failure the dev session does NOT exit cleanly — it
loops back to fix the breakage and re-runs the gate until it
passes. Pushing broken TypeScript, type errors, failing tests, or
a build that doesn't bundle and signalling completion is forbidden.

The dev session's exit block must state, for each touched stack,
whether the gate ran and what its result was (PASS / FAIL / SKIPPED).
An exit block that omits the gate result is itself a protocol
violation.

### Compliance Verify — first step before any other check

The compliance verifier **MUST** run the gate (when eligible)
before evaluating acceptance criteria, static analysis, or any
other check. On gate failure the verifier emits an immediate FAIL
verdict and short-circuits.

The gate's run-and-result is the first item in the compliance
report. Subsequent sections (static analysis, AC table) appear only
when the gate passed or was skipped per the rules below.

### When the toolchain is unavailable

If `package.json` IS present but `node` or `npm` is **not** on the
runner's PATH (or `node_modules/` is missing and `npm ci` fails to
install), the gate is treated as **SKIPPED with a WARN** — not as
PASS, not as FAIL, not as BLOCKED. The recipe records:

- a `compliance-warn:v1` comment noting that the TypeScript gate
  was skipped because the toolchain isn't installed on the runner,
  with the exact `which node` / `which npm` probe output
- a recommendation to install Node.js on the runner image (via
  `actions/setup-node@v4` or an apt step) so the gate can actually
  run on the next cycle
- AC verdicts for build / test fields are marked **WARN — skipped**
  rather than PASS or FAIL

Compliance still produces an overall verdict, runs the static
analysis section, and evaluates the AC table. The PR is permitted
to open. CI is the authoritative backstop for actually running the
tests once the PR is open.

### What is still forbidden

- **PASS-by-inspection is still forbidden.** The gate is either
  PASS (commands ran and exited zero), FAIL (commands ran and
  exited non-zero), or SKIPPED-with-WARN (commands could not run).
  No fourth state.
- **FAIL-by-inspection is still forbidden.** Compliance MUST NOT
  emit FAIL based on a CI run from a closed PR, sibling branch, or
  any commit other than the current branch HEAD.

---

## Dependency Management

```bash
npm install <package>          # runtime dependency
npm install --save-dev <pkg>   # dev-only dependency
npm ci                         # clean install (CI / reproducible)
```

- Prefer libraries already used in the project over introducing new ones
- Never edit `package-lock.json` by hand — it is managed by npm
- Pin major versions in `package.json` — avoid unbound `*` ranges
- Verify package names on [npmjs.com](https://www.npmjs.com) before adding — do not assume import paths

---

## Coding Standards

- **No `any`** — use `unknown` and narrow with type guards, or define a proper type
- **Explicit return types** — all exported functions and class methods must declare their return type
- **Null safety** — use optional chaining (`?.`) and nullish coalescing (`??`); never assert non-null (`!`) without a code comment explaining why it cannot be null
- **Immutability** — prefer `const` over `let`; never use `var`; prefer `readonly` on interface fields where the value is not intended to mutate
- **Enums** — use `const enum` or string literal union types (`type Status = "active" | "inactive"`) over numeric enums
- **Error handling** — never `throw` raw strings; define typed error classes or use a discriminated union result type (`{ ok: true; value: T } | { ok: false; error: Error }`)
- **Async** — use `async`/`await` exclusively; never mix `.then()`/`.catch()` chains with `await` in the same function
- **Sensitive data** — never log personally identifiable information, credentials, or financial values

---

## Naming Conventions

| Construct | Convention | Example |
|---|---|---|
| Variables / functions | `camelCase` | `getUserById` |
| Classes / interfaces / types | `PascalCase` | `UserService`, `ApiResponse` |
| Constants | `UPPER_SNAKE_CASE` | `MAX_RETRY_COUNT` |
| Files | `kebab-case.ts` | `user-service.ts` |
| Test files | `<name>.test.ts` | `user-service.test.ts` |

---

## Testing

**Framework:** Vitest

**Commands:**
```bash
npm test                        # run all tests
npm test -- --watch             # watch mode
npm test -- --coverage          # with coverage report
npm test -- user-service        # filter by name
```

**Requirements:**
- Every `.ts` file with exported functions must have an accompanying `.test.ts` file
- Files that only declare types, interfaces, or re-export are exempt
- Tests must run and pass — writing without running does not satisfy this rule
- Unit tests must NOT require external services (databases, APIs) — mock at the boundary

**Test structure:**
```typescript
import { describe, it, expect, vi } from "vitest";

describe("getUserById", () => {
  it("returns the user when found", async () => {
    // arrange
    // act
    // assert
  });

  it("throws NotFoundError when user does not exist", async () => {
    await expect(getUserById("unknown")).rejects.toThrow(NotFoundError);
  });
});
```

**Test naming:** `describe` block = function or module name; `it` block = scenario in plain English.

---

## Architecture Boundaries

- Keep business logic out of HTTP handlers — delegate to service functions
- All external I/O (HTTP clients, database, queues) must be injectable for testing
- Configuration from environment variables at the entry point only — never read `process.env` inside business logic

---

## Documentation

- All exported functions, classes, and types must have a JSDoc comment
- Comments describe what and why — not restate the code

---

## Static Analysis

The compliance-verify skill reads this section to execute the correct toolchain
when verifying a TypeScript Feature. Run these tools in order against the full
source tree.

### Native tools — commands

| Tool | Command | Notes |
|---|---|---|
| TypeScript compiler | `npx tsc --noEmit` | Always run — type errors are hard failures |
| ESLint | `npx eslint . --ext .ts,.tsx` | Requires `@typescript-eslint/recommended` + `eslint-plugin-security` |
| npm audit | `npm audit --audit-level=moderate` | Known CVE scan against declared dependencies |

**Required ESLint plugins** — the following must be configured in `.eslintrc.*` for
the ESLint run to cover security and bug categories:

```bash
npm install --save-dev \
  @typescript-eslint/eslint-plugin \
  @typescript-eslint/parser \
  eslint-plugin-security \
  eslint-plugin-n
```

If `.eslintrc.*` is absent, record a MAJOR finding
`{ tool: "eslint", severity: "MAJOR", message: "No ESLint configuration found — static analysis rules not enforced" }`
and skip the ESLint run.

### Native tools — severity mapping

| Tool | Finding type | Compliance severity |
|---|---|---|
| `tsc --noEmit` | any type error | CRITICAL |
| ESLint (`eslint-plugin-security` rules) | security rule violation | CRITICAL |
| ESLint (`@typescript-eslint` — `no-floating-promises`, `no-misused-promises`, `strict-boolean-expressions`) | bug-prone rule | MAJOR |
| ESLint (`@typescript-eslint` — style/preference rules) | style rule | MINOR |
| `npm audit` | CRITICAL or HIGH severity CVE | CRITICAL |
| `npm audit` | MODERATE severity CVE | MAJOR |
| `npm audit` | LOW severity CVE | MINOR |

---

## Compliance & Quality

The compliance-verify skill reads this section to determine what to enforce when
verifying a TypeScript Feature's implementation. Rules here are machine-parseable
constraints — they supplement (not replace) the guidance in the sections above.

### Test Quality Expectations

Coverage numbers alone are not sufficient. The compliance verifier additionally
enforces:

- Tests must assert on the content of return values and thrown errors, not merely
  that they exist. A test that only checks `expect(result).toBeDefined()` without
  inspecting the result's meaningful content does not satisfy coverage requirements.
- Async tests containing multiple assertion branches must use `expect.assertions(N)`
  to prevent false-positive passing caused by a branch that does not run at all.
- At least 50% of test lines must exercise non-trivial logic — error paths,
  conditional branches, business-rule outcomes. Tests that only invoke constructors
  or access read-only properties do not satisfy the 80% threshold in spirit.

### TypeScript-Specific Enforcement Rules

1. **Accompanying test file** — every `.ts` file that exports at least one function
   must have a corresponding `.test.ts` file. Files that only declare types,
   interfaces, or re-export are exempt. Files without a companion test file fail
   the compliance check.

2. **No `as unknown as X` casts in test code** — casting to `unknown` and then
   to a specific type in test files hides type bugs that the test is meant to
   catch. Any occurrence of `as unknown as` in a `.test.ts` file is a failing
   pattern.

3. **Every exported function must have an error-path test** — the happy path alone
   is insufficient. Each exported function must have at least one `it` block that
   exercises a failure scenario (rejected promise, thrown error, or returned error
   discriminant). Functions with no error-path test fail the compliance check.

4. **`expect.assertions(N)` in async multi-branch tests** — async test functions
   that contain two or more assertion branches (e.g. a `try/catch` with assertions
   in both arms) must declare `expect.assertions(N)` at the top of the `it` block
   to ensure every branch fires. Omitting this in a qualifying test is a failing
   pattern.
