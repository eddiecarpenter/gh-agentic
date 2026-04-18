---
name: skill-creation
description: Produces a correctly-classified, schema-conformant skill file from a human description (reactive mode) or surfaces a proactive suggestion when the agent observes the same substantive action repeated in the current session (proactive mode). Use when the human asks to create a skill that does X, or when the agent notices it has performed three or more substantively-similar actions at a natural pause between user turns.
category: Operation
triggers:
  - human-interactive
  - on-demand
loads:
  - ask-user
  - skill-categories
emits-exit-block: false
exit-hands-to: null
---

# Skill Creation

## Purpose

Convert a human-described capability — or a recurring action the agent
observes within a session — into a new, schema-conformant skill file placed
correctly in either the framework `skills/` directory or the repo's local
`skills/` directory. The skill produced must pass `gh agentic check` and be
invocable by name after the next `session-init` (the catalogue self-heals on
mtime drift; no manual rebuild step).

This skill has two modes:

- **Reactive** — the human explicitly asks for a new skill.
- **Proactive** — the agent detects a repeated substantive action at a
  natural pause, and offers to capture it as a skill.

Both modes terminate in the same write path and emit the same feedback block.

## When to Invoke

- **Reactive:** the human says anything that maps to *"create a skill that
  does X"*, with enough context for classification.
- **Proactive:** at a natural pause between user turns, after the agent has
  performed the same substantive action ≥3 times in the current session (not
  counting retries). Never invoke mid-action.

## Reactive Procedure

Execute these steps in order. Every confirmation, classification,
disambiguation, or collision moment **must** delegate to `skills/ask-user.md`
— do not reinvent interaction shape, option count, or fallback phrasing
inline.

### Step 1 — Elicit the description

If the human's request already contains a clear description, skip this step.
Otherwise, ask for a short description of what the skill should do. Do not
guess. Do not write a file with a placeholder description — if the description
is missing, the skill's purpose is unknown and must be sourced from the human.

### Step 2 — Classify via the rubric

Apply the short decision tree below to classify the requested skill into one
of the six categories defined in `skills/skill-categories.md`.

| Question | Category |
|---|---|
| Does it drive a complete pipeline phase and hand over at the end? | Session |
| Is it a human-controlled escape hatch for a blocked pipeline? | Recovery |
| Is it invoked before other skills to prepare the environment? | Bootstrap |
| Is it inline-only — a rule, template, or pattern read by other skills? | Reference |
| Is its primary output information to surface, not an artefact? | Information |
| Is it a self-contained procedure with a defined input → output? | Operation |

Answer the questions in the order above. The **first** question that answers
*yes* picks the category. The rubric is designed so that Session and Recovery
("does it terminate a session?") are resolved before the lower-ceremony
categories.

**Confidence threshold.** If two categories both plausibly fit, the
classification is uncertain. Do not silently pick one — invoke
`skills/ask-user.md` using Shape 2 (multi-choice selection), presenting the
candidate categories as options with one-line rationales, and let the human
choose. Include an "Other — describe" escape. Do not write the file until the
human has selected a category.

### Step 3 — Elicit missing detail

Before writing anything, verify that every required frontmatter field can be
populated with a real value. Specifically check:

- `name` — derived from the description or asked of the human; must match the
  schema regex in `skills/skill-categories.md`.
- `description` — third-person, trigger-oriented, ending in "Use when …". If
  the human's supplied description is not in this shape, ask for a rewrite
  via `ask-user` Shape 1 (confirm/revise), presenting a proposed description
  for the human to accept or revise.
- `triggers` — concrete values from the schema (`human-interactive`,
  `on-demand`, `automation: <label>`, `session-start`, `post-sync`, …). Ask
  the human to confirm or correct via `ask-user` Shape 2 if ambiguous.

If any field cannot be populated with a real, deliberate value, **stop and
ask** — never fill placeholders, never hallucinate a plausible-but-wrong
purpose, never invent a trigger the human has not confirmed.

### Step 4 — Generate the frontmatter

Populate the frontmatter from the schema in `skills/skill-categories.md`.
Enforce the consistency rules locally before writing:

- `category: Session` or `Recovery` ⇒ `emits-exit-block: true`
- any other category ⇒ `emits-exit-block: false`
- `emits-exit-block: false` ⇒ `exit-hands-to: null`
- `emits-exit-block: true` ⇒ `exit-hands-to` is a non-empty short label

### Step 5 — Generate the body from the category skeleton

Read the matching **Category Skeleton** subsection in
`skills/skill-categories.md`. Copy its body structure, then fill in the
placeholders (`<…>` tokens and `<!-- comments -->`) with real content derived
from the human's description. Do not drop required sections. Do not add
category traits or frontmatter fields the skeleton does not show — the
skeletons are exhaustive per category.

For Session and Recovery skills, the skeleton includes an exit-block
placeholder — fill it in from `skills/session-exit.md`. For Operation skills,
the skeleton includes a feedback-block placeholder — shape as shown.

### Step 6 — Determine placement

Determine the repo identity from the origin remote:

```
git remote get-url origin
```

- If the origin URL resolves to the `eddiecarpenter/gh-agentic` repo (any of
  the common forms — `https://github.com/eddiecarpenter/gh-agentic`,
  `https://github.com/eddiecarpenter/gh-agentic.git`,
  `git@github.com:eddiecarpenter/gh-agentic.git`) → write to the framework
  `skills/` directory at the repo root.
- Otherwise → write to the local `skills/` directory at the repo root
  (**outside** any `.ai/` mount, which is managed and read-only). Create the
  directory if it does not exist.

Never write inside a mounted `.ai/skills/` directory in a domain repo — the
framework mount is read-only and will be overwritten on the next
`gh agentic mount`.

Record the chosen path for the feedback block.

### Step 7 — Handle name collision

Before writing, check whether a file with the chosen `name` already exists at
the target path. If it does, invoke `skills/ask-user.md` using Shape 4 (name
collision), offering:

- Rename — the human supplies a different name; re-run Step 7 with the new
  name.
- Overwrite — write over the existing file; the human has accepted the
  discard.
- Cancel — do not create this skill; abort without writing.

Never silently overwrite. Never offer a default that commits the human to a
destructive action.

### Step 8 — Write the file

Write the file to the target path, staging it immediately:

```
git add <path>
```

Writing is the only step that mutates the repo — every earlier step is
preparation.

### Step 9 — Emit the feedback block

Emit the Operation feedback block described below. Do not emit a session exit
block; this is an Operation, not a Session. Do not invoke
`skills/build-catalogue.md` — the catalogue self-heals on the next
`session-init` via the mtime-based staleness check.

## Proactive Procedure

### Detection

The threshold for surfacing a proactive suggestion is **three or more
substantively-similar actions in the current session**, not counting retries
of a failed action.

*"Substantively similar"* means one of:

- The same tool invocation class with similar parameters — e.g. three
  `gh issue list --label X` calls, each narrowed by a similar filter.
- The same natural-language intent reached through different paths — e.g.
  three instances of *"rename this file and update all imports"*, each
  executed via different mechanics.

Distinct, varied work — different tools, different targets, different intents
— does **not** accumulate toward the threshold. Retries of a failed action
(same tool, same inputs, second attempt after a recoverable error) do **not**
count.

### Timing

Surface the suggestion only at a **natural pause between user turns** — after
the agent has finished a unit of work and is awaiting the next human input.
Never interrupt mid-action, mid-tool-call, or in the middle of an ongoing
dialogue with the human about something else.

### Invocation

At the pause, invoke `skills/ask-user.md` using Shape 3 (yes / not now /
later). Name the observed pattern concretely in the prompt title — e.g.
*"I've noticed I've built the same release-summary query three times this
session. Capture it as a skill?"*

### Reply handling

- **Yes** — run the Reactive Procedure from Step 1, using the observed
  pattern as the initial description. The agent may propose a name,
  category, and description drawn from the observed pattern; the confirm/
  revise flow in Step 3 catches any drift.
- **Not now** — record the pattern as dismissed for the rest of this
  session. Do not raise a proactive suggestion for the same pattern again in
  this session, even if the pattern continues.
- **Later** — do not create a file now. If the same pattern continues to
  accumulate additional substantively-similar actions, the agent may surface
  the suggestion again at a *later* natural pause. "Later" is not a timer —
  it is permission to re-ask, not a commitment to do so.

### Suppression rules

- Under varied work, the threshold is never met — no suggestion is surfaced.
- A single retry of a transient failure does not push the counter from 2 to
  3 — the retry does not count.
- Silence after a prompt does not count as any of the three options. The
  dialogue simply remains open; the agent does not fabricate a default.

## Classification Rubric

Restated here as a standalone reference for quick application — the same
rubric as Step 2, kept local so Step 2 does not have to repeat it.

1. Does it drive a complete pipeline phase and hand over at the end? → **Session**
2. Is it a human-controlled escape hatch for a blocked pipeline? → **Recovery**
3. Is it invoked before other skills to prepare the environment? → **Bootstrap**
4. Is it inline-only — a rule, template, or pattern read by other skills? → **Reference**
5. Is its primary output information to surface, not an artefact? → **Information**
6. Is it a self-contained procedure with a defined input → output? → **Operation**

On low confidence, invoke `skills/ask-user.md` using Shape 2 with the
candidate categories and one-line rationales. Never pick silently.

## Placement Logic

| Origin remote | Target directory |
|---|---|
| Matches `eddiecarpenter/gh-agentic` (any of the common URL forms) | Framework `skills/` at the repo root |
| Any other repo | Local `skills/` at the repo root (create if missing; never inside `.ai/`) |

The chosen path is recorded verbatim in the feedback block so the human sees
exactly where the file landed.

## Name Collision Handling

If a skill with the proposed `name` already exists at the target path,
invoke `skills/ask-user.md` using Shape 4. The three options are **rename**
(default, listed first — the human supplies a new name), **overwrite**
(explicit only; writes over the existing content), **cancel** (abort; no
write). The agent never overwrites silently and never defaults to overwrite.

## Feedback Block

On successful completion, emit the Operation feedback block. The block is
**not** a session exit block — it reports the completion of a one-shot
operation and returns control to the caller (the session continues).

Canonical shape:

```
=== skill-creation — Completed ===

Produced:
  - <path to new skill file> (<category>)

Blocked: none

Next: next session-init will regenerate CATALOGUE.md automatically
```

Variants:

- **Cancelled by human at collision or classification step:**

  ```
  === skill-creation — Completed ===

  Produced:
    - nothing — human cancelled before writing

  Blocked: none

  Next: nothing
  ```

- **Proactive suggestion declined or deferred:**

  ```
  === skill-creation — Completed ===

  Produced:
    - nothing — suggestion <declined | deferred>

  Blocked: none

  Next: nothing (session continues)
  ```

Do not invent a different shape. Do not add decoration.

## Rules

- **Delegate every confirmation, classification, disambiguation, and
  collision moment to `skills/ask-user.md`.** Never reinvent the
  interaction shape, option count, label length, ordering, or fallback
  phrasing inline.
- **Never invoke `skills/build-catalogue.md` directly.** The catalogue
  self-heals during the next `session-init` via the existing mtime-based
  staleness check (see `skills/session-init.md`).
- **Never silently overwrite an existing skill.** A name collision must
  surface via `ask-user` Shape 4 with rename/overwrite/cancel; write only
  after the human's explicit choice.
- **Never write a file with placeholder or empty fields.** Every frontmatter
  field must hold a real, deliberate value sourced from the human or derived
  unambiguously from the description.
- **Never hallucinate a plausible-but-wrong purpose.** If the human's
  description is too vague to classify or to describe, ask — do not invent.
- **Never name a specific tool or harness anywhere in this skill's body or
  in any skill it generates.** The interaction shape lives in
  `skills/ask-user.md`; the mechanism lives in the runtime.
- **Never write inside a mounted `.ai/skills/` directory.** The framework
  mount is read-only in domain repos.
- **Never surface a proactive suggestion mid-action.** Only at a natural
  pause between user turns.
- **Never treat silence as a reply.** If the human does not respond to a
  proactive suggestion, the dialogue stays open — the agent does not default
  to any option.
- **Never push the counter past the threshold using retries.** A retry of a
  failed action does not count as an additional similar action.
- **Reactive and proactive share the same write path.** Once the human
  approves, both modes flow through the same Steps 1–9 — no parallel
  universe of subtly-different skill files.
