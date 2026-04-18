---
name: ask-user
description: Canonical harness-neutral interaction shape for every confirmation, classification, disambiguation, or choice prompt raised by any skill — defines when to use a selectable prompt, option constraints, fallback phrasing, and the four canonical prompt shapes (confirm/revise, multi-choice selection, yes/no/later, name-collision). Use inline whenever a skill needs to ask the human for a decision, never as a standalone session.
category: Reference
triggers: on-demand
loads: []
emits-exit-block: false
exit-hands-to: null
---

# Ask User — Reference Skill

## Purpose

This file is the **single source of truth** for how any skill asks the human a
question. Every confirmation, classification, disambiguation, or choice prompt
in the framework invokes this reference rather than reinventing the phrasing,
the option shape, or the fallback behaviour inline.

Why a single source of truth:

- **No drift.** Consumers do not re-specify interaction rules locally. A change
  to the interaction shape happens here once and propagates everywhere.
- **Harness-neutral.** The rules describe the *interaction shape*, not any
  specific tool or runtime. The same skill must work in every harness a human
  might drive a session from, without edits.
- **Reviewable.** Reviewers checking a consumer skill do not have to re-read
  interaction rules — they only have to verify the consumer invokes this file
  at every decision point.

Follows the precedent of `skills/set-issue-status.md` — a Reference skill
invoked inline by consumers, never run as a standalone session.

## When to Invoke

Invoke this skill **inline** from any other skill whenever the agent must:

- **Confirm** a proposed artefact, classification, path, or decision before
  committing to it (e.g. "Is this feature summary correct?").
- **Disambiguate** between two or more plausible options when the agent cannot
  decide alone (e.g. candidate categories, similar existing artefacts).
- **Choose** one of a small, directed set of outcomes (e.g. deployment
  strategy, yes/no/later, rename/overwrite/cancel).
- **Collect** a short free-text response when no closed set of options is
  meaningful (e.g. a reason, a name, a custom description).

Do **not** invoke this skill for:

- Long-form information gathering that needs a multi-turn dialogue in its own
  right — that belongs in the consumer skill's procedure.
- Silent automatic decisions the agent is confident enough to take without
  asking — do not invent ceremony around a non-decision.

## When a Selectable Prompt Applies

Present the question as a selectable prompt (structured option list the human
picks from) when **all** of these hold:

- There is a **closed, directed** set of outcomes with 2–4 options.
- Every option can be labelled in **under ~40 characters** without losing the
  meaningful distinction between them.
- The human's reasonable reply is *one of the options*, not a sentence.
- The consequence of each option is **different enough** that the human is
  making a real choice — not rubber-stamping a foregone conclusion.

Use a typed free-text reply when **any** of these hold:

- The answer is essentially narrative (a reason, a name, a description).
- The option set is open-ended or cannot be enumerated.
- More than ~4 meaningful options exist — at that point list-plus-free-text is
  a closed menu, not a selection.

## Option Constraints

When using a selectable prompt:

- **Count:** present between 2 and 4 options. Fewer than 2 is a non-choice;
  more than 4 is information-gathering disguised as selection — switch to
  typed reply.
- **Labels:** each option's label is **≤ ~40 characters**, on a single line,
  imperative or descriptive but never a full sentence. Use the first line of
  the label as the anchor; any extra detail belongs below the label, not
  inside it.
- **Directed:** every option points at a distinct action or outcome. Do not
  list synonyms of the same outcome.
- **Order:** safest option first, then more-aggressive options, with the
  cancel/escape path last.
- **Escape:** include an **"Other — describe"** free-text option whenever the
  agent is classifying or disambiguating based on the human's intent. The
  escape exists so the human is never trapped by the agent's enumeration. A
  fixed two-way decision (e.g. yes/no) does not need an Other.

## Fallback Phrasing — Harnesses Without Structured Selection

When the current harness does not expose a structured selection mechanism,
fall back to a **typed free-text reply** prompt with the same semantic content
as the selectable version would have had. The fallback is not a degradation
warning — it is a natural alternative that must read well on its own.

Fallback rules:

- Present the question in one line.
- List the options inline as `1)`, `2)`, `3)`, … with the same labels.
- Invite the human to **reply with the number, the label, or free text**.
- Never mention that the structured mechanism is unavailable. The prompt must
  stand on its own.
- Interpret the reply liberally: a typed number maps to the option at that
  position; a typed label (or clear prefix of one) maps to that option; any
  other reply is treated as free text — which for classification/
  disambiguation prompts counts as exercising the "Other" escape.

A consumer skill should never contain two different prompt versions for
"selectable" and "typed" — it invokes this skill once and describes the
question. The harness behaviour is this skill's concern.

## Canonical Prompt Shapes

Every prompt issued by a consumer skill fits one of the four shapes below.
When a consumer invokes this file, it describes *which shape* it needs and
supplies the content — not the phrasing.

### Shape 1 — Confirm / revise

Used when the agent has produced a proposed artefact (summary, criterion,
name, draft block) and needs the human to accept it as-is or ask for changes.

Canonical form:

```
<short one-line title — what is being confirmed>

<the proposed artefact, verbatim, on its own lines>

Selectable:
  ○ Looks good — proceed
  ○ Needs revision — describe what to change
  ○ Cancel — discard this artefact

Typed fallback:
  Reply with: 1 to accept, 2 with a description of the change, or 3 to cancel.
```

Rules for Shape 1:

- Always include a "needs revision" option — never force accept/cancel only.
- A "needs revision" reply is always followed by a free-text elaboration. The
  consumer skill is expected to read the reply and regenerate the artefact.
- "Cancel" is the escape; do not default to it.

### Shape 2 — Multi-choice selection

Used when the human must pick one of several directed outcomes — classification,
deployment mode, placement path, etc.

Canonical form:

```
<short one-line question>

Selectable:
  ○ <Option A label, ≤ ~40 chars>
     <one-line clarification if needed>
  ○ <Option B label>
     <one-line clarification>
  ○ <Option C label>
     <one-line clarification>
  ○ Other — describe <what>

Typed fallback:
  Reply with: 1) <Option A label>, 2) <Option B label>, 3) <Option C label>,
  or a free-text description.
```

Rules for Shape 2:

- 2–4 curated options plus an "Other" escape when the consumer is interpreting
  the human's intent.
- Option labels read as answers to the question, not restatements of it.
- A per-option clarification is optional; when present it stays under one
  line and never duplicates the label.

### Shape 3 — Yes / not-now / later

Used for any proactive suggestion the agent surfaces to the human — capturing
a recurring action as a skill, escalating a tentative decision, proposing an
opportunistic cleanup. A deferrable offer, never a demand.

Canonical form:

```
<short one-line description of the observed situation or opportunity>

Selectable:
  ○ Yes — proceed now
  ○ Not now — do not ask again this session
  ○ Later — ask again if the situation persists

Typed fallback:
  Reply: yes / not now / later (or describe).
```

Rules for Shape 3:

- Exactly three options. Yes, Not now, Later. No extra variants.
- "Not now" is session-scoped suppression — the consumer must record that the
  suggestion for *this particular pattern* is dismissed for the rest of the
  session.
- "Later" permits re-asking at a *later* natural pause if the situation
  continues. The consumer decides when "later" has arrived; this skill does
  not prescribe a timer.
- Never treat silence as "Later" — silence ends the dialogue, nothing more.

### Shape 4 — Name collision

Used when the agent is about to write a file or create a named artefact whose
name already exists. The agent must not overwrite silently; it asks.

Canonical form:

```
A <kind> named "<name>" already exists at <path>.
Overwriting would discard its current content.

Selectable:
  ○ Choose a different name
  ○ Overwrite the existing <kind>
  ○ Cancel — do not create this <kind>

Typed fallback:
  Reply with: 1) a new name, 2) "overwrite", or 3) "cancel".
```

Rules for Shape 4:

- Three options: rename (safest, listed first), overwrite, cancel.
- Overwrite is explicit and never the default. If the consumer needs
  overwrite-by-default behaviour, it is the wrong shape — write a design
  document first.
- The prompt must name the existing path so the human has an unambiguous
  reference point.
- A rename reply is the new name itself; the consumer then re-runs its
  collision check against the new name.

## Rules

- **Invoke inline, never standalone.** This skill describes the interaction
  shape — it does not run a session or emit an exit block.
- **No specific tool names** in any skill body that invokes this skill. The
  interaction shape is the contract; the mechanism is the harness's concern.
- **No specific harness names** in any skill body that invokes this skill.
  Describe what the agent asks, not which runtime is asking.
- **Shape first, then content.** A consumer skill states which of the four
  shapes applies, then supplies the question, options, and any proposed
  artefact. The phrasing scaffolding comes from this file.
- **Constraints are normative.** 2–4 options, ≤ ~40-character labels,
  safest-first ordering, "Other" escape for intent-based classifications —
  every consumer must conform.
- **Fallback parity.** Every selectable prompt has a typed fallback that
  reads naturally on its own. Consumers never specify two prompts; this skill
  handles the difference.
- **Silence is not an answer.** The consumer must treat silence as "no reply
  yet" and keep the dialogue open. Do not invent a default that commits the
  human to an outcome they did not choose.
