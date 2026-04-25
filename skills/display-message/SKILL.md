---
name: display-message
description: Surfaces a structured message — progress update, intermediate finding, draft for review, or final hand-off report — from the calling skill to the human in a consistent format. Use when a skill needs to show the human something mid-execution or at hand-off. Use even when the calling skill doesn't say "display" — phrases like "let the user know", "show the draft", "report progress", or "hand off the result" should trigger this primitive.
triggers: automated
user-invocable: false
loads:
  - skills/definitions/error-handling.md
emits-exit-block: false
---

# Display Message

## Goal

Render a structured message from the calling skill to the human in a
consistent shape — so the human can recognise at a glance what kind
of update they are reading and which skill produced it.

## Output Artefacts

- A formatted message rendered into the assistant turn of the
  conversation, with:
  - A header line naming the calling skill and the message kind.
  - The body the caller supplied, unmodified.
  - (Optional) a footer block when the caller signals a follow-up
    expectation (e.g., "review and respond", "no action needed").

No file artefacts. No GitHub state mutation. The conversation
transcript is the durable record.

## Definitions

- `skills/definitions/error-handling.md` — the severity taxonomy
  applied to the kinds below.

## Dependencies

None.

## Steps

1. **Receive the message from the caller.** Expected fields:
   - `from` (string) — the calling skill's `name`.
   - `kind` (enum) — one of `progress`, `finding`, `draft`,
     `report`, `handoff`. Required.
   - `body` (string, markdown allowed) — the content. Required.
   - `expects` (string, optional) — short note on what the human
     is expected to do, e.g. `review and respond`, `no action`,
     `decide: continue / iterate`.

   If `kind` is missing or not in the enum, raise
   `KIND_INVALID`. If `body` is empty, raise `BODY_EMPTY`.

2. **Render the header.** Single line, format:

   ```
   [<from> · <kind>]
   ```

   Examples:
   - `[skill-creator · draft]`
   - `[apply-label · report]`
   - `[feature-design · progress]`

   The bracketed prefix lets the human visually scan the
   conversation and locate updates by skill or by kind.

3. **Render the body.** Insert the caller's `body` verbatim under
   the header. Preserve markdown — code fences, tables, lists. Do
   not wrap, summarise, or paraphrase.

4. **Render the footer (when present).** If the caller supplied
   `expects`, append a single line at the end:

   ```
   → expects: <expects text>
   ```

   The arrow signals "next expected human action" without ambiguity.

5. **Surface the rendered message in the assistant turn.** Return
   no value — the message is the output. Control returns to the
   caller immediately; this primitive does not wait for the human.

## Verification

Run the framework checks against this skill:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/display-message/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/display-message/SKILL.md
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
  the `GROUND_TRUTH` entry for this skill (add the entry on first
  run; see `check-description-triggers.py` for the format).

## Error Handling

- `KIND_INVALID` from step 1 → severity `ERROR`; propagate. The
  caller passed an unsupported `kind`; rather than silently render
  with a default, fail visibly so the caller fixes the call site.
- `BODY_EMPTY` from step 1 → severity `ERROR`; propagate. An empty
  display call is almost always a bug in the caller (forgot to
  populate `body` after constructing it).
- All other errors: propagate (default).
