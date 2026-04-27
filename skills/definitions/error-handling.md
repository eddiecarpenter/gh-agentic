# Error Handling

The framework's error model: severity taxonomy, default response per
severity, propagation rules, and the relationship between detection
(per-step, defined by the skill author) and handling (framework-wide,
defined here).

## Detection vs handling

Two distinct concerns, often conflated:

- **Detection** — "what could go wrong at this step, and how do I
  recognise it?" Lives in the Steps section, per step. The skill
  author specifies the type and severity of each detectable error.
- **Handling** — "what does the framework do when an error of this
  severity is raised?" Lives here. Skill authors do not redefine
  handling per skill — they inherit the default and only document
  deviations.

The split keeps Steps focused on the work and keeps the response
policy consistent across skills.

## Severity taxonomy

| Severity | Meaning | Default response |
|---|---|---|
| `INFO` | Notable but not problematic. | Log; continue. |
| `WARN` | Something is off; the work continued but a human should be aware. | Log with `WARN:` prefix; continue. |
| `ERROR` | The current step failed. The skill cannot complete its goal under the current attempt. | HALT SKILL; return to caller with error details. |
| `FATAL` | The error invalidates session-level assumptions. No caller should attempt recovery. | HALT SESSION; emit recovery exit block. **Do not catch.** |

Each detection block in a Steps section assigns one of these four
severities to each error type it surfaces.

## Propagation: ripple-up by default

Errors propagate up the call stack by default.

- A skill that does not explicitly handle a callee's error lets it
  propagate to its own caller.
- A skill at the top of the call stack — invoked directly by the
  session's workflow with no calling skill — propagates to the
  session level. An unhandled error there terminates the session.

This means the **default error response is "do nothing special"** —
the framework's halt rules per severity take over automatically.
Skills only need to write Error Handling content when they
*deviate* from the default.

## HALT scopes

| Scope | Meaning |
|---|---|
| `HALT STEP` | Stop this step. The skill may continue with a different step (alternative path, retry). No exit block emitted. |
| `HALT SKILL` | Stop the current skill. Emit the error exit block. Control returns to the caller. If the skill has no caller (top-level), this becomes `HALT SESSION`. |
| `HALT SESSION` | Terminate the session. Emit the recovery-needed exit block. State is preserved for `foreground-recovery`. No further skill invocations. |

`HALT SESSION` is the emergent behaviour when an unhandled error
reaches the top of the stack. Skills do not normally invoke it
explicitly — they raise `FATAL` and let the framework terminate.

## FATAL is sacred

When a callee signals `FATAL`, no caller may catch and continue.
This convention is what preserves session integrity — a caller that
swallows a FATAL has decided unilaterally that recovery is safe,
which it cannot know.

If a skill discovers it can in fact recover from what was previously
classified `FATAL`, the fix is to **reclassify the error** at the
detection site (drop it to `ERROR`), not to catch it upstream.

## Misuse that violates the model

These are conceptual violations of the model — independent of how a
skill happens to be written.

- **Catching `FATAL`.** A `FATAL` is a contract that no caller
  attempts recovery. Catching it removes the contract.
- **Generic catch-all.** "On any error, retry" loses severity
  information and turns the four-level taxonomy into one level.
- **Redefining severities.** The taxonomy is framework-wide; a skill
  that introduces new severities or repurposes existing ones breaks
  the propagation model for every caller.
- **Mixing handling into detection.** Detection (per-step, in Steps)
  identifies errors; handling (this model) decides the response.
  Embedding handling logic into a step's detection block collapses
  the split this model exists to maintain.

The authoring conventions for the `## Error Handling` section of a
skill — how to declare deviations, when to write "None", the
catch-all line — live in the skill spec, not here. This file fixes
the *model*; the spec describes how to *use* it in a skill.
