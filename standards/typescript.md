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
