# Skill Frontmatter Schema

Canonical schema for the YAML frontmatter block at the top of every
skill in `skills/`. The mechanical verifier
(`skills/skill-creator/scripts/verify-skill-mechanical.py`) validates against this
schema; skill-creator drafts against it.

This is a schema definition. It does not duplicate the rationale in
`skills/definitions/skill-spec.md` — see the spec for *why* each
field exists.

## Fields

| Field | Type | Required | Notes |
|---|---|---|---|
| `name` | string | yes | Kebab-case `[a-z0-9-]{1,64}`. Must match the filename without `.md`. |
| `description` | string | yes | ≤ 1024 chars. Third-person. Must contain "Use when". Should contain an "even if not explicitly asked"-style assertive clause. See length guidance below. |
| `triggers` | string or list | yes | Machine-readable invocation handle. Common values: `human-interactive`, `automated`, or a comma-separated combination. Must stay consistent with the description's "when" clause. |
| `loads` | list of strings | conditional | Required when the body has any entry under `## Definitions` or `## Dependencies`. Each entry is a repo-relative path. Combined list mirrors body sections. |
| `emits-exit-block` | boolean | optional | `true` when the skill terminates a session by emitting the standard exit block. Defaults to `false`. |
| `exit-hands-to` | string | conditional | Required when `emits-exit-block: true`. Free-form short phrase naming the next actor (`human — …`, `caller — …`, `pipeline — …`). |

No other fields are recognised. Unknown fields are flagged by the
verifier as a hint that either the schema or the skill is wrong.

## Description length guidance

| Skill type | Soft target | Reason |
|---|---|---|
| Primitive | 150–300 chars | Narrow purpose; long descriptions feel inflated. |
| Core skill | 300–600 chars | Broader scope; needs more "use when" coverage to combat under-triggering. |
| Anything > 800 chars | smell | Likely doing too much, or padding. |

Hard maximum (Anthropic): 1024 chars.

## Validation rules used by the verifier

- `frontmatter_required_fields` → `name`, `description`, `triggers`
  must all be present and non-empty. `loads` must be present when
  the body has Definitions or Dependencies entries.
- `frontmatter_name_valid` → `name` matches `^[a-z0-9-]{1,64}$`.
- `description_within_length_limit` → ≤ 1024 chars.
- `description_assertive` → case-insensitive substring match for
  `use when` (or `use this when`).
- `description_third_person` → no first-person (`I `, `I'm`, `my `)
  or second-person (`you `, `your `) tokens at sentence start.
- `references_resolve` → every path in `loads` resolves to a file
  that exists on disk.

## Example

```yaml
---
name: skill-creator
description: Creates a new skill in this framework that conforms to
  the skill-spec — frontmatter, sections, verification, error
  handling, and a minimal evaluation set. Use when the user asks to
  create a skill, wants to formalise a recurring action as a
  reusable skill, or is refactoring an existing skill to match the
  current skill-spec. Use even when the user doesn't explicitly say
  "skill" — phrases like "let's make this reusable", "capture this
  pattern", or "wrap this so we can call it again" should trigger
  this skill.
triggers: human-interactive
loads:
  - skills/definitions/skill-spec.md
  - skills/definitions/skill-frontmatter-schema.md
  - skills/prompt-user/SKILL.md
emits-exit-block: true
exit-hands-to: human — skill ready for use, or returned for further iteration
---
```
