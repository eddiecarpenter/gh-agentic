# concept: agent rationale as artefact (SAPAV)

This document names the pattern that governs every autonomous phase in this
framework: **SAPAV — Stop, Assess, Plan, Act, Verify.** Each step produces a
durable, externally-visible artefact rather than collapsing into pure Act
inside the agent's head.

SAPAV is the structural answer to a recurring failure mode. The agent knows
the rules, but its reasoning is invisible until the irreversible work is done.
By the time the human sees the output — a PR, a set of Task issues, a commit
stream — the decisions that shaped it are buried in the model's ephemeral
context. If the reasoning went off-track, the cost of correction is at
PR-rewind scale, not at "noticed five minutes in" scale.

SAPAV closes that gap by turning each phase of agent reasoning into a
published artefact *before* the phase produces its irreversible output. The
artefact is the intervention point. The human can read it, challenge it, or
let the phase continue.

This doc captures the pattern, the per-phase mapping, the four structural
levers that make it work, and — explicitly — the limits. Concrete
applications of SAPAV across individual phases live in the Features under
requirement #632; this doc is the foundation they reference.

---

## The SAPAV acronym

Every autonomous phase follows the same five-step shape:

### Stop

Before acting, the agent halts and acknowledges it is about to enter
irreversible work. Stop is not a wait — it is a discipline: do not start
producing outputs until the remaining steps have produced their artefacts.

### Assess

The agent reads the current state of the world — the feature issue, the
repo, the relevant concept docs, the existing code — and writes down what
it finds. Assess produces an artefact: a summary of the situation as the
agent understands it. If the agent's understanding is wrong, the assessment
makes that visible.

### Plan

The agent decides what it is going to do and publishes the plan as a
durable artefact (a GitHub issue comment, a task list, a decomposition).
The plan is the structure of the work — units, ordering, rationale, files
to be touched. Plan is published *before* Act.

### Act

The agent executes the plan. Act produces the irreversible work: Task
issues, commits, PR comments. Act is where today's pipeline already
succeeds; SAPAV's contribution is ensuring that by the time Act starts, the
prior four letters have produced their artefacts.

### Verify

After Act completes, the agent — or a later phase — checks the produced
work against the published plan. Verify is the feedback loop that makes
Plan accountable: if the output drifts from the plan, Verify surfaces the
discrepancy. Verify produces its own artefact (a verification comment, a
review finding, a discrepancy report) so the check itself is auditable.

The five artefacts — assessment, plan, output, verification (and the
durable record of the Stop that preceded them) — are the externally-visible
trail of the phase's reasoning.

---

## Per-phase mapping

SAPAV maps cleanly onto the three autonomous phases of the pipeline:

### Feature Design

- **Stop** — the design session opens without creating Task issues.
- **Assess** — the agent reads the Feature issue, the scoping conversation,
  and the relevant code, then summarises the problem.
- **Plan** — the agent posts a structured **Design Summary** comment on the
  Feature: the decomposition, the ordering, the rationale. Published before
  any Task is created.
- **Act** — the Task sub-issues are created from the published plan.
- **Verify** — the downstream Dev Session and PR Review can diff the Tasks
  actually produced against the Design Summary. Drift becomes visible.

### Dev Session

- **Stop** — the dev session does not begin edits when it opens a Task.
- **Assess** — the agent reads the Task body, the relevant files, and the
  Design Summary posted upstream.
- **Plan** — the agent posts a structured **Implementation Plan** comment
  on the Task: the files to be touched, the units of work, the tests. One
  plan per Task, published before any edit.
- **Act** — the agent implements the Task one planned unit at a time,
  committing per unit. The commit stream mirrors the plan's unit list.
- **Verify** — the commit count and per-commit file footprint can be
  matched against the plan. Intra-task checkpoint commits that the plan
  promised but did not produce become detectable.

### PR Review

- **Stop** — the review does not produce findings before reading the
  upstream artefacts.
- **Assess** — the agent reads the Design Summary from the Feature, the
  Implementation Plans from each Task, and the PR diff.
- **Plan** — the agent publishes a **Review Plan** (what dimensions it will
  check: scope, contracts, tests, plan-vs-output, reuse, standards).
- **Act** — the review findings are posted as PR comments.
- **Verify** — the PR author (human or downstream automation) can see
  every finding tied back to a declared review dimension. Findings without
  a declared check become visible as ad-hoc additions rather than hidden
  in the same list.

In all three phases, the "Plan" artefact is what changes the shape of the
work. Today's pipeline runs Stop-Assess-Act implicitly; SAPAV adds
**publish-before-act** and **verify-against-published** as first-class
structural steps.

---

## The four structural levers

Writing "the agent should publish its plan" as a prose rule is not enough.
The framework has tried that shape before — RULEBOOK already tells the
dev-session agent to commit intra-task, and the rule is ignored by every
agent in every session because it competes with execution momentum and has
no enforcement. SAPAV only works when four structural levers back it up.

### 1. Publish-before-act

The rationale artefact is published as a GitHub issue comment *before* the
phase's irreversible work begins. This is the load-bearing constraint: the
plan is durable, externally addressable, and exists before any Task is
created, any file is edited, or any PR comment is posted. A plan that is
published after the fact is not a plan — it is a narrative.

Publish-before-act also creates the human intervention point. If the plan
is wrong, the human can react before the work happens, at cheap-correction
cost.

### 2. Mandatory template

The plan is not free-form prose. The agent fills in a prescribed
structure — named sections, required fields, a fixed artefact shape.
Templates do two things:

- They make omissions visible. An empty "Files to be touched" section is
  obviously an empty section; a prose plan that happens not to mention
  files is not obviously incomplete.
- They make verification tractable. A later phase can parse named sections
  mechanically rather than inferring intent from paragraphs.

Free-form plans degrade into "whatever the model felt like saying" under
load. Templates force the shape.

### 3. Recipe-side enforcement

The recipe — the step-by-step playbook in `skills/` and the pipeline
workflow — drives the SAPAV sequence. The agent does not decide when to
post the plan; the recipe's Step N says "post the plan" before Step N+1
says "start acting." Enforcement is in the pipeline's control flow, not in
the agent's judgement.

This matters because agent judgement under execution momentum is precisely
what SAPAV is designed to compensate for. Asking the agent to pause itself
is asking the failure mode to self-correct. Asking the recipe to pause the
agent is asking the structure to correct the agent.

### 4. Change-pinning verification

Later phases diff the actually-committed work against the declared plan.
**change-pinning** is the name for this mechanism: the plan pins the
expected change set, and the verification step compares the observed
change set against that pin. Drift — files touched that were not in the
plan, tasks produced that were not in the decomposition, review
dimensions checked that were not declared — becomes mechanically visible.

Without change-pinning, a published plan is still just prose. With
change-pinning, the plan is a commitment the work can be measured against,
and the verification artefact turns the measurement into a durable record.

---

## What the pattern does NOT solve

SAPAV is a structural lever, not a magic bullet. The pattern has explicit
limits, and naming them up front keeps the framework from over-claiming.

### Weak plans are still possible

The agent can follow every step of SAPAV and still publish a plan that is
shallow, misframed, or strategically wrong. Structure forces a plan to
**exist** and to have a **fixed shape**; it does not force the plan to be
**good**. Publishing "Files: all of them. Units: implement the feature."
satisfies the template mechanically and tells a reviewer nothing.

SAPAV's contribution is that a weak plan is now *visible*. The human can
read it, see that it is weak, and intervene — cheaply — before Act. That
visibility is a precondition for correction; it is not correction itself.

### Verification can still miss drift

change-pinning compares the committed change set to the declared plan. It
catches gross drift: files the plan did not mention, commit counts that
don't match planned units, tasks produced outside the decomposition. It
does not catch semantic drift — code that touches the right files in the
right count but implements the wrong thing. That class of error is still
the PR reviewer's problem.

### The recipe can under-enforce

Recipe-side enforcement works only if the recipe actually gates Act on the
existence of the plan artefact. A recipe that posts the plan and then
proceeds to Act in the same step has not enforced SAPAV — it has merely
emitted extra text. Gating the pipeline on the presence of a
well-structured artefact (not just the attempt to post one) is a separate
engineering concern that each Feature under #632 must address in its own
implementation.

### Templates can ossify

Mandatory templates are load-bearing against weak prose plans, but they
cost flexibility. A template that fits Feature Design well may fit Dev
Session poorly. The framework's position: accept the cost, keep the
templates per-phase, and revise them through the normal template-update
path when practice shows they under-serve a phase.

### SAPAV does not replace the human

The entire pattern is designed to give the human *visibility*, not to
replace the human in the loop. The plan artefact, the verification
artefact, and the audit trail all assume a reviewer eventually looks at
them. A pipeline that runs SAPAV end-to-end with no human reading the
artefacts has the mechanics of the pattern without its point.

---

## Relationship to other framework principles

SAPAV does not stand alone. It is the structural answer to a discipline
the framework already asserts elsewhere:

- **Lean pipeline** (`concepts/lean-pipeline.md`) — SAPAV's artefacts live
  in GitHub issues and PR comments, the control plane the framework
  already uses. The pattern adds no new external tools; it uses the
  existing minimal surface more deliberately.
- **Contracts are guarded** (`RULEBOOK.md` → Contract Rules) — contract
  decisions are already required to be human-gated. SAPAV generalises the
  underlying shape: surface the agent's intent as an artefact before the
  irreversible action, so the human gate has something concrete to see.
- **Phase transitions are human-gated** (`RULEBOOK.md` → Working
  Principles) — applying pipeline labels is reserved for humans. SAPAV
  gives each autonomous phase a publishable artefact that makes the
  gating decision informed rather than blind.

Future framework rules that follow the same shape — Reuse annotations,
scope assertions, standards self-reports — inherit the same visibility
primitive: **convert agent judgement into externally-visible artefacts
that humans can review and machines can verify.**
