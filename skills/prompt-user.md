---
name: prompt-user
description: Asks the human a single question and returns their reply as a structured value to the calling skill. Use when a skill needs information only the human can supply — intent, choice between options, confirmation, or a free-form value — and cannot infer it from context. Use even when the calling skill doesn't say "ask" — phrases like "find out from the user", "confirm with the human", or "check what they want" should trigger this primitive.
triggers: automated
loads:
  - skills/definitions/error-handling.md
emits-exit-block: false
---

# Prompt User

## Goal

Surface a single question to the human and return their reply as a
structured value to the calling skill.

## Output Artefacts

- A return value passed back to the caller, of shape:
  ```
  { question: <string>, answer: <string>, cancelled: <bool> }
  ```
  `cancelled` is `true` when the human explicitly declines or supplies
  an empty / abort response; `answer` is the verbatim reply otherwise.

No file artefacts. No GitHub state mutation. The conversation
transcript itself is the durable record.

## Definitions

- `skills/definitions/error-handling.md` — the severity taxonomy
  applied to the `INTENT_AMBIGUOUS` and `USER_CANCELLED` outcomes
  below.

## Dependencies

None.

## Steps

1. **Receive the question from the caller.** A single string. If the
   caller passes multiple questions, raise `MULTI_QUESTION_REJECTED`
   — this primitive handles one question at a time so each reply has
   an unambiguous binding to its prompt. Loop the primitive instead.

2. **Surface the question to the human.** Use the assistant turn to
   ask the question directly. Do not pad it with explanation the
   caller didn't supply. Do not embed it inside a longer narrative —
   the human should see one clear question.

3. **Wait for the human's reply.** Do not proceed, do not assume,
   do not invent an answer. The next human turn is the answer.

4. **Classify the reply.**
   - Substantive answer → `cancelled: false`, `answer: <reply text>`.
   - Explicit cancel ("cancel", "stop", "abort", "skip") or empty
     reply → `cancelled: true`, `answer: ""`. Surface as `WARN`
     `USER_CANCELLED` to the caller.
   - Ambiguous reply (the human asked a clarifying question instead
     of answering) → loop back to step 2 with the human's clarifying
     question incorporated. Do not return until a substantive answer
     or cancel arrives.

5. **Return the structured value to the caller.** The caller decides
   what to do with `cancelled: true` — this primitive does not.

## Verification

Run the framework checks against this skill:

```bash
python3 skills/tools/verify-skill-mechanical.py skills/prompt-user.md
python3 skills/tools/check-description-triggers.py skills/prompt-user.md
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

- `MULTI_QUESTION_REJECTED` from step 1 → severity `ERROR`;
  propagate. The caller is misusing the primitive and must loop
  per-question instead.
- `USER_CANCELLED` from step 4 → severity `WARN`; surface as a
  normal return value (`cancelled: true`). Not an error condition
  for this primitive — the caller decides whether cancellation is
  fatal to its own goal.
- All other errors: propagate (default).
