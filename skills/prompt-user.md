---
name: prompt-user
description: Asks the human a single question through the best available UI primitive — Claude Code's structured AskUserQuestion card when running interactively, or an inline conversation prompt when running headlessly — and returns the reply as a structured value to the calling skill. Use when a skill needs information only the human can supply — intent, choice between options, confirmation, or a free-form value. Use even when the calling skill doesn't say "ask" — phrases like "find out from the user", "confirm with the human", "check what they want", or "let the user decide" should trigger this primitive.
triggers: automated
loads:
  - skills/definitions/error-handling.md
emits-exit-block: false
---

# Prompt User

## Goal

Surface a single question to the human via the richest UI primitive
available in the current Claude Code surface, and return the reply
as a structured value to the calling skill.

## Output Artefacts

- A return value passed back to the caller, of shape:
  ```
  { question: <string>, answer: <string>, cancelled: <bool>,
    selected_option: <string | null> }
  ```
  - `answer` — the verbatim reply (free text) or the selected option's
    label (when options were offered and one was chosen).
  - `selected_option` — the option's label when the human picked from
    a structured list; `null` when the reply was free text.
  - `cancelled` — `true` when the human explicitly declines, supplies
    an empty reply, or selects a "cancel/skip" option; `false`
    otherwise.

No file artefacts. No GitHub state mutation. The conversation
transcript is the durable record.

## Definitions

- `skills/definitions/error-handling.md` — the severity taxonomy
  applied to `MULTI_QUESTION_REJECTED` and `USER_CANCELLED`.

## Dependencies

None as a skill. At runtime this primitive prefers the Claude Code
built-in tool `AskUserQuestion` when it is available; that tool is
not a framework skill and therefore not declared in `loads:`.

## Steps

1. **Receive the question from the caller.** Required:
   - `question` (string) — the question to ask.

   Optional:
   - `options` (list of `{label, description}`) — 2–4 structured
     choices. If supplied, the human picks one rather than typing
     free text.
   - `multiSelect` (bool, default `false`) — only meaningful when
     `options` is supplied.
   - `header` (string ≤ 12 chars, optional) — short label for the
     UI card.

   If the caller passes multiple questions in one call, raise
   `MULTI_QUESTION_REJECTED`. This primitive handles one question
   per invocation so each reply has an unambiguous binding to its
   prompt. The caller loops the primitive instead.

2. **Confirm the session is interactive.** Inspect the available
   tool set: `AskUserQuestion` MUST be present. If it is not, this
   primitive cannot do its job — there is no human at the other end
   of a non-interactive session (`claude -p`, `claude --bare`,
   Goose recipes, CI runners) to reply to a prompt.

   - `AskUserQuestion` is available → continue to step 3.
   - `AskUserQuestion` is NOT available → raise
     `INTERACTION_REQUIRED` with severity `ERROR` and end the skill.
     The caller decides what to do (a skill that legitimately needs
     user input cannot proceed; an automated caller should not have
     been calling `prompt-user` in the first place).

   Detection is by tool registry inspection, not by env var — there
   is no `CLAUDE_CODE_HEADLESS` flag.

3. **Spawn the walk-away reminder.** A 30-second background task
   that fires a desktop notification with sound if the human takes
   longer than 30 seconds to respond — useful when they've walked
   away from the screen.

   ```
   Bash(
     command: "sleep 30 && osascript -e 'display notification \"Claude is waiting for your input\" with title \"Claude Code\" sound name \"Glass\"' >/dev/null 2>&1",
     run_in_background: true,
     description: "30s walk-away reminder for prompt-user"
   )
   → returns { task_id: <id> }
   ```

   Capture the returned `task_id`. The notification command is
   macOS-specific (`osascript`); on Linux or Windows the spawn
   succeeds, `osascript` silently no-ops, and the output redirect
   absorbs any error.

4. **Invoke `AskUserQuestion`.** Build the call from the caller's
   inputs:

   ```
   AskUserQuestion(questions=[{
     question: <caller's question>,
     header:   <caller's header, or a short auto-derived one>,
     options:  <caller's options, or a sensible default pair>,
     multiSelect: <caller's flag, default false>,
   }])
   ```

   If the caller did NOT supply `options` (free-text question),
   pass a minimal default of two options that still steer toward
   a structured answer when possible:

   ```
   options: [
     {label: "Yes / Answer below",  description: "Answer in the conversation"},
     {label: "Cancel / Skip",       description: "Decline to answer"},
   ]
   ```

   The user can always supply free text via the "Other" affordance
   `AskUserQuestion` provides natively.

5. **Cancel the reminder.** Once `AskUserQuestion` returns —
   whether the human answered before or after the 30s reminder
   fired — immediately:

   ```
   TaskStop(task_id: <id from step 3>)
   ```

   Unconditional on every return path. If the task already
   completed naturally (the reminder did fire), `TaskStop` will
   return `"No task found with ID: <id>"` — that is **not a real
   failure**, just the runtime's signal that there is nothing left
   to stop. Handled by the `TASK_STOP_FAILED` rule in Error
   Handling: severity `INFO`, ignore.

6. **Classify the reply and return.** From the `AskUserQuestion`
   tool result:

   - User selected a labelled option → `answer = label`,
     `selected_option = label`, `cancelled = false`.
   - User chose "Other" with free text → `answer = <free text>`,
     `selected_option = null`, `cancelled = false`.
   - User selected the "Cancel / Skip" option (or any caller-
     defined cancel option) → `cancelled = true`, `answer = ""`,
     `selected_option = <label>`. Surface as `WARN` `USER_CANCELLED`.

   Return the structured value to the caller. The caller decides
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
  the `GROUND_TRUTH` entry for `prompt-user`.

## Error Handling

- `MULTI_QUESTION_REJECTED` from step 1 → severity `ERROR`;
  propagate. The caller is misusing the primitive and must loop
  per-question instead.
- `USER_CANCELLED` from step 3a or 3b → severity `WARN`; surface as
  a normal return value (`cancelled: true`). Not an error condition
  for this primitive — the caller decides whether cancellation is
  fatal to its own goal.
- `INTERACTION_REQUIRED` from step 2 (`AskUserQuestion` is not in
  the runtime's tool set — non-interactive session) → severity
  `ERROR`; propagate. There is no human to reply; the skill cannot
  produce its declared artefact. The caller is responsible for
  handling this — automated callers should not invoke prompt-user.
- `ASK_USER_QUESTION_FAILED` from step 4 (the tool was advertised
  as available but its invocation errored at runtime) → severity
  `ERROR`; propagate. Same outcome as `INTERACTION_REQUIRED` from
  the caller's perspective.
- `REMINDER_SPAWN_FAILED` (the background `Bash` call to set up the
  walk-away reminder errored) → severity `INFO`; log and proceed
  without a reminder. The prompt itself is the critical path; the
  reminder is a courtesy. The skill must NOT halt because the
  reminder couldn't start.
- `TASK_STOP_FAILED` (the `TaskStop` call after the human replied
  errored, e.g., task already completed) → severity `INFO`; ignore.
  Worst case the notification fires after the user already
  answered — annoying but not broken.
- All other errors: propagate (default).
