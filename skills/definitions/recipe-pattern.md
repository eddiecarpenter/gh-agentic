# recipe-pattern.md — Canonical Goose recipe pattern

This document defines the shape of a Goose recipe
(`recipes/<name>.yaml`) and what its `instructions:` block may
contain. It is the authoritative reference loaded by
`skills/recipe-creation/SKILL.md` for templating and linting.

The recipe-creation skill is the playbook (how to author / slim /
audit). This file is the **specification** (what the artefact looks
like).

## The thin-shell rule

A recipe is a glue layer between two systems that already know
everything else:

- **The workflow** detects the event, extracts parameters from the
  payload, and invokes the recipe with parameters bound.
- **The recipe** tells Goose to load the project rulebook and
  execute the skill with those parameters.
- **The skill** does the work — playbook content, gates, error
  handling.

The recipe carries Goose-specific configuration only; everything
else lives in the skill.

> A recipe answers "what runtime do I need?" — extensions,
> parameters, model settings, max turns.
>
> A skill answers "what should I do?" — steps, gates, error
> handling, output artefacts.

When the two overlap, Goose follows the recipe's inline
instructions verbatim and the skill silently drifts. That failure
mode is what this pattern exists to prevent.

## Required YAML structure

A compliant recipe has exactly these top-level keys, in this
order:

```yaml
version: "1.0.0"
title: "<Human-readable session name>"
description: "<As the <agent role>, I want <action> so that <outcome>.>"

parameters:
  - key: <param-name>
    input_type: <string | number>
    requirement: <required | optional>
    default: ""
    description: "<one-liner>"
  # ... one entry per parameter the workflow passes ...

extensions:
  - type: builtin
    name: developer
    display_name: Developer
    timeout: <seconds>
    bundled: true

settings:
  max_turns: <integer>

instructions: |
  <thin-shell instructions block — see below>
```

No other top-level keys are permitted.

### The `description:` field — User Story shape

The `description:` is human-facing — it shows in `goose recipe
list`, in workflow logs, and is what someone reading the repo sees.
It uses the framework's User Story format:

```
"As the <agent role>, I want <action> so that <outcome>."
```

Example: `"As the feature-design agent, I want to decompose a
Feature into ordered Task sub-issues and create the feature branch,
so that the dev-session has everything it needs to run unattended."`

This format is for HUMAN consumption only. The directive given to
the agent at session start is the `instructions:` block (next
section) — that uses an imperative shape with no role-play.

## Standardised parameter vocabulary

A recipe's `parameters:` block uses ONLY names from this canonical
set. Custom parameter names fail the lint.

| Param | Meaning | Type | Used by |
|---|---|---|---|
| `repo` | Active repo in `owner/name` form. Always passed by the workflow. | string | every recipe |
| `requirement_number` | A Requirement issue | number | (when wired to a recipe) |
| `feature_number` | A Feature issue | number | feature-design, dev-session |
| `issue_number` | A non-pipeline issue (`assigned-to-agent`) | number | issue-session |
| `pr_number` | A pull request | number | pr-review-session |
| `tag` | A release tag (e.g. `v1.2.3`) | string | release-notes |

Extending the vocabulary requires updating this document
explicitly. Do NOT add ad-hoc parameter names to recipes.

The workflow always passes `repo`. Skills consume it directly; they
no longer need to fall back to `gh repo view --json nameWithOwner`
when invoked via a recipe.

## Recipes wrap automated / hybrid skills only

A recipe is invoked headlessly by a GitHub Actions workflow. There
is no human in the loop at recipe-execution time.

A recipe may wrap a skill ONLY if the skill's frontmatter
`triggers:` field is `automated` or `hybrid`. Skills with
`triggers: human-interactive` (e.g. `solution-architecture`,
`foreground-recovery`, `requirements-session`,
`requirement-scoping`) cannot be invoked headlessly — they assume
a human at the prompt — and the recipe-creation skill refuses to
wrap them.

If you need to invoke an interactive skill non-interactively,
that's an architectural change to the skill (add a hybrid mode),
not a recipe trick.

## The instructions block — canonical shape

The `instructions:` block is the only place where recipe-vs-skill
discipline is at risk. It MUST follow this floor template and
nothing else.

### Default: single skill

```
Load `AGENTS.md` (the project rulebook). Then execute the
`<skill-name>` skill for <inline parameters>. The skill is the
authoritative playbook; do not improvise.
```

Three sentences, ~3 lines:

1. **Load AGENTS.md** — pulls in RULEBOOK.md, LOCALRULES.md, and
   the Session Bootstrap rule. The agent self-applies the bootstrap
   (which runs `session-init`, which builds the in-memory skill
   index).
2. **Execute the `<skill-name>` skill for <params>** — names the
   skill by NAME ONLY (no path). The agent resolves the name via
   the skill index built during bootstrap. Parameters are woven
   into the directive inline ("for Feature {{ feature_number }}
   in {{ repo }}").
3. **Authoritative playbook; do not improvise** — closes the
   "I-think-I-know-what-this-skill-does" failure mode.

That's it. No identity statement, no "you are running X", no
session bootstrap details, no role/persona, no gh commands, no
git commands.

### Multi-skill variant (rare)

Permitted when one workflow trigger genuinely orchestrates
multiple distinct skills:

```
Load `AGENTS.md` (the project rulebook). Then execute the
`<skill-1>` skill, followed by the `<skill-2>` skill, for
<inline parameters>. The skills are the authoritative playbook;
do not improvise.
```

Rules for multi-skill recipes:

- Order is explicit (`followed by`). No `and` (which leaves order
  ambiguous).
- Plural: "The skills are authoritative".
- A YAML comment at the top of the recipe documents WHY a
  multi-skill chain was chosen instead of a wrapper skill.
- **At 3+ skills**: recipe-creation flags this as "consider a
  wrapper skill instead." Three orchestrated skills almost always
  means there's a higher-level concept that deserves its own
  SKILL.md and would be cleaner to reference 1:1.

### Inputs fallback

When parameters are too many or too-nuanced to inline cleanly
(say, ≥ 3 parameters, or any one needs an explanation longer than
a noun phrase), use a separate `Inputs:` block:

```
Load `AGENTS.md` (the project rulebook). Then execute the
`<skill-name>` skill. The skill is the authoritative playbook;
do not improvise.

Inputs:
  - {{ <param-a> }}: <one-line concept>
  - {{ <param-b> }}: <one-line concept>
  - {{ <param-c> }}: <one-line concept>
```

This is the fallback, not the default. Inline is shorter and reads
better when there's room for it.

### Why these choices

- **No identity / role statement** — the skill names itself in its
  banner; duplicating that here is filler.
- **Skill name only, no path** — the skill index built by
  `session-init` resolves names. Future-proofs against `skills/`
  restructuring.
- **No `gh repo view` resolver** — the workflow passes `repo`;
  the skill consumes it.
- **No reference to `session-init`** — the AGENTS.md bootstrap
  rule mandates it; the agent self-applies.
- **No persona** — voice is implicit in the skill's content.

### Dependency on session-init

This pattern works because `session-init`'s step 2 walks
`skills/*/SKILL.md` and builds an in-memory skill index keyed by
name. By the time the agent reaches "execute the `<skill-name>`
skill", the name resolves to a file path via that index.

If `session-init` ever skips the index walk (e.g., a "headless
fast-path" optimization), this recipe pattern breaks — recipes
would have to revert to full paths. **Do not optimize session-init
in a way that removes the index walk** without simultaneously
updating this pattern doc.

## Anti-patterns (lint flags)

Inside the `instructions:` block, ANY of the following flags the
recipe as non-compliant:

### Numbered step markers
- `^\s*\d+\.\s` — `1. Do X`, `  2. Then Y`
- `^Step \d+` — `Step 1: Do X`
- `^## Step` / `^### Step`

Steps belong in the SKILL.md.

### Inline `gh` commands
Forbidden: any `gh` command, full stop. The recipe receives
parameters; the skill makes the API calls.

### Inline git operations
Forbidden: `git commit`, `git checkout`, `git push`, `git merge`,
`git branch -D`, `git rebase`, etc.

### Decision keywords
Forbidden phrases that imply branching playbook logic:
- `If <condition> then <action>`
- `Branch on <result>`
- `When <condition>, raise <ERROR>`
- `verify the result`
- `loop until passing`

### Sub-headings beyond `Inputs:`
Forbidden:
- `## Steps`
- `## Verification`
- `## Error Handling`
- `## Acceptance Criteria`
- `## Output Artefacts`
- `## State Model`
- `## Definitions`

The only sub-section the canonical shape uses is `Inputs:` (no
`##`), and only as the fallback when parameters can't be inlined.

### `prompt-user` invocations
Forbidden. Interactive prompting is a skill-level concern.

### Skill-internal terminology
Forbidden — references to:
- "anti-fabrication clause"
- "step-skip rule"
- "per-revision diff"
- "impact-delta"
- "commit-discipline"
- "concurrency beacon"
- "T1/T2 cancel"

### Custom parameter names
Forbidden — recipe `parameters:` keys outside the canonical
vocabulary above.

### Full skill paths in instructions
Discouraged — `skills/<name>/SKILL.md` should appear in the
recipe only as the file the human authoring the recipe is
referring to. The instructions block uses skill NAMES, not paths,
to leverage the index. Full paths in instructions trigger a
WARN-level lint (not a hard fail) and a comment to the human
reviewing the recipe.

### Wrapping interactive-only skills
Forbidden — the recipe wraps a skill whose `triggers:` is
exclusively `human-interactive`.

### Persona / role declarations in instructions
Forbidden in the `instructions:` block. The User Story format is
permitted in `description:` only.

## Worked example

A compliant recipe for the `feature-design` skill (which has
`triggers: hybrid`):

```yaml
version: "1.0.0"
title: "Feature Design"
description: "As the feature-design agent, I want to decompose a Feature into ordered Task sub-issues and create the feature branch, so that the dev-session has everything it needs to run unattended."

parameters:
  - key: feature_number
    input_type: number
    requirement: required
    default: ""
    description: "The Feature issue to design."

  - key: repo
    input_type: string
    requirement: required
    default: ""
    description: "The active repo in owner/name form."

extensions:
  - type: builtin
    name: developer
    display_name: Developer
    timeout: 600
    bundled: true

settings:
  max_turns: 100

instructions: |
  Load `AGENTS.md` (the project rulebook). Then execute the
  `feature-design` skill for Feature {{ feature_number }} in
  {{ repo }}. The skill is the authoritative playbook; do not
  improvise.
```

That's the entire recipe. Below `instructions: |` are exactly
three lines of directive: load the rulebook, execute the named
skill with parameters, do not improvise. No steps, no gh commands,
no decisions, no sub-headings.

## What changes go in the recipe vs the skill

When updating the framework, the question "do I update the recipe
or the skill?" is decided by the change type:

| Change type | Update |
|---|---|
| Goose timeout / max_turns adjustment | Recipe |
| New parameter from the standardised vocabulary | Recipe (parameters block) AND skill (consume the new input) |
| New extension Goose needs | Recipe |
| New step / new gate / new validation | Skill ONLY |
| Renamed label / new label transition | Skill ONLY |
| New error code / cancel rule | Skill ONLY |
| Changed `gh` command or git operation | Skill ONLY |
| New persona / voice direction | Skill ONLY (if at all) |
| New parameter NOT in the vocabulary | First update this pattern doc; THEN update the recipe and skill |
| Re-naming a skill | Recipe (the skill name in the directive) AND every other recipe / workflow that references it |

If a change feels like it belongs in the recipe but isn't on this
list, it almost certainly belongs in the skill.
