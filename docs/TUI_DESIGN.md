# TUI_DESIGN.md — gh agentic bootstrap

> **Legacy notice:** This document describes the v1 `gh agentic bootstrap` UX
> prototype. The v2 replacement is `gh agentic -v2 init`, which uses a different
> flow and configuration model. This document is retained for historical reference
> only and may not reflect the current v2 implementation.

This document was the authoritative UX reference for the v1 `gh agentic bootstrap`
command. All UI-impacted features (#6, #7, #8, #9) were implemented from this spec.

Prototyped and agreed using `gum` — see `prototype.sh` (v1 only).

---

## Colour palette — GitHub colours

| Role | Hex | Usage |
|---|---|---|
| Primary | `#0969DA` | Headings, prompts, cursors, borders, spinners, URLs |
| Success | `#1A7F37` | ✔ check marks, final summary border |
| Warning | `#9A6700` | ⚠ warnings |
| Danger | `#CF222E` | ✖ errors, failures |
| Muted | `#656D76` | Labels, dividers, secondary text |
| White | `#FFFFFF` | Values, selected items |

---

## Flow overview

```
Banner
  └── Preflight checks
        └── [claude install prompt if missing]
              └── Step 1 — Topology
                    └── Step 2 — Owner
                          └── Step 3 — Project details
                                └── Summary + confirm
                                      └── Execution (steps 3-9)
                                            └── Final summary
                                                  └── Launch Goose
```

---

## Banner

Double border, GitHub blue, centred, 60 chars wide.

```
╔══════════════════════════════════════════════════════════╗
║                                                          ║
║               ⚡ gh agentic bootstrap                    ║
║                                                          ║
║               Agentic Development Framework              ║
║                                                          ║
╚══════════════════════════════════════════════════════════╝

  Initialising new agentic environment...
```

---

## Preflight checks

Section heading in GitHub blue bold. One status line per check, animated with short sleeps.

```
  Preflight checks

  ✔ git found
  ✔ gh found
  ✔ gh authenticated as <username>
  ✔ goose found
  ⚠ claude not found (recommended)

  Install Claude Code now? [Yes] [No]
```

- ✔ in Success green
- ⚠ in Warning amber
- ✖ in Danger red (hard stop)
- Install prompt uses `gum confirm` — Yes highlighted in Primary blue
- If Yes: spinner while installing, ✔ on success
- If No: muted "· Skipping Claude Code — continuing without it"
- If required tool missing and declined: exit with install URL, non-zero exit code

Divider after section: muted `────────────────────────────────────────────────`

---

## Step 1 — Topology

```
  Step 1 — Topology

  How is this project structured?
  > Embedded   — single repo, all-in-one
    Organisation — separate agentic control plane

  ✔ Topology: Embedded
```

- Arrow-key navigable `gum choose`
- Selection confirmed with ✔ in Success green
- Cursor in Primary blue

---

## Step 2 — Owner

```
  Step 2 — Owner

  Where should the repo be created?
  > eddiecarpenter       (personal)
    NewOpenBSS           ✔ clean
    open-bss             ✔ clean
    quarkiverse          ⚠ has repos

  ✔ Owner: NewOpenBSS
```

- Owner list populated live from `gh` API (personal account + orgs)
- Clean orgs show ✔, orgs with existing repos show ⚠
- Cursor in Primary blue

---

## Step 3 — Project details

```
  Step 3 — Project details

  Name › my-project
  Description › A test bench for OCS diameter testing

  Stack
  > Go
    Java — Quarkus
    Java — Spring Boot
    TypeScript / Node.js
    Python
    Rust
    Other

  Antora documentation site? [Yes] [No]
```

- Name: `gum input`, validated (lowercase, hyphens only, no spaces)
- Description: `gum input`
- Stack: `gum choose`
- Antora: `gum confirm`
- All prompts in Primary blue

---

## Summary box

Rounded border in Primary blue, 56 chars wide.

```
  Summary

  ╭──────────────────────────────────────────────────────╮
  │  Topology    Embedded                                 │
  │  Owner       NewOpenBSS                               │
  │  Name        my-project                               │
  │  Description A test bench for OCS diameter testing    │
  │  Stack       Go                                       │
  │  Antora      No                                       │
  ╰──────────────────────────────────────────────────────╯

  Create project? [Yes] [No]
```

- Labels in Muted grey, values in White
- Confirm uses `gum confirm` — Yes highlighted in Primary blue
- No → muted "Aborted.", exit cleanly

---

## Execution steps

Section heading in Primary blue bold. One spinner per step, replaced by ✔ on completion.

```
  Creating your agentic environment

  ✔ Creating repository
  ✔ Removing template files
  ✔ Scaffolding Go project
  ✔ Configuring labels
  ✔ Populating repository
  ✔ Creating GitHub Project
```

- Spinner: dot style, Primary blue
- ✔ in Success green
- ✖ in Danger red on failure — print exact error and exit immediately
- Steps run sequentially — no parallelism

---

## Final summary

"✔ Bootstrap complete" in Success green bold.
Rounded border in Success green, 56 chars wide.

```
  ✔ Bootstrap complete

  ╭──────────────────────────────────────────────────────╮
  │  Repo     https://github.com/NewOpenBSS/my-project   │
  │  Project  https://github.com/orgs/NewOpenBSS/...     │
  │  Clone    ~/Development/my-project                   │
  ╰──────────────────────────────────────────────────────╯
```

- Labels in Muted grey
- URLs in Primary blue
- Clone path in White

---

## Launch Goose

```
  Start Requirements Session

  How would you like to continue?
  > Terminal  — launch Goose CLI in my-project
    Skip      — I'll do it manually
```

- Two options only — no desktop launch (see decision below)
- Terminal → spinner, then launch: `cd <clone-path> && goose session --recipe requirements`
- Skip → print clone path in White, muted instructions

```
  ✔ Goose launched in ~/Development/my-project

  Your agent will read context and begin the Requirements Session.
```

---

## Design decisions

### CLI-only Goose launch

Desktop AI clients (Goose, Claude Code) kill the current active session when
opened with a new workspace path while already running. This is a data loss
risk that applies to all desktop AI clients. CLI is simpler, predictable, and
works in all environments (macOS, Linux, SSH). Desktop launch is not offered.

### GitHub colour palette

The extension is a `gh` extension — GitHub colours feel native and appropriate.
Pink (Charm default) was rejected in prototyping.

### No agent involvement in bootstrap

Steps 3-9 are deterministic Go functions. The AI agent is not invoked during
bootstrap — it is launched at the end by the user's choice (Terminal/Skip).
This eliminates hallucination risk for deterministic operations.
