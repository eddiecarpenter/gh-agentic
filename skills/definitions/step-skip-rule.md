# Step-Skip Rule

A reusable safeguard that any skill may load to prevent the agent from
silently skipping steps. The mechanism is articulation: forcing the
agent to write down *why* a step is being skipped causes the agent to
recognise that the step is in fact required, and execute it.

## The rule

Steps in a skill are mandatory unless explicitly marked optional. If
the agent believes a step can be skipped, it MUST first emit — as
written output in its response stream — which step it is skipping and
the concrete reason why.

- The output does not need to be addressed to the user or formatted
  for them. A plain inline note ("Skipping step N because …") is
  sufficient.
- What is forbidden is skipping silently, or skipping on the basis of
  vague reasoning like "seems unnecessary" or "already covered".
- Treat any urge to skip as a signal to slow down and run the step.

## How a skill adopts this rule

Add this file to the skill's `loads:` list and reference it in the
`Definitions` section. No further wording is needed in the skill
itself — the rule applies to every step in any skill that loads it.

## Why this works

Empirically observed in this framework: agents asked to justify a
skip out loud will, more often than not, conclude the step should run
after all and execute it. The cost of articulating a defensible reason
exceeds the cost of just doing the step. This is a behavioural
mechanism, not a logical one — it works because writing forces
inspection that silent reasoning skips.
