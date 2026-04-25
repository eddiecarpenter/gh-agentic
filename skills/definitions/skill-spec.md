# Skill Spec

The standard for writing skills in this framework.

## Purpose

Skills under this spec have a predictable shape, testable behaviour,
and a consistent error model. The spec exists because the previous
skill layer had become archaeology — rules accumulated over time as
responses to specific failures, with no coherent structure. The result
was skills with rationale mixed into procedure, duplicated invocation
conditions, verbose padding, and silently-skipped steps.

Skills written under this spec follow the same structural shape
regardless of what they do, making them reviewable by humans,
verifiable mechanically, and learnable by agents with minimal prior
context. The archived skills at `skills/archive/` remain as historical
reference but are not canonical.

## Repository Layout

Four top-level locations matter for the skill ecosystem:

```
skills/
├── (active skills go here as flat .md files)
└── definitions/
    └── (canonical schemas, templates, and component definitions
         that skills load on demand)

concepts/
└── (framework concepts and design rationale)

docs/
└── (project information and operational guides)
```

- **`skills/`** — active skills, executed by agents, written for agent
  consumption but human-readable.
- **`skills/definitions/`** — formal canonical sources that skills load.
  Schemas, templates, rules (e.g., what a Definition of Done section
  must contain, what change-pinning verification means, the severity
  taxonomy for errors). This file (the skill spec) is itself a
  definition — it defines the rules every skill must follow, and is
  loaded by skill-creator. Loaded via the `loads:` frontmatter field.
- **`concepts/`** — framework concepts and design rationale. The "why"
  documents. Agents may read for context; do not execute.
- **`docs/`** — project information and operational guides for humans
  (App setup, project brief, architecture overview).

There is no separate top-level `definitions/` — definitions are nested
under `skills/` because they exist to serve skills.

The split between `concepts/` and `docs/` is deliberate but its
boundary is sometimes fuzzy. When in doubt, place design philosophy
and framework ideas in `concepts/`; place operational instructions and
project metadata in `docs/`.

## Universal Structure

Every active skill must have these sections, in this order:

1. **Frontmatter** (YAML metadata)
2. **Goal**
3. **Output Artefacts** (what the skill produces)
4. **Definitions** (schemas, templates, rules the skill consults)
5. **Dependencies** (other skills the skill invokes)
6. **Steps**
7. **Verification**
8. **Error Handling** (mandatory; may state "none" when default applies)

All eight sections are always present. The conceptual difference
between Definitions and Dependencies is *how* the loaded content is
used — passive reference vs active invocation — not whether it gets
loaded.

## Frontmatter

YAML block at the top of every skill. The canonical schema lives in
`skills/definitions/skill-frontmatter-schema.md`.

### `description` is the discovery signal

At session start, **only the frontmatter (name + description) of each
skill is pre-loaded** into the agent's context. Skill bodies are read
on-demand when the skill is invoked. This means:

- The agent selects a skill based on the description alone. If the
  description is weak, the skill will not be discovered.
- Body content does not help the agent decide whether to use a skill.
  It only helps once the skill is already being used.

The description must state both **what the skill does** and **when to
use it**. Template:

```
<verb phrase describing what the skill does>. Use when <specific
triggering conditions, including key terms the task would mention>.
```

Third person, always. No first-person ("I can...") or second-person
("You can use this to..."). Third-person is how the description reads
once injected into the system prompt.

### Description length

Anthropic's hard maximum is 1024 characters. Soft targets within that
limit:

| Skill type | Soft target | Why |
|---|---|---|
| Primitives | 150–300 chars | Narrow purpose; description should be terse and pointed. Overly long descriptions on small skills feel inflated. |
| Core skills | 300–600 chars | Broader scope; needs more "use when" coverage to combat under-triggering. |
| Anything > 800 chars | smell | Likely trying to do too much or padding the description. Consider: is this one skill, or two? |

Length is determined by what the description must accomplish:

- State what the skill does.
- State when to use it (with key terms the task would mention).
- Be assertive ("even if not explicitly asked") to combat
  under-triggering.

Hit those three; nothing more. If the description is short and still
hits all three, that's better than long. If it can't hit all three
within the soft target, the skill may be doing too much.

### Other fields

- `name` — machine identifier; kebab-case; matches the filename.
- `triggers` — machine-readable invocation handle for workflows. For
  workflow-driven skills (invoked automatically by CI on label events,
  session start, etc.) this is how the workflow knows which skill to
  run. Must stay consistent with the description's "when" clause.
- `loads` — other skills/definitions this skill reads. Mirrors the
  Dependencies section in the body.
- `emits-exit-block`, `exit-hands-to` — lifecycle fields for skills
  that terminate sessions. Omit when not applicable.

There is no `category` field. Skill properties are declared explicitly
per-skill rather than derived from a category enum.

## Goal

One or two sentences. What the skill accomplishes — not how, not why,
not when. Just the outcome.

Good:

```
Decompose a Feature issue into ordered Task sub-issues, publish a
Design Plan comment on the Feature, and create the feature branch.
```

Bad (too vague):

```
Handles feature design.
```

Bad (too long; includes history and sequencing):

```
This skill is used during the design phase of the pipeline to take a
Feature issue that has been scoped and break it into tasks that can
then be worked on by the dev session which runs after this one.
```

The Goal is what you would say if someone asked "what does this skill
do?" in one breath.

## Output Artefacts

Concrete things the skill produces. Each artefact has a recoverable
identifier — a file path, a GitHub issue/comment/PR number, a branch
name, a label state, or similar.

This section makes the skill's deliverables explicit. Without it,
"what this skill produces" lives implicitly in the Steps and
Verification, where it can drift or be missed. Explicit declaration
forces the author to enumerate the deliverables, and gives Verification
an unambiguous list of things to check for presence.

Common artefact kinds:

- **Files** — `docs/<name>.md`, `skills/<name>.md`, etc.
- **GitHub state** — comments on an issue, sub-issues, labels, branches,
  PRs, project status changes.
- **In-memory responses** — a structured value returned to the caller
  (typical for primitives like prompt-user).

Example:

```markdown
## Output Artefacts

- A Design Plan comment on the Feature issue — body matches `^## Design Plan`.
- Sub-issues under the Feature — one per planned Task; each labelled `task`.
- Feature branch — name matches `feature/<N>-*`; one scaffolding commit pushed to origin.
- Label state on the Feature — `in-development` present; `in-design` absent.
- Project status — Feature card moved to "In Development".
```

Each entry names the artefact and the **observable property** that
identifies it (header pattern, label name, branch pattern). The
Verification section then has a check per artefact.

For skills that produce no concrete artefacts (rare — typically
analysis-only primitives), state this explicitly:

```markdown
## Output Artefacts

None — this skill is read-only and produces no observable side effects.
Its return value is the analysis result conveyed in the calling agent's
context.
```

The explicit "None" makes the absence visible (rather than ambiguously
omitted).

## Definitions

Schemas, templates, and rules the skill **consults** during execution.
Definitions are passive reference — they tell the skill what valid
inputs look like, what format outputs must take, or what rules apply
when specific situations arise.

Three kinds of definitions:

- **Schemas** — define what valid X looks like (e.g., the YAML
  frontmatter schema)
- **Templates** — define the format/shape the skill must produce
  (e.g., the Design Plan comment shape)
- **Rules / Procedures** — define how the skill must behave in
  specific situations (e.g., change-pinning verification, error
  severity handling)

All definitions live in `skills/definitions/` and are loaded into the
agent's context when the skill is invoked. They remain available for
the duration of the skill's execution.

Example:

```markdown
## Definitions

- `skills/definitions/design-plan-template.md` — the format of the
  Design Plan comment this skill publishes.
- `skills/definitions/verification-procedure.md` — the change-pinning
  rule the Verification section below must follow.
- `skills/definitions/error-handling.md` — the severity taxonomy and
  default response for errors detected in the steps below.
```

State only the path and a short note on what the definition provides.
Don't restate the definition's content — the agent reads it from the
file when needed.

## Dependencies

Other skills this skill **invokes** as part of its procedure —
typically primitives that handle common operations (posting comments,
prompting the user, applying labels). Dependencies are active calls;
the skill calls them from within its steps.

Like definitions, dependencies are loaded at skill invocation and
remain available throughout the skill's execution. The conceptual
difference is invocation pattern, not loading mechanism.

Example:

```markdown
## Dependencies

- `skills/post-issue-comment.md` — used in step 4 to publish the
  Design Plan comment.
- `skills/apply-label.md` — used in step 7 to transition the Feature
  to `in-development`.
```

Inline references inside Steps may use bare names without paths once
Dependencies has named the full path.

### Frontmatter mirror

Both Definitions and Dependencies are mirrored in the `loads:`
frontmatter field as a single combined list. The body sections are
the human-readable canonical source; the frontmatter is for machine
parsing. The body distinguishes the two purposes; the frontmatter
treats them uniformly because the loading mechanism is the same.

## Steps

A numbered list of the agent's actions, in order.

### Form

- **Imperative voice.** "Read the Feature body." — not "You should read..."
  or "The agent reads...".
- **One action per step.** If a step reads more like two actions, split it.
- **Inline examples** where the action benefits from a concrete
  illustration. Examples belong with the step they support, not in a
  separate section.
- **Explain why** when the action is non-obvious. Don't just say "Do X";
  say "Do X because Y."

### Detection — per-step error awareness

Each step that can fail declares what to check for. The author knows
what errors are possible at the step; list them with type and severity:

````markdown
4. Publish the Design Plan comment:

   ```bash
   gh issue comment ${ISSUE} --body-file plan.md
   ```

   **Detect:**
   - Exit code non-zero → `FATAL` type `PUBLICATION_FAILED` — the API
     call did not succeed; session cannot proceed.
   - Response body empty after 200 OK → `ERROR` type
     `UNEXPECTED_EMPTY_RESPONSE` — propagate to caller.
````

Detection is local to each step. Handling is not specified in Steps —
it is defined once in `skills/definitions/error-handling.md` and
propagates up the call stack per the ripple-up model (see Error
Handling section below).

### Inline branching

When a step has a specific alternative path (branch-on-condition, not
a general error response), include the branch inline:

```markdown
4. Check if the feature branch exists on origin:

   ```bash
   git ls-remote --heads origin feature/${ISSUE}-*
   ```

   - If it exists → skip to step 7 (branch is ready).
   - If it does not exist → continue to step 5.
```

Alternative paths belong in the step. General error responses belong
in the error-handling definition.

## Verification

Mechanical checks that prove the skill completed its work correctly.
Every check must be **change-pinning**: it must fail if the change is
reverted.

Weak (existence check):

```bash
test -f docs/github-app-setup.md
```

Strong (change-pinning):

```bash
grep -qF "Every autonomous phase must publish its plan" RULEBOOK.md
```

The first passes whether or not the content is correct. The second
fails if the specific addition is removed. The rule: **the
verification command must fail if the change described by this step
is reverted or materially modified.**

The canonical rule, failure semantics, and examples live in
`skills/definitions/verification-procedure.md`. Skills reference this
definition rather than restate it.

### Verification is mandatory

Every active skill has a Verification section with at least one
change-pinning check. Verification is how the agent self-confirms
"done" — without it, the skill's Definition of Done is declaratory,
not enforceable.

Skills that produce only side effects (e.g., posting a comment with
no repo change) still need verification — the check simply queries
the side effect ("the comment exists on the issue with the expected
header").

### Verification covers every output artefact

Every artefact named in the Output Artefacts section has a
corresponding presence check in Verification. The Output Artefacts
section is the source of truth for *what* the skill produces;
Verification proves *that* each was produced.

Without this discipline, a skill could declare an artefact in Output
Artefacts but skip producing it during execution, and Verification
wouldn't catch the omission. Explicit one-to-one coverage prevents
the gap.

### Verification has two layers: mechanical and ground-truth

Verification is the *skill writer's* tool for self-checking the
skill — it is not a quality grader for the skill's runtime output.
Quality of artefacts produced when the skill runs is enforced by
the skill's own internal verification gates (per-step) and by the
surrounding pipeline gates, not by an external rubric over the
skill text.

The skill writer specifies two kinds of checks:

| Layer | What it checks | How it runs |
|---|---|---|
| **Mechanical** | Structure, schema conformance, references resolve — deterministic | `skills/tools/verify-skill-mechanical.py <skill-path>` |
| **Ground-truth** | Specific behavioural assertions with a pinned answer key (e.g., "this description triggers on these phrasings, not those") | `skills/tools/check-description-triggers.py <skill-path>` and similar narrow scripts |

Both run as plain Python scripts. There is no orchestration layer,
no recipe, no rubric grader. Checks pass deterministically (or
near-deterministically — ground-truth uses an LLM call but the
assertion is binary against a fixed answer key, which is stable
across runs).

The change-pinning rule applies to both layers: a check must fail
when the change it covers is reverted. Vague rubrics that
"sometimes pass" are not allowed.

### Declaring checks in a skill's Verification section

Use sub-sections to distinguish the two kinds. Each skill names
the checks that apply to it; the runner is the appropriate Python
script.

```markdown
## Verification

Run the framework checks against this skill:

```bash
python3 skills/tools/verify-skill-mechanical.py skills/<name>.md
python3 skills/tools/check-description-triggers.py skills/<name>.md
```

Pass criteria: both commands exit 0.

### Mechanical checks

Run by `verify-skill-mechanical.py`:

- all_sections_present — every mandatory section heading exists.
- frontmatter_required_fields(name, description, triggers, loads).
- frontmatter_name_valid — kebab-case, matches filename.
- description_within_length_limit — ≤ 1024 chars.
- description_assertive — contains "Use when" + assertive clause.
- description_third_person.
- references_resolve — all paths in `loads:` exist.

### Ground-truth checks

Run by `check-description-triggers.py` (and any other narrow ground-
truth scripts the skill writer adds):

- description_triggers_appropriately — the description correctly
  classifies the phrasings pinned in the script's GROUND_TRUTH dict
  for this skill.
```

The mechanical checks above are universal — every skill gets them.
A skill is exempt from the ground-truth check only if it has no
behavioural property worth pinning (rare). Adding a new ground-
truth check means appending an entry to the appropriate Python
script's answer-key dict; no separate config files.

### Verification serves dual purpose

The Verification section serves two roles:

1. **In execution** — the agent runs the mechanical checks before
   declaring the skill done. Failure halts the skill (per the
   Error Handling default).

2. **In testing** — the test framework runs the skill on test
   inputs, then runs the skill's own Verification commands as the
   fidelity test. The same checks that prove "done" in production
   prove "the test scenario succeeded" in testing.

This dual role is why mechanical checks must be change-pinning and
semantic checks must catch intent drift — they are the contract for
both the agent's self-confirmation and the test framework's
fidelity assessment.

## Error Handling

Mandatory section. Forces the author to consciously consider what
errors the skill might encounter and how they should be handled.

When the skill needs no special handling beyond the framework default,
the section explicitly states this:

```markdown
## Error Handling

None — default error handling applies. All errors detected in the
steps above propagate to the caller per the rules in
`skills/definitions/error-handling.md`.
```

An empty or absent section is ambiguous (did the author skip this, or
decide it wasn't needed?). The explicit "None" makes the decision
visible.

### Default error policy

Most skills inherit the default policy from
`skills/definitions/error-handling.md`:

| Severity | Default response |
|---|---|
| `INFO` | Log, continue. |
| `WARN` | Log with warning prefix, continue. |
| `ERROR` | Halt skill, return to caller with error details. |
| `FATAL` | Halt skill, return to caller with "do not catch" marker. |

Errors **propagate up the call stack** by default. A caller that does
not explicitly handle a callee's error lets it propagate. Unhandled
errors at the top of the call stack halt the session.

### When the skill deviates from default

When the skill catches specific callee errors for retry, alternative
paths, or escalation, list them explicitly:

```markdown
## Error Handling

- `API_RATE_LIMIT` from any gh API call → sleep 30s, retry once; on
  continued failure, propagate.
- All other errors: propagate (default).
```

The "All other errors: propagate (default)" line makes the catch-all
behaviour explicit even when the skill only handles a few cases.

### HALT scopes

- **HALT STEP** — stop this step; the skill may continue with a
  different step (alternative path, retry). No exit block emitted.
- **HALT SKILL** — stop the current skill. Emit the error exit block.
  Control returns to the caller. If the skill is top-level (invoked
  directly by the session's workflow, no caller), HALT SKILL becomes
  HALT SESSION.
- **HALT SESSION** — terminate the session. Emit the recovery-needed
  exit block. No further skill invocations. State is preserved for
  foreground-recovery. This is the emergent behaviour when an
  unhandled error reaches the top of the call stack.

## Execution Discipline

Rules that apply to *any* skill being executed by an agent.

### Skip-with-reason

When executing a skill, if you decide not to perform a step as
written, you must, BEFORE proceeding to the next step:

1. Record the step number and the verbatim step heading.
2. Record a one-or-two-sentence reason explaining *why the step does
   not apply in this run*.
3. Append both to a `skipped-steps.md` file in the skill's run
   artefact directory (or, if the skill produces no run directory,
   to the conversation transcript with a `[SKIP]` prefix).

The reason must be specific to the current execution context (e.g.,
"step 4 doesn't apply because no new definitions were created in
step 3"). The following are *not* sufficient reasons:

- "I judged it unnecessary."
- "The step seemed optional."
- "I covered it implicitly in step N."

The discipline is intentional friction. Forcing the agent to
articulate a reason for skipping often surfaces the realisation
that the step does, in fact, apply. Skips that survive the
articulation test are real and documented; skips that don't
survive get performed instead. Either outcome is acceptable; the
silent skip is not.

This rule is framework-wide and overrides any per-skill instruction
that might suggest "skip this step if obvious".

## Writing Style

Cross-cutting principles that apply across all sections of a skill.

### Explain why, not MUSTs

When a rule matters, state the reasoning. LLMs follow rules better
when they understand them; heavy-handed MUSTs without context often
get ignored under execution load.

Weak:

> **ALWAYS** post the Design Plan comment before creating tasks. **MUST NOT** skip.

Better:

> Post the Design Plan comment before creating tasks. This is the
> irreversibility boundary of the design phase — everything before it
> is analysis; everything after it mutates GitHub state that must
> align with the published plan. Skipping the publish step breaks the
> audit baseline PR review relies on.

The second version explains what breaks if the rule is violated. The
LLM can reason about the rule rather than just memorise it. Reserve
all-caps imperatives for genuinely non-negotiable constraints.

### Theory of mind

Imagine an LLM reading each sentence. What would it misunderstand?
What might it interpret too loosely? What step is it likely to skip
under momentum? A skill that anticipates these failure modes in its
wording is more robust than one that relies on explicit prohibitions.

### Concise above all

Claude is smart. Don't explain what a PDF is, what markdown is, what
git is. Every token competes with conversation history once the skill
is loaded. Each paragraph should earn its token cost.

Assume Claude knows everything a reasonably-experienced engineer
would know. Only state what is specific to this skill or this framework.

### Push the description

Claude tends to under-trigger skills — it does not invoke a skill
even when one applies. Counter this by writing the description
assertively. End with patterns like "Use when the user mentions X, Y,
or Z, even if they don't explicitly ask for it." The "even if they
don't explicitly ask" clause is particularly effective at lifting
under-triggered skills.

### Consistent terminology

Pick one word for each concept and use it throughout. Not "endpoint /
URL / path / route" for the same thing; not "extract / pull / retrieve
/ get". Mixed terminology makes the agent think they are different
things.

## Primitives Library

Small, reusable skills that handle common operations. Core skills
orchestrate; primitives handle the consistent *how* for recurring
actions.

### Likely groupings

- **Output to user** — display-message, show-progress, notify-completion
- **Input from user** — prompt-user, confirm-action, select-from-list
- **GitHub interactions** — post-issue-comment, apply-label,
  transition-issue-state
- **Verification** — verify-file-exists, verify-comment-exists,
  change-pinning-grep
- **Workflow control** — halt-with-diagnostic, emit-exit-block

### When something earns being a primitive

A primitive is justified when **reuse exists** — two or more
consumers, real or concretely imminent in the next 1–2 features.
Reuse is the primary gate. Consistent output format and centralisable
nuance are amplifiers when reuse exists; they do not justify
separation on their own.

For exactly one consumer:

- **Default to inlining** the logic in the consumer skill.
- **If the logic deserves separation** for clarity or testing,
  **bundle it as a helper script** next to the consumer skill
  (Anthropic's pattern), not as a separate skill.
- **Extract to a skill when the second consumer arrives.**

This protects against speculative decomposition — the failure mode
that produces many small skills none of which justify their discovery
cost.

### Reasons that sound stronger than they are

- **Cognitive load** — smaller is easier to understand, but this is a
  corollary of focus, not of separation. A focused inline section in
  a larger skill is also easy to grasp.
- **Independent versioning** — real for libraries, rare in our
  framework where everything moves forward together.
- **Substitution / swappability** — real in theory, rare in practice.
  Most primitives never have alternative implementations.

### A primitive is not justified when

- The behaviour is a single shell command with no nuance (e.g.,
  `git branch --show-current`).
- Exactly one skill uses it (inline or bundle).
- The agent already knows how to do it reliably without guidance.

### Extraction signal: repeated work across test cases

When evaluating a skill, read the test runs. If independent test cases
all produce similar helper logic (same script, same sequence of API
calls, same branching), that is a strong signal the logic belongs in
a primitive. Extract, put it in `skills/`, have the core skill load
it.

Primitives emerge from observed repetition, not from upfront
enumeration.

## Evaluations

Every skill ships with at least 2–3 realistic test prompts — the kind
a real user would actually send. Evaluations are how the skill is
validated against reality rather than imagination.

### Structure

Each prompt has:

- The input the user would provide.
- The expected output or behaviour. Qualitative description is fine;
  assertions are preferred for objectively verifiable outputs.

The exact file location for test prompts is to be finalised (likely
`skills/evals/<skill-name>.json` or a co-located file).

### Comparison

Where possible, run **with-skill** and **without-skill** versions of
each prompt. The delta between them is the skill's actual
contribution. A skill that does not measurably improve outcomes over
the baseline does not earn its context cost.

### Iteration

- Run the test prompts.
- Review outputs qualitatively and (where assertions exist)
  quantitatively.
- Adjust the skill based on observed agent behaviour — not on
  assumptions about how the agent *should* behave.
- Repeat until the skill meets its goal reliably across test cases.

### Evaluation-driven development

Write the test cases before or alongside the skill itself. This
ensures the skill solves real problems rather than documenting
imagined ones.

## Anti-Patterns

Things to avoid when writing a skill.

### Duplicated invocation conditions

Stating "when to invoke this skill" in multiple places:

- `description:` frontmatter (the canonical discovery signal)
- `triggers:` frontmatter (the machine-readable workflow handle)
- A `## When it Runs` or `## When to Use` body section (redundant)

The description and triggers are the only places invocation conditions
live. Body sections describing "when" drift out of sync and give
three sources of truth for one fact.

### README-creep

Embedding project history, design rationale, or forward-looking
commentary in the skill body. A skill contains only what an agent
needs to perform the work. History, rationale, and philosophy belong
in `concepts/` documents that link to the skill.

### Verbose padding

Explaining concepts the agent already knows. "PDFs are a document
format..." "Markdown is a lightweight markup language..." "The `gh`
CLI provides access to GitHub's API..." — all tokens that cost
context without adding value.

### Nested references

`SKILL → advanced.md → details.md → actual info`. Keep references one
level deep from the skill. If a reference file exceeds ~100 lines,
add a table of contents at the top so partial reads still surface the
structure.

### Heavy MUSTs without explanation

Writing **ALWAYS**, **NEVER**, **MUST NOT** as the primary enforcement
mechanism. LLMs follow reasoned rules better than commanded ones.
Reserve all-caps imperatives for genuinely non-negotiable constraints;
explain the reasoning behind everything else.

### Categories driving structure

The previous framework imposed a six-category skill taxonomy that
drove structure. This spec deliberately has no categories — properties
that need to be declared are declared explicitly in frontmatter. A
skill is a skill.

### Catching FATAL errors

When a callee signals FATAL severity, it means "no caller should
attempt recovery." A skill that catches and continues past a FATAL
error violates the convention that preserves session integrity.

### Under-triggering descriptions

Descriptions that are technically accurate but too neutral to signal
when to invoke. Descriptions should be assertive — "Use when the user
mentions X, Y, or Z, even if they don't explicitly ask for it" — not
timid.

### Silent step-skipping

A skill whose steps describe "do X, then do Y" with no mechanical
enforcement that X was actually done. Under load, LLMs skip steps
that have no downstream verifier. Every step whose output is required
by later steps should have either (a) a detection block that would
surface the miss, or (b) a verification check downstream that would
fail if the step had been skipped.

## Process

How skills get written, tested, and refined.

### 1. Capture intent

Before drafting, answer four questions:

- What should this skill enable the agent to do?
- When should it be invoked? (specific user phrases, workflow events,
  contexts)
- What is the expected output format?
- Is this skill objectively testable? If yes, set up test cases. If
  no — subjective outputs — skip formal assertions and rely on
  qualitative review.

### 2. Draft

Write the Frontmatter, Goal, Dependencies, Steps, Verification. Apply
the writing style. Keep it concise.

### 3. Test

Create 2–3 realistic prompts a user would actually send. Run them
against the skill (with-skill vs baseline where possible). Collect
outputs.

### 4. Review

Inspect outputs. Look for:

- Steps the agent skipped despite being instructed.
- Silent error swallowing.
- Repeated helper logic across test cases (primitive extraction
  candidate).
- Ambiguous wording the agent misinterpreted.
- Under-triggering (skill did not activate when it should have).

### 5. Iterate

Refine based on observed behaviour, not assumptions. If multiple test
cases surface the same issue, generalise the fix rather than patch
each case. Repeat from step 3 until the skill meets its goal.

### Claude A creates, Claude B tests

The most effective development loop involves two Claude instances:

- **Claude A** helps design and refine the skill (this conversation
  pattern).
- **Claude B** — a clean session with the skill loaded — tests it on
  real tasks.

Claude A has full context from the creation conversation; Claude B
reveals what the skill actually communicates to a fresh agent. Gaps
between what Claude A *meant* and what Claude B *does* are the
signals to iterate on.

### The self-reformat test

A well-formed skill, when passed through the skill-creator's reformat
flow, produces no **structural or semantic** changes. The test
distinguishes substantive change from cosmetic difference.

**Counts as a change (skill is non-compliant):**

- New headings added, existing headings removed, or section order
  changed.
- Different number of steps (steps added, removed, merged, or split).
- Frontmatter fields changed (added, removed, value semantics changed).
- The essence of the skill changed — different goal, different
  verification criteria, different dependencies, different error
  handling approach.

**Does not count as a change (skill is still compliant):**

- Whitespace differences.
- Rewording that says the same thing — synonyms, sentence
  reorganisation, clarifications that preserve meaning.
- Cosmetic improvements (formatting, capitalisation conventions).

The skill-creator skill itself must pass this test. If running
skill-creator on its own SKILL.md produces structural or semantic
changes, skill-creator is non-compliant with its own rules — fix the
structure first, then ship.

This is a continuous compliance check for any skill: re-run
skill-creator on it; if structural or semantic changes appear, the
skill has drifted from the spec.
