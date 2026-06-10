# Render-Before-Confirm Rule

A reusable safeguard that any skill may load to guarantee the human
can actually *see* the content they are being asked to approve. It
governs every confirm-or-revise gate where the agent renders content
(a plan, a draft artefact, an issue body, a task list) and then asks
the human to Confirm / Revise / Cancel.

## The problem it prevents

When an agent renders content and invokes the approval prompt in the
**same turn**, the prompt UI (the `prompt-user` / `AskUserQuestion`
card) is what the human sees — and text emitted *before* a tool call
in the same turn is not reliably displayed. The result is an approval
gate for content the human never saw: "you are asking me to approve a
plan but did not show it to me." Approval given blind is not approval.

## The rule

When a gate requires the human to approve rendered content, the agent
MUST separate rendering from prompting across two turns:

1. **Render and stop.** Emit the full content as the final output of a
   turn — verbatim, in a fenced block where the content is structured —
   and end the turn with NO tool call after it. The content is the last
   thing in the response.
2. **Prompt on the next turn.** Only after the content has been
   displayed does the agent invoke the `prompt-user` gate
   (Confirm / Revise / Cancel) in a subsequent turn.

A gate that renders and prompts in one turn is malformed, regardless
of how the rendering reads in the skill prose. Wording in a skill step
such as "render the content … then `prompt-user(...)`" describes the
logical order, NOT permission to do both in a single turn — the "then"
is a turn boundary.

The same rule applies to the per-revision re-render: when the human
chooses Revise, the updated content (and any per-revision diff) is
rendered as the final output of its turn, and the re-prompt comes
on the following turn.

## How a skill adopts this rule

Add this file to the skill's `loads:` list and reference it in the
`Definitions` section. The rule then applies to every confirm-or-revise
gate in that skill — the individual gate steps do not need to restate
it.

## Why this works

The failure is structural, not a matter of agent diligence: it stems
from reading "render … then prompt" as a single atomic step. Naming
the turn boundary explicitly removes the ambiguity — the agent treats
the rendered content as a complete response and waits for it to land
before gating on it. Observed in this framework when a Design Plan and
its approval prompt were emitted together and the plan was hidden; the
human could only see the card asking them to approve content they had
never been shown.
