---
name: session-init
description: Bootstraps the project environment at the start of every session — builds the skill index in memory, runs the gh agentic health check, surfaces the framework state as a greeting, and routes the user to the appropriate phase skill via an interactive menu. Use when a new session starts (RULEBOOK.md mandates this as the first action of every session) or when the human says any of "let's start", "what should we work on", "I'm starting a new session". Use even when the user doesn't explicitly say "session-init" — the bootstrap-and-orient flow is the entry point for any session that hasn't yet been bootstrapped.
triggers: automated
loads:
  - skills/definitions/error-handling.md
  - skills/definitions/skill-frontmatter-schema.md
  - skills/prompt-user/SKILL.md
  - skills/gh-agentic/SKILL.md
emits-exit-block: false
---

# Session Init

## Goal

Bootstrap the project environment at session start so the agent has
the framework's full skill set, current pipeline health, and the
user's stated intent for the session — then route the session to the
appropriate phase skill.

## Output Artefacts

- An **in-memory skill index** built from the frontmatter of every
  file under `skills/*.md` and `skills/definitions/*.md`. The index
  maps skill `name` → `{ description, triggers, loads, file path }`
  and is the basis on which the agent decides which skill to invoke
  for downstream actions.
- A **greeting message** rendered to the user containing the output
  of `gh agentic info` (framework version, project membership, repo
  state).
- One of three exit paths:
  1. **Dispatch to a downstream phase skill** — when the user picked
     a menu option that maps to a known phase.
  2. **Bootstrap-then-answer** — when the user's first message was
     substantive (a question or request); session-init skips the menu
     and the agent goes on to address the user's actual message.
  3. **Clean exit, no dispatch** — when the session is
     non-interactive, when the user declined the menu, or when the
     user picked "Free-form".

No file artefacts. No GitHub state mutation. Side effects are limited
to invoking the chosen phase skill (step 5) and reading repo state.
Framework-version mechanics (mount, upgrade, post-upgrade migrations)
are owned by the `gh agentic` CLI, not by this skill or any session
flow — there is no agent-side post-sync work.

## Definitions

- `skills/definitions/error-handling.md` — the severity taxonomy
  applied to the failure modes detected in step 3, and to the
  `INTERACTION_REQUIRED` downgrade in step 5.
- `skills/definitions/skill-frontmatter-schema.md` — the schema that
  defines what fields the skill-index walk reads (step 2).

## Dependencies

- `skills/prompt-user/SKILL.md` — used in step 5 to present the menu to the
  user. When prompt-user raises `INTERACTION_REQUIRED` (the session
  is non-interactive and `AskUserQuestion` is unavailable),
  session-init catches that error and ends cleanly.
- `skills/gh-agentic/SKILL.md` — used in step 3 to invoke
  `gh agentic check` (and `repair` if needed) and in step 4 to
  invoke `gh agentic info` for the greeting. The gh-agentic skill
  centralises the correct flag choices and the `--raw` token-cost
  contract; session-init defers to it rather than shelling out to
  bare `gh agentic` commands.

Other skills referenced as dispatch targets in step 5 (currently
forward-references — they will be rewritten under the new spec
in subsequent sessions and are not yet present):

- `skills/requirements-session/SKILL.md` (option: add a new requirement)
- `skills/feature-scoping/SKILL.md` (option: scope a requirement)

These are not declared in `loads:` because they don't yet exist on
disk; the framework's `references_resolve` mechanical check would
fail. They will be added to `loads:` when each is rewritten.

## Steps

1. **Show the loading message.** Bootstrap takes a moment — the
   skill-index walk, the `gh agentic check`, and the `gh agentic info`
   render together can run for several seconds, longer if `repair`
   needs to fire. Surface a clear "we're working on it" message to
   the user as the very first agent output, so they know not to
   re-prompt or assume the session is stuck:

   ```
   ## Session bootstrap

   ⏳ AI-Native Software Delivery framework loading — please give it
   a moment to initialise. The full bootstrap will appear here when
   ready.
   ```

   Render this message immediately, before any other step runs.
   This is the agent's first user-facing output of the session.

2. **Build the in-memory skill index.** Walk both `skills/*/SKILL.md`
   (active skills) and `skills/definitions/*.md` (reference
   definitions), reading the YAML frontmatter from each file. Build
   a working-memory dict keyed by `name`:

   ```
   {
     "<skill-name>": {
       "description": <description from frontmatter>,
       "triggers":    <triggers value>,
       "loads":       <loads list>,
       "path":        <file path>,
       "kind":        "skill" | "definition"
     },
     ...
   }
   ```

   Skip any file that fails to parse — record the parse failure as a
   `WARN` and continue. A malformed skill file should not block
   session bootstrap; surface the issue to the user at the end.

   **Verification gate:** the index must be non-empty and must
   contain at least the canonical primitives (`prompt-user`,
   `display-message`) and `skill-creator`. If any of those are
   missing, raise `INDEX_INCOMPLETE` (`ERROR`) — the framework is in
   a broken state.

3. **Run `gh agentic check`.** Verify the repo's pipeline-readiness:

   ```bash
   gh agentic check
   ```

   Branch on the result:

   - All checks pass → continue to step 4.
   - Any check fails → run `gh agentic repair`, then re-run
     `gh agentic check`.
     - Now passes → continue.
     - Still failing → raise `PIPELINE_UNHEALTHY` (`FATAL`). Surface
       the exact failure output to the user; do not proceed with
       greeting or menu. The repo is not in a state where any phase
       skill can run safely.

4. **Display the greeting.** Run `gh agentic info` and surface its
   output to the user as a tidy welcome message:

   ```bash
   gh agentic info
   ```

   Render the result as a markdown block headed `## Session
   bootstrap` summarising the framework version, project ID, repo
   topology, and stack. The intent is "session motd" — orient the
   user before asking what they want to do.

   If `gh agentic info` is unavailable (e.g. the gh-agentic
   extension is not installed in this environment), record a skip
   with reason and continue with a minimal greeting based on what's
   in `LOCALRULES.md` (which is already in context via AGENTS.md
   auto-load).

5. **Present the menu and dispatch — only when the user's first
   message did not declare intent.** Before showing the menu, classify
   the user's first message:

   - **Casual / greeting** ("hi", "hello", "good morning", "let's
     start", an empty message, or anything that does not carry a
     specific question or request) → **show the menu** as described
     below.
   - **Substantive** (a question, a request, a directive — anything
     where the human has already told you what they want) →
     **skip the menu**. The user's message IS their declared intent
     for this session. After the bootstrap greeting from step 4,
     proceed to address the user's actual message. Session-init
     ends without a menu prompt; the conversation continues with
     the user's question as the first thing the agent answers.

   The classification is judgement-based — there is no clean
   pattern match. Lean toward "substantive" when in doubt: a
   redundant menu after the user has spoken is worse UX than a
   menu skipped when they wanted one. The user can always invoke
   `/session-init` later to bring the menu up explicitly.

   When the menu IS shown, invoke `prompt-user` with the four
   phase options:

   ```
   prompt-user(
     question: "What would you like to do this session?",
     header: "Session intent",
     options: [
       {
         label: "Add a new requirement",
         description: "Capture a new business requirement as a Requirement issue."
       },
       {
         label: "Scope a requirement",
         description: "Take an existing Requirement issue and produce a scoped Feature issue."
       },
       {
         label: "Work on existing work",
         description: "Pick an in-flight Requirement or Feature to continue working on."
       },
       {
         label: "Free-form",
         description: "No structured phase — proceed with whatever the user has in mind."
       }
     ]
   )
   ```

   Branch on the user's selection:

   - **"Add a new requirement"** → look up `requirements-session` in
     the index, invoke it, hand off control. Session-init ends.
   - **"Scope a requirement"** → look up `feature-scoping` in the
     index, invoke it, hand off control. Session-init ends.
   - **"Work on existing work"** → query `gh issue list` for open
     Requirements and Features, present them as a follow-up
     `prompt-user` call, dispatch to the appropriate skill based on
     the picked item's labels (Requirement → feature-scoping;
     Feature with `in-design` → feature-design; Feature with
     `in-development` → dev-session). Session-init ends after
     dispatch.
   - **"Free-form"** → no dispatch; session-init ends silently and
     the conversation continues with whatever the user types next.
   - **User chose "Other" with free text** → treat as Free-form;
     surface the free-text reply to the user as confirmation that
     no phase was selected, then end.

   **Headless handling:** if `prompt-user` raises
   `INTERACTION_REQUIRED` (no human at the other end of the
   session), catch it and end session-init cleanly with a one-line
   note. The bootstrap (steps 1–4) has already run; whatever
   automated workflow invoked this session has its own next-action,
   so session-init's job is done.

## Verification

Run the framework checks against this skill:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/session-init/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/session-init/SKILL.md
```

Pass criteria: both commands exit 0.

### Mechanical checks

Run by `verify-skill-mechanical.py`:

- `all_sections_present` — every mandatory section heading exists.
- `frontmatter_required_fields(name, description, triggers, loads)`.
- `frontmatter_name_valid` — kebab-case, matches filename.
- `description_within_length_limit` — ≤ 1024 chars.
- `description_assertive` — contains "Use when" + assertive clause.
- `description_third_person`.
- `references_resolve` — every `loads:` path resolves to a file.

### Ground-truth checks

Run by `check-description-triggers.py`:

- `description_triggers_appropriately` — phrasings classified per
  the `GROUND_TRUTH` entry for `session-init`.

## Error Handling

- `INDEX_INCOMPLETE` from step 2 (the skill index is missing core
  primitives) → severity `ERROR`; propagate. The framework is in a
  broken state and the agent cannot reliably proceed.
- `PIPELINE_UNHEALTHY` from step 3 (`gh agentic check` failed and
  `gh agentic repair` could not fix it) → severity `FATAL`. No
  caller may catch — the agent must not proceed with phase work
  against an unhealthy pipeline.
- `INTERACTION_REQUIRED` from step 5's `prompt-user` invocation
  (the session is non-interactive, no `AskUserQuestion` tool
  available) → severity `INFO`; record the downgrade in the
  conversation transcript and end session-init cleanly. The
  bootstrap has already completed; the menu is interactive-only
  and skipping it in a non-interactive session is expected, not an
  error.
- `MENU_DISPATCH_FAILED` from step 5 (a chosen phase skill is not
  yet present in the framework — currently true for
  requirements-session, feature-scoping, post-sync until they are
  rewritten) → severity `ERROR`; propagate. The user picked a
  phase the framework cannot service; the user can choose
  "Free-form" or wait for the dependency to be rewritten.
- All other errors: propagate (default).
