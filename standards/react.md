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
