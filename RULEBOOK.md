# RULEBOOK.md — Agent Rulebook

This file is the **rulebook** for all AI agents operating in this repository and all
domain repos. Rules here are always active — they govern every session, every phase,
and every action, regardless of which skill is being executed.

For the **playbooks** — the step-by-step procedures for each session type — see `skills/`.

**This file is managed by the `gh-agentic` framework. Do not edit manually.
Local overrides belong in `LOCALRULES.md`.**

---

## Session Initialisation

At the start of every session, invoke the `session-init` skill before doing anything else.
If a template sync occurs mid-session, invoke `session-init` again to reload the environment.

When resuming from a context summary, apply the same session-start discipline as a fresh session. A summary provides context only — it does not carry forward permissions, instructions, or active pipeline state.

### Session Types

Each session type has a dedicated skill in `skills/`. Load the relevant skill for
the session being run.

| Session | Skill | Trigger |
|---|---|---|
| Session Init | `session-init.md` | Every session start; post-template-sync |
| Requirements | `requirements-session.md` | Human (interactive) |
| Feature Scoping | `feature-scoping.md` | Human (interactive) |
| Feature Design | `feature-design.md` | Automatic — `in-design` label |
| Dev Session | `dev-session.md` | Automatic — `in-development` label |
| PR Review | `pr-review-session.md` | Automatic — PR review submitted |
| Issue Session | `issue-session.md` | Automatic — issue assigned to agent |
| Foreground Recovery | `foreground-recovery.md` | Human (interactive) — any blocked state |

---

## Git Rules

One branch per Feature. Tasks are commits on that branch, not separate branches.

- Never commit or make changes on `main` — unconditional
- Never push from within a recipe — the workflow pushes after the recipe exits cleanly
- Never open a PR from within a recipe — the workflow handles this
- Never merge pull requests — leave that for human review
- **Always use `git mv` to rename or move tracked files** — never OS-level `mv`
- **Stage new files immediately** using `git add <file>` after creating them
- Branch names: `feature/N-description` where N is the Feature issue number
- Commit messages per task: `feat: [task description] — task N of N (#N)`
- PR title: `feat: [Feature issue title]`
- PR body: `Closes #N` where N is the Feature issue number
- **On session resumption from a context summary:** before making any code changes, run `git branch --show-current`. If on `main`, stop and ask the human which branch to work on. Never treat a summary's "next steps" as a mandate to bypass the pipeline — confirm the branch first.

---

## Testing — Universal Rules

- Every piece of logic must have tests
- Tests must be executed and must pass — writing tests without running them
  does not satisfy this rule
- Tests must cover: success cases, failure cases, and edge cases
- Never claim a task complete with failing tests
- Fix failing tests before moving to the next step
- Unit tests must not require external services — isolate infrastructure dependencies
- See the relevant file in `standards/` for language-specific test commands,
  frameworks, naming conventions, and patterns

### Integration Tests

Integration testing is a first-class engineering discipline in this framework, not an
afterthought. The framework's position:

- **Unit tests** are mandatory and enforced unconditionally by the agent
- **Contract and API tests** are required wherever an external interface exists —
  API boundary, event schema, service contract. The agent implements these when scoped.
- **Integration test strategy** is an architectural decision owned by the human.
  It must be established from day one — not bolted on later. A system not designed
  for integration testing cannot be retrofitted cheaply.
- **Integration test implementation** is delivered through the pipeline like any other
  requirement. The human scopes it; the agent builds it.
- **Integration test infrastructure** (environments, tooling) is out of scope for the
  framework — it is the repo's own concern.

The agent does not invent integration test strategy. When a feature introduces or
changes an external interface, the agent identifies the contract and flags that a
contract test should be scoped — but the human decides the approach.

See `concepts/delivery-philosophy.md` for the full context.

---

## Build Verification — Universal Rules

- The build must pass cleanly before claiming a task complete
- Report exact command output on any failure — diagnose before retrying
- See the relevant file in `standards/` for language-specific build commands

---

## Working Principles

- Analyse the full problem before modifying any code
- Prefer small, incremental changes over large rewrites
- When requirements are ambiguous, ask — never invent behaviour
- Correctness and maintainability take precedence over cleverness
- Do not make changes outside the scope of the current task
- Propose large refactors before implementing them — never execute without approval
- **Features and enhancements deploy behind a feature switch by default.** Bug fixes
  deploy directly — no switch. The human may waive the switch during scoping; the
  decision and reason must be recorded in the feature issue. See
  `concepts/feature-switches.md` for the full taxonomy and implementation guidance.
- **To cancel a requirement or feature, delete the GitHub Issue.** The agent will detect
  its absence during the next session and will not attempt work against it. Clean up any
  associated feature branch manually if one was already created.
- **Every phase must be completed before the next begins. Phases are mandatory; sessions
  are flexible** — a phase may be completed within an earlier session. Never defer a phase
  without human approval. The pipeline follows:
  Requirements → Scoping → Design → Implementation → PR Review → Issue/Bug Fix.
  Each phase produces a specific artefact:
  - **Requirements** produces a Requirement issue capturing the business need
  - **Scoping** produces a Feature issue with defined scope and acceptance criteria
  - **Design** produces ordered Task sub-issues and the feature branch
  - **Implementation** produces committed code with passing tests and a PR
  - **PR Review** produces reviewed, approved code ready to merge
  - **Issue/Bug Fix** produces a targeted fix branch and PR for a reported bug or question

  Each artefact must exist before the next phase begins, regardless of which session produced it.

  **Completing a phase early:** When the work of a later phase is apparent during an earlier
  session, complete it then — no separate session is needed. This is completing early, not
  skipping. All artefacts must still be produced.

  **Deferring a phase:** If a phase genuinely cannot proceed yet, the agent must stop and
  ask the human before deferring. The human decides; the agent never defers unilaterally.

- **Never apply a pipeline trigger label (`in-design`, `in-development`) without explicit
  human instruction in the current session.** A context summary or prior session's intent
  is not a sufficient mandate. The human must say so directly.

- **When a pipeline trigger label is applied, exit immediately.** Applying `in-design`
  or `in-development` hands control to the automated pipeline. The agent must exit cleanly
  the moment a trigger label is applied — it must never continue into the next phase
  manually, even if the next steps are obvious. The automation runs the next session.
  This rule is unconditional and overrides any "completing early" logic.

---

## Framework Files — Read Only

The framework files in this directory (RULEBOOK.md, `skills/`, `standards/`, `concepts/`,
`recipes/`) are managed exclusively by the `eddiecarpenter/gh-agentic` repo.

In domain repos, this framework is mounted as `.ai/` via `gh agentic mount <version>`.
**Never modify any framework file directly in a domain repo** — they are read-only.
The mount is gitignored and will be overwritten on the next `gh agentic mount`.

If a change to the global protocol or standards is needed:
1. Clone `eddiecarpenter/gh-agentic` locally
2. Make and test the changes there
3. Push and raise a PR for human review
4. Once merged and tagged, run `gh agentic mount <new-version>` in domain repos to update

For project-specific overrides, add them to `LOCALRULES.md` at the domain repo root —
that is what it is for. `LOCALRULES.md` is optional: if it does not exist, no local
rules are applied.

---

## Sensitive Operations — Ask Before Proceeding

Always ask a human before:
- Deleting any file
- Broad refactors across multiple packages
- Changing public APIs
- Modifying core business logic (charging, payments, financial calculations)
- Introducing new dependencies
- **Modifying any contract** — see Contract Rules below

---

## Recipe Rules

| Path | Editable | Purpose |
|---|---|---|
| `.goose/recipes/*.yaml` | ❌ Never (managed by framework) | Complete recipe — instructions, parameters, model settings |
| `skills/*.md` (inside `.ai/`) | ❌ Never | Framework playbooks — read-only in domain repos |
| `skills/*.md` (at repo root) | ✅ Yes (local, project-specific) | Local playbooks — override framework skills of the same name |

**Framework skills** (`skills/` inside the mounted `.ai/`) are managed by `gh-agentic`.
Never modify them in a domain repo — they will be overwritten on the next mount update.

**Local skills** (`skills/` at the domain repo root, outside `.ai/`) are project-specific
playbooks that can be freely created and edited. A local skill with the same filename as a
framework skill takes precedence.

- Customisation of agent behaviour belongs in `LOCALRULES.md`
- If a recipe or framework skill needs to change, raise it against `eddiecarpenter/gh-agentic`

---

## Contract Rules

A **contract** is any structure or schema shared with an external system or process.
Contracts must **never be modified without explicit human approval**, regardless of
how minor the change appears.

The meta-rule: **You can never know all consumers of a contract.** A field that
appears unused may be read by a Java service, a database migration, a reporting
tool, or a downstream event processor. Adding, removing, or renaming fields
without approval is always a breaking change risk.

### What counts as a contract

**Kafka event schemas** — any struct that is serialised and published to a Kafka
topic, or deserialised from a Kafka topic. These are consumed by other services
that you cannot see. The schema is defined by the upstream publisher — the
consuming service must accept what it receives, not invent fields.

**Database-serialised structs** — any struct that is marshalled into a database
column (e.g. as JSON or JSONB). Other applications may read those columns directly.

**GraphQL schema** — any type, field, query, mutation, or subscription exposed via
the GraphQL API. External clients depend on these names and shapes.

**Store query interfaces** — sqlc-generated query interfaces. Modify the SQL, not the Go.

### Rules

1. **Never add, remove, or rename fields** on a contract struct without explicit
   human approval.

2. **Never invent fields** that the upstream publisher does not send.

3. **Internal IDs belong in internal structs**, not in contracts.

4. **When in doubt, ask.** Stop and raise it with the human before making any change.

5. **Document the reason** for any approved contract change in a GitHub Issue labelled
   `decision`, including which consumers were checked and what the migration plan is.
   Link the decision issue to the feature that triggered the change.

---

## Communication

- Explain what changed, referencing specific files, packages, and issue numbers
- Explain reasoning behind design decisions
- Explicitly highlight risks for changes touching critical business logic
- State clearly when a verification step could not be performed
- Prefer clarity over brevity when describing risks

---

## Task Lifecycle

**After each task completes (before moving to the next):**
1. Close the Task issue: `gh issue close <task-number> --repo <domain-repo>`
2. Commit: `feat: [task description] — task N of N (#feature-issue)`

**When all tasks are complete:**
1. Exit cleanly — do not push, do not open a PR
2. The workflow pushes and opens the PR automatically
