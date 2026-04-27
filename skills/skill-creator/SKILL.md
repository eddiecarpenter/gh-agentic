---
name: skill-creator
description: Creates a new skill in this framework that conforms to the skill-spec — frontmatter, sections, verification, error handling, and a minimal evaluation set. Use when the user asks to create a skill, wants to formalise a recurring action as a reusable skill, or is refactoring an existing skill to match the current skill-spec. Use even when the user doesn't explicitly say "skill" — phrases like "let's make this reusable", "capture this pattern", or "wrap this so we can call it again" should trigger this skill.
triggers: human-interactive
loads:
  - skills/definitions/skill-spec.md
  - skills/definitions/skill-frontmatter-schema.md
  - skills/definitions/definition-of-done.md
  - skills/definitions/verification-procedure.md
  - skills/definitions/error-handling.md
  - skills/prompt-user/SKILL.md
emits-exit-block: true
exit-hands-to: human — skill ready for use, or returned for further iteration
---

# Skill Creator

## Goal

Create a new skill at `skills/<name>/SKILL.md` conforming to the framework's
skill-spec, including the definitions it consults, the dependencies it
invokes, and a minimal evaluation set sufficient to verify the skill
achieves its goal.

## Output Artefacts

- The new skill file at `skills/<name>/SKILL.md` — body matches the eight-
  section structure required by the spec (`## Goal`, `## Output
  Artefacts`, `## Definitions`, `## Dependencies`, `## Steps`,
  `## Verification`, `## Error Handling` headings present; valid
  YAML frontmatter).
- The new skill's evaluation set at `skills/evals/<name>.json` —
  contains at least 2 test prompts.
- Any new definitions referenced by the skill but not previously
  existing — at `skills/definitions/<name>.md`. Listed individually
  in the hand-off (step 10) so the user knows what was created.
- (Optional) Any new primitive skills extracted during testing —
  at `skills/<name>/SKILL.md`, listed in the hand-off if any.

## Definitions

- `skills/definitions/skill-spec.md` — the canonical standard the new
  skill must conform to.
- `skills/definitions/skill-frontmatter-schema.md` — the frontmatter
  schema the new skill must satisfy.
- `skills/definitions/definition-of-done.md` — the DoD section
  structure used in the new skill's Verification.
- `skills/definitions/verification-procedure.md` — the change-pinning
  rule the new skill's Verification section follows.
- `skills/definitions/error-handling.md` — the severity taxonomy and
  propagation rules the new skill's Error Handling follows.

## Dependencies

- `skills/prompt-user/SKILL.md` — used in step 1 to capture intent through
  the four-question interview when intent isn't already clear from
  the conversation.

Drafts, findings, iteration progress, and the final hand-off are
surfaced as direct conversation messages — the agent talks to the
user. No display primitive needed.

## Steps

1. **Capture intent.** Open `skills/evals/<name>/runs/intent.md` for
   writing — this file IS the deliverable of this step; you build it
   as you go. Fill it with the four answers below, taking each answer
   either from the current conversation (when present and
   unambiguous) or by invoking `prompt-user` once per missing answer:

   ```markdown
   # Captured intent for skill <name>

   ## What should the skill enable the agent to do?
   <answer>

   ## When should it be invoked?
   <specific user phrases, workflow events, contexts>

   ## What is the expected output format?
   <answer>

   ## Is the skill objectively testable?
   <yes — plan test cases | no — qualitative review only>
   ```

   Do not infer answers. Do not skip questions. The step is done
   when the file exists with all four sections populated.

2. **Decide whether to proceed.** Write the placeholder structure
   to `skills/evals/<name>/runs/intent.md` BEFORE doing anything
   else this step:

   ```markdown
   ## Consumers
   <to fill>

   ## Decision: ___
   <to fill — one of "inline (1 consumer)" or "proceed (≥2 consumers)">
   ```

   The placeholder is the artefact contract — the headings exist
   before any decision is made, so the decision *cannot* be reached
   without a file edit.

   Then fill it:

   1. Invoke `prompt-user` with:
      *"Name every consumer of this skill — the contexts (skills,
      recipes, workflows) that will invoke it, including any
      concretely planned within the next 1–2 features."*
   2. Write the user's verbatim answer as a bulleted list under
      `## Consumers`, replacing `<to fill>`. Do not infer; do not
      summarise.
   3. Count the bullets and replace the `___` in the `## Decision:`
      line:
      - 1 bullet → `## Decision: inline (1 consumer)` followed by a
        one-sentence reason. Then surface the recommendation via
        chat, emit `DECLINE_CREATE_SKILL`, and end the
        skill normally.
      - 2+ bullets → `## Decision: proceed (≥2 consumers)`. Continue
        to step 3.

   **Verification gate:** before continuing to step 3 (or before
   emitting `DECLINE_CREATE_SKILL`), run:

   ```bash
   grep -E "^## Decision: (inline|proceed)" skills/evals/<name>/runs/intent.md
   ```

   The output must be a single line matching the pattern. If it is
   empty (the placeholder `___` is still there) or matches more than
   once, fix the file before proceeding.

3. **Draft the skill** at `skills/<name>/SKILL.md` following the eight
   sections in spec order. Each bullet below is a section to write,
   in order, before moving to step 4:
   - Frontmatter — `name`, `description`, `triggers`, `loads` per
     `skills/definitions/skill-frontmatter-schema.md` (the schema
     fixes valid field shapes). The `description` follows the
     **what + when template** — a verb phrase stating what the skill
     does, followed by `Use when <specific triggering conditions>`,
     ending with the assertive clause `even if not explicitly asked`.
     Example: *"Posts a structured comment to a GitHub issue. Use when
     a skill needs to publish text content tied to an issue or PR,
     even if the calling skill doesn't say 'post comment'."*
   - Goal — one or two sentences; what, not how.
   - Output Artefacts — every concrete thing the skill produces, each
     with the observable property (file path, comment header pattern,
     label state, return shape) that identifies it.
   - Definitions — passive references the skill consults.
   - Dependencies — other skills the skill invokes.
   - Steps — imperative voice; one action per step; inline examples
     where useful; per-step detection blocks only when failure modes
     are non-obvious.
   - Verification — change-pinning checks; mandatory.
   - Error Handling — mandatory; explicit "None — default applies"
     when no deviations from the default policy are needed.

4. **Identify and create needed definitions.** List any new
   definitions the draft references that don't yet exist. For each
   candidate, apply the **definition-justification test** — a
   definition is justified when *any one* of the following holds:

   - (a) The content is shared by 2+ skills (or will be after the
     skill in step 3 lands).
   - (b) The content is a canonical schema, template, or rule that
     warrants a single source of truth across the framework.
   - (c) The inline form would exceed ~20 lines and obscure the host
     skill.

   If none apply → inline the content in the host skill; do not
   create a definition file. Otherwise → create at
   `skills/definitions/<name>.md` per the definition kinds (schema,
   template, or rule/procedure).

   For each new definition created, update both:
   - The `## Definitions` section of the new skill — append a bullet
     of the form: `- \`skills/definitions/<name>.md\` — <one-line
     description of what the definition provides>`.
   - The frontmatter `loads:` list — append the same path on its own
     line under the existing `loads:` entries (YAML list format).

   If no new definitions were created, no updates are needed; skip
   to step 5.

5. **Add ground-truth coverage if applicable.** If the new skill has
   any behavioural property worth pinning to a fixed answer key —
   most commonly, that the description triggers on the right user
   phrasings and not on unrelated ones — add an entry to the
   `GROUND_TRUTH` dict in
   `skills/skill-creator/scripts/check-description-triggers.py`:

   ```python
   "<skill-name>": {
       "<phrasing that should trigger>": True,
       "<another that should trigger>": True,
       "<one that should NOT trigger>": False,
       "<another that should NOT trigger>": False,
   },
   ```

   Aim for 4–6 phrasings: at least 2 should-trigger and 2 should-not-
   trigger. Pick phrasings the description specifically claims to
   handle (or specifically disclaims).

   If the skill has no description-trigger property worth pinning
   (rare — most do), skip this step with a recorded reason in
   `skills/evals/<name>/runs/skipped-steps.md`.

6. **Run the framework checks.** Two commands; both must pass:

   ```bash
   python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/<name>/SKILL.md
   python3 skills/skill-creator/scripts/check-description-triggers.py skills/<name>/SKILL.md
   ```

   The mechanical check is deterministic and free. The description-
   trigger check makes one Claude call (~$0.01 of credit). Both
   exit 0 on pass, non-zero on fail; their stdout/stderr is the
   audit trail.

   Save the combined output to
   `skills/evals/<name>/runs/iteration-<N>.results.txt`. The file
   is the artefact proving this step ran.

7. **Review the results file.** Open the latest
   `skills/evals/<name>/runs/iteration-<N>.results.txt`. For each
   failure shown, write one bullet to
   `skills/evals/<name>/runs/iteration-<N>.findings.md`:

   `- **<check>** — <stderr/stdout reason verbatim> → planned fix: <one-sentence action>`

   Failures fall into one of two categories:

   - **`mechanical`** — `verify-skill-mechanical.py` reported a
     structural issue (missing section, malformed frontmatter,
     broken reference). Fix is a file edit to `skills/<name>/SKILL.md`.
   - **`description-trigger`** — `check-description-triggers.py`
     reported a phrasing classified incorrectly. Fix is to tighten
     the description (add a more specific "Use when" clause, or
     adjust the assertive clause to better differentiate from the
     mis-fired phrasing).

   If both checks pass → write exactly the line `no findings` to
   the iteration findings file and continue to step 8. The findings
   file must exist either way before proceeding.

8. **Iterate.** This step has two halves; do them in order, with no
   skipping between.

   **Half A — record the iteration.** Append one row to
   `skills/evals/<name>/runs/INDEX.md`:

   `| <iteration-N> | <ISO-timestamp> | <findings-count> | <converged?> |`

   (Create the file with a header row on iteration 1.) The row is
   the audit trail; it must exist before Half B runs.

   **Half B — branch on convergence.** Convergence has two
   conditions, BOTH must hold:

   1. The latest findings file contains exactly `no findings`.
   2. The latest results file (`iteration-<N>.results.txt`) shows
      both checks reporting PASS.

   The findings file is your interpretation; the results file is
   the raw output from the verifiers. Both must agree before step 8
   is done.

   - **Both conditions hold** → step 8 is done; proceed to step 9.
   - **Either fails** AND iteration count < 5 → apply each fix
     listed in the findings file by **editing `skills/<name>/SKILL.md`
     directly** (or, for description-trigger failures, by adjusting
     the GROUND_TRUTH entry or the description itself). The fix is
     not "noted" — it is "implemented as a file edit". Then **loop
     back to step 6** to re-run the framework checks. This is the
     primary action of the non-converged branch — do not skip to
     step 9.
   - **Either fails** AND iteration count == 5 → halt with
     `FAILED_TO_CONVERGE`. Surface the persistent findings via
     chat and hand back to the user — they decide
     whether to accept partial conformance, escalate, or rework the
     skill. **Do not delete `skills/evals/<name>/`** in this branch;
     leave it in place so the user (or a follow-up session) can
     inspect intent.md, the iteration findings, and the INDEX log
     to diagnose what went wrong.

9. **Hand off.** Surface to the user directly in chat:
   - Path to the new skill.
   - Any new definitions created.
   - Any primitives observed during testing that should be considered
     for future extraction (noted, not extracted now).
   - **Consumer wiring deferred:** for each consumer named in
     step 2's `## Consumers` list, the consumer's `## Dependencies`
     section and frontmatter `loads:` need a back-reference to the
     new skill. This skill does not perform that back-edit (it
     would risk touching stable skills mid-creation). Surface the
     consumer paths and the back-edit instruction so the human (or
     a follow-up session) can complete the wiring.

   Then **clean up the run scaffolding** — the `runs/` folder
   exists only to support skill-creator's own iteration loop and
   has no role after hand-off:

   ```bash
   rm -rf skills/evals/<name>
   ```

   Cleanup is unconditional on success. The exception is
   `FAILED_TO_CONVERGE` (handled in step 8) — that branch leaves
   the run folder in place and surfaces its path for diagnosis;
   step 9 is not reached on that branch.

## Verification

Per `skills/definitions/verification-procedure.md` "Section format".
Skill-specific commands:

```bash
python3 skills/skill-creator/scripts/verify-skill-mechanical.py skills/skill-creator/SKILL.md
python3 skills/skill-creator/scripts/check-description-triggers.py skills/skill-creator/SKILL.md
```

Pass criteria: both commands exit 0.
## Error Handling

- `INTENT_AMBIGUOUS` detected during step 1 → propagate. The user
  must clarify before this skill can proceed.
- `DECLINE_CREATE_SKILL` from step 2 (one-consumer case) → not an
  error; surface the recommendation and end the skill normally.
- `DEFINITION_MISSING` from step 4 (a referenced definition doesn't
  exist and the user elects not to create it) → propagate.
- `CHECK_RUNNER_FAILED` from step 6 (one of the verifier scripts
  itself errors — exit code 2 on the mechanical verifier, or a
  Claude API failure on the description-trigger check) → retry once,
  then propagate.
- `FAILED_TO_CONVERGE` from step 8 (5 iterations elapsed without
  reaching `no findings`) → severity `WARN`; surface the persistent
  findings to the user and end the skill normally. Not an error of
  skill-creator — it is a signal that the skill being created has a
  defect skill-creator cannot fix unattended.
- All other errors: propagate (default).
