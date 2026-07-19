# concept: context layers and human roles

This document defines **who provides what to the pipeline before the first line
of code is designed** — the human roles in the first half of the pipeline, the
layers of context they own, and the single rule that keeps a goal-seeking agent
from dragging a human past their competence.

Read `lean-pipeline.md` for why the autonomous stages are tool-lean, and
`knowledge-plane.md` for where knowledge lives. This document is the companion
for *the first half* — everything above the first judgment gate, where humans
author and the agent facilitates.

---

## The two human contributions have opposite shapes

With agentic workflows in the loop, the human stops being an author of every
artefact and becomes the source of two things the agent cannot supply itself:

- **Judgment is episodic and gated.** It happens at discrete, blocking points:
  *approve the design/plan*, and *approve the code at merge*. Point-in-time,
  yes/no, "this is right / redo it."
- **Context is continuous and foundational.** It is the substrate every
  autonomous step draws from — plan, design, implementation all read it. Not a
  gate; an ambient input that must exist *before* the autonomous stages can be
  trusted.

This distinction is load-bearing. Judgment wants clean gates. Context wants a
persistent, loadable home. The pipeline already has the gates; this concept
closes the gap on the context.

---

## Role collapse — to two, not one

Agentic workflows collapse the traditional delivery roles, but not all the way
to a single person. Everything **technical** — solution architect, architect,
developer — collapses into **the software engineer + the agent**, because those
are the same kind of competence: technical judgment about how the system works
and how to build it.

**Product owner does not collapse into the engineer.** Business competence —
what the market needs, what is worth prioritising, whether an outcome fits the
customer — is a genuinely different faculty. It does not fuse into a technical
head just because agentic workflows arrived. The one seam that survives the
collapse is **business competence vs technical competence.**

| Role | Competence | Degree in the pipeline | Faces the agent? |
|---|---|---|---|
| **Product owner** | Business — need, priority, outcome-fit | Low / bounded | **No** |
| **Software engineer** | Technical — context, design, build judgment | Primary (the operator) | **Yes** |

The engineer is the pipeline's **front door**. The product owner is a *source
that feeds the engineer*, deliberately kept one step back from the agent — for
the reason in the next section.

---

## Why the product owner stays one step back

It is tempting to let the product owner define their requirement directly in the
pipeline. This fails — not because the product owner is weak, but because it
points a **goal-seeking agent at a human who cannot match its drive.**

Endgoal-drive is the agent's asset *below* the first judgment gate — you want
execution to run to done. *Above* the gate it is a **hazard**: "running with it"
means the agent keeps escalating toward technical context and design decisions
the product owner cannot supply *or judge*. Three failure modes, in increasing
danger:

1. The product owner blocks, overwhelmed.
2. The agent fills the gap itself — context invented, not provided.
3. Worst: the product owner **rubber-stamps agent-invented context they cannot
   evaluate** — so unjudged context enters the pipeline wearing a human's
   approval. That is more dangerous than no approval at all.

The product owner's contribution is real, but it belongs **upstream of the
facilitator** — delivered human-to-human (or via a bounded, non-agentic
capture) to the engineer. The engineer receives the business need, brings the
technical context layers, and is competent to keep pace with the agent's drive.

> Point the goal-seeking agent only at the human who can match it.

---

## The context layers

Context is layered, and each layer answers a different question. In the
traditional world each layer had a different role owner. The roles collapse; the
**layers do not** — they still have to exist as artefacts, because the layers
are exactly what the agent reads to avoid inventing.

| # | Layer | Old role | Answers | Home |
|---|---|---|---|---|
| 1 | Business / org context | Business analyst | Why the org cares; domain; constraints | `BRIEF.md` *(exists)* |
| 2 | **Technical / solution context** | **Solution architect** | Tech landscape; existing systems; integration reality; NFRs; the sanctioned approach | **`SOLUTION_CONTEXT.md`** *(new — the missing layer)* |
| 3 | Architectural design | Architect | How it maps into system structure | `ARCHITECTURE.md` *(exists)* |
| 4 | The requirement | Business | The specific ask (what/why, no how) | Requirement issue *(exists)* |
| 5 | Implementation plan | Developer | How to build *this* requirement | scoping + feature-design *(agent)* |
| 6 | Implementation | Developer | The code | dev-session *(agent)* |

Layers 1, 3, 4, 5, 6 already have homes. **Layer 2 — the technical/solution
context the solution architect used to own — had none.** Its owning role
collapsed into the engineer, but its output artefact was never re-homed. That
missing middle is why implementation "lacked the bigger context and filled the
blanks with its own ideas": the pipeline carried the requirement (4) and the
structural design (3) but not the connective tissue between them.

`SOLUTION_CONTEXT.md` closes that gap.

---

## `SOLUTION_CONTEXT.md` — the contract

**Holds:** the durable, cross-requirement technical context an engineer would
otherwise carry in their head — the technology landscape, the systems this
work integrates with, the non-functional requirements, the sanctioned way to
approach problems in this codebase, and the constraints that shape *how* things
get built. It is the forward-looking design intent that precedes requirements,
distinct from `ARCHITECTURE.md`, which is the backward-looking, current-state
description assembled *after* features merge (see `knowledge-plane.md`).

**Author:** the software engineer, with the agent facilitating — interview,
challenge, structure, draft. Never authored by an autonomous stage.

**Cadence:** rare and strategic, like `BRIEF.md` — a direct, human-driven PR.
It changes when the technical landscape shifts, not per feature.

**Distinguished from scoping:** durable, reusable technical context lives in
`SOLUTION_CONTEXT.md`; requirement-specific "how" lives in the per-feature
scoping artefact. Same kind of content, sorted by lifetime.

**Load-or-stop rule (the guarded cliff):** `SOLUTION_CONTEXT.md` is loaded into
every session and is a **required input** to the autonomous stages. The
autonomous stages may **consume it but never author it.** A stage that cannot
find the technical context it needs must **stop and ask — never invent.**
Inventing context below the first gate is precisely the failure this concept
exists to prevent.

> Topology note: in single topology `SOLUTION_CONTEXT.md` lives at the repo
> `docs/` root alongside `BRIEF.md` and `ARCHITECTURE.md`. The federation
> homing (system tier vs domain tier, per `knowledge-plane.md`) is a scoping
> decision for the build, not fixed here.

---

## The disposition inversion at the first gate

Matching the right human to the pipeline is necessary but not sufficient. The
agent's momentum must **change character** at the first judgment gate:

- **Below gate 1 — drive to *done*.** Execution; momentum is the feature.
- **Above gate 1 — drive to *"everything this human can competently give,"
  then stop.*** Not drive-to-done. The agent elicits and structures, and when it
  reaches the edge of the operator's competence it **flags the gap or hands to a
  more competent human — it never fills it itself.**

A goal-seeking agent left un-inverted above the context layer will *always* drag
a human past their limit. So the first-half design is not only "which human" but
"which human **and** an agent whose disposition is capped to that human's
competence." This single inversion protects the human and closes the
invented-context bug at the same time.

---

## The two judgment gates

- **Gate 1 — design / plan approval.** The engineer judges the plan, either by
  being in the loop through interactive design, or by reviewing the published
  design rationale after a headless design run (the SAPAV pattern — see
  `agent-rationale-as-artefact.md`).
- **Gate 2 — code review at merge.** The engineer judges the code.

The stretch of autonomous execution is, by definition, the span *between* two
gates. That is why headless automation starts at gate 1 and no higher: you
cannot push automation above the first gate without deleting the human's core
contribution or merely relocating the gate.

---

## Cross-references

- `lean-pipeline.md` — why the autonomous stages are tool-lean; the
  facilitator-vs-executor boundary is the same boundary as autonomy.
- `knowledge-plane.md` — where `BRIEF.md`, `ARCHITECTURE.md`, and (by extension)
  `SOLUTION_CONTEXT.md` live and how they load into every session.
- `agent-rationale-as-artefact.md` — the SAPAV pattern behind gate 1.
- `delivery-philosophy.md` — the pipeline's scope and the deploy/release/enable
  distinction.
