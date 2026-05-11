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

### Coverage gate

Run the full test suite with coverage instrumentation:

```bash
npm test -- --coverage --reporter=json
```

Parse the `pct` field under `statements` from the generated `coverage/coverage-summary.json`:

```bash
COVERAGE=$(node -e "const c=require('./coverage/coverage-summary.json'); \
  console.log(c.total.statements.pct)")
```

**Threshold:** ≥ 80% statement coverage required.

| Coverage | Compliance severity |
|---|---|
| ≥ 80% | PASS — no finding |
| 70–79% | MAJOR |
| < 70% | CRITICAL |

If the test suite itself fails, record a CRITICAL finding per failing module and
proceed — coverage is unmeasurable but the failure must be reported.

### SonarQube — OWASP hotspot severity mapping

When SonarQube is configured, map security hotspot categories to compliance severity:

| OWASP categories | Compliance severity |
|---|---|
| A01 Broken Access Control, A02 Cryptographic Failures, A03 Injection | CRITICAL |
| A04 Insecure Design, A05 Security Misconfiguration, A06 Vulnerable & Outdated Components | MAJOR |
| A07 Auth Failures, A08 Integrity Failures, A09 Logging Failures, A10 SSRF | MAJOR |

---

## Compliance & Quality

The compliance-verify skill reads this section to determine what to enforce when
verifying a TypeScript Feature's implementation. Rules here are machine-parseable
constraints — they supplement (not replace) the guidance in the sections above.

### Coverage Threshold

≥80% statement coverage is required for every module containing business logic.

**Coverage command:**
```bash
npm test -- --coverage
```

Any module below 80% statement coverage fails the compliance check.

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
