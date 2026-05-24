# React — Framework Standards

Apply these rules in addition to `standards/typescript.md` when working in any React project.

---

## Build Verification

After any change to React source, components, or dependencies — run in this order:

```bash
npx prettier --check .
npx tsc --noEmit
npm test
```

Never claim an implementation is complete without all three passing.

---

## Verification Gate (build + test)

The Verification Gate for React projects is identical to TypeScript —
React is a TypeScript-superset standard. See
[`standards/typescript.md` → Verification Gate](typescript.md#verification-gate-build--test)
for the full contract:

- The same gate commands apply: `npx tsc --noEmit`, `npm test`,
  `npm run build`.
- The same manifest-presence pre-check applies (`package.json`).
- The dev-session runs the gate as its last step before exit when
  the gate is eligible; on FAIL it loops back and never claims
  completion.
- The compliance verifier runs the gate as its first step before
  any AC evaluation when the gate is eligible; FAIL short-circuits.
- When the toolchain is absent on the runner, the gate is treated
  as **SKIPPED with a WARN** — not PASS, not FAIL, not BLOCKED.
  PASS-by-inspection and FAIL-by-inspection both remain forbidden.

Skills loading the gate definition may load either `standards/react.md`
or `standards/typescript.md` for a React project — the gate contract
is the same.

---

## Component Rules

- **Functional components only** — class components are prohibited
- **One component per file** — the file name matches the component name (`UserCard.tsx`)
- **Props must be typed** — define an explicit interface for every component's props; never use inline object types or `any`
- **Default exports for components** — named exports for utilities and hooks in the same file

```typescript
interface UserCardProps {
  userId: string;
  displayName: string;
  onSelect?: (id: string) => void;
}

export default function UserCard({ userId, displayName, onSelect }: UserCardProps) {
  // ...
}
```

---

## Hooks

- **Custom hooks** — extract any stateful or side-effect logic shared across two or more components into a custom hook (`use<Name>.ts`)
- **`useEffect` discipline** — every `useEffect` must declare all dependencies in its array; never suppress the exhaustive-deps lint rule without a code comment explaining why
- **No direct DOM manipulation** — use `useRef` where a DOM handle is needed; never query selectors inside components
- **State shape** — keep state as flat as possible; derive computed values with `useMemo` rather than storing derived state

---

## File Structure

```
src/
  components/         # presentational components
  features/           # feature-scoped components and their hooks
  hooks/              # shared custom hooks
  services/           # API clients and external integrations
  types/              # shared TypeScript types and interfaces
```

Components that are only used by one feature live under `features/<name>/` — they do not move to `components/` until they are used by a second feature.

---

## Styling

- Co-locate styles with the component — CSS modules (`UserCard.module.css`) or a CSS-in-JS library already in use in the project
- Never use inline `style` props for anything other than truly dynamic values (e.g. computed pixel widths)
- Never use global class names that are not scoped to the component

---

## Testing

**Frameworks:** Vitest + React Testing Library

**Install:**
```bash
npm install --save-dev @testing-library/react @testing-library/user-event @testing-library/jest-dom
```

**Commands:**
```bash
npm test                        # run all tests
npm test -- --watch             # watch mode
npm test -- UserCard            # filter by component name
```

**Requirements:**
- Every component file must have an accompanying `.test.tsx` file
- Test behaviour from the user's perspective — query by role, label, or visible text; never by CSS class or test ID unless unavoidable
- Unit tests must NOT require a real backend — mock `fetch` / API clients at the module boundary

**Test structure:**
```typescript
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import UserCard from "./UserCard";

describe("UserCard", () => {
  it("displays the user's name", () => {
    render(<UserCard userId="1" displayName="Alice" />);
    expect(screen.getByText("Alice")).toBeInTheDocument();
  });

  it("calls onSelect with the userId when clicked", async () => {
    const onSelect = vi.fn();
    render(<UserCard userId="1" displayName="Alice" onSelect={onSelect} />);
    await userEvent.click(screen.getByRole("button"));
    expect(onSelect).toHaveBeenCalledWith("1");
  });
});
```

**Test naming:** `describe` block = component name; `it` block = what the user sees or does, in plain English.

---

## Accessibility

- Every interactive element must be reachable by keyboard and have an accessible name
- Use semantic HTML elements (`<button>`, `<nav>`, `<main>`) before reaching for `<div>` with an `onClick`
- Images must have descriptive `alt` text; decorative images use `alt=""`

---

## Static Analysis

React projects inherit the full `standards/typescript.md` `## Static Analysis`
toolchain — run all TypeScript native tools and apply all TypeScript severity
mappings first. The additions below are React-specific and supplement (not
replace) the TypeScript rules.

### Additional ESLint plugins — commands

The following plugins must be configured in `.eslintrc.*` in addition to the
TypeScript ESLint setup:

```bash
npm install --save-dev \
  eslint-plugin-react \
  eslint-plugin-react-hooks \
  eslint-plugin-jsx-a11y
```

Run ESLint with the extended config:

```bash
npx eslint . --ext .ts,.tsx
```

(Same command as TypeScript — the additional plugins activate via `.eslintrc.*`.)

### React-specific severity mapping

These supplement the TypeScript severity mapping table:

| Tool | Rule / finding type | Compliance severity |
|---|---|---|
| `eslint-plugin-react-hooks` | `rules-of-hooks` violation | CRITICAL — hook ordering violations cause runtime crashes |
| `eslint-plugin-react-hooks` | `exhaustive-deps` violation | MAJOR — stale closures produce incorrect behaviour |
| `eslint-plugin-jsx-a11y` | any accessibility rule violation | MAJOR — accessibility is a delivery requirement |
| `eslint-plugin-react` | deprecated lifecycle or API usage | MAJOR |
| `eslint-plugin-react` | style or display-name rules | MINOR |

---

## Compliance & Quality

The compliance-verify skill reads this section to determine what to enforce when
verifying a React Feature's implementation. Rules here are machine-parseable
constraints — they supplement (not replace) the guidance in the sections above.

### Test Quality Expectations

Coverage numbers alone are not sufficient. The compliance verifier additionally
enforces:

- Component tests must render the component via React Testing Library — never
  import and call component functions directly without rendering.
- Tests must query by accessible role, label text, or visible text content. Tests
  that query by CSS class name or `data-testid` are permitted only when no
  accessible query is available, and must include a code comment explaining why.
- Every interactive element (buttons, form inputs, links) must have at least one
  test that exercises the interaction via `userEvent` — not just that the element
  renders.
- At least 50% of test lines must exercise non-trivial logic — event handlers,
  conditional rendering, async state changes. Tests that only verify initial render
  without any interaction or state change do not satisfy the 80% threshold in spirit.

### React-Specific Enforcement Rules

1. **Accompanying test file** — every `.tsx` component file must have a
   corresponding `.test.tsx` file. Files that only re-export or declare types are
   exempt. Files without a companion test file fail the compliance check.

2. **No shallow rendering** — tests must use `render` from React Testing Library,
   not `shallow` from Enzyme or any equivalent. Shallow rendering does not exercise
   child component logic and does not satisfy coverage requirements.

3. **Every `useEffect` with external I/O must have a cleanup test** — any component
   that calls an API, starts a timer, or subscribes to an event inside `useEffect`
   must have at least one test that verifies cleanup occurs (e.g. the subscription
   is cancelled on unmount). Missing cleanup tests are a failing pattern.

4. **No `act()` warnings in test output** — React's `act()` warning indicates a
   state update was not properly awaited in the test. Any test run that produces
   `act()` warnings fails the compliance check, even if all assertions pass.
