# concept: the lean pipeline

This document captures **why the agentic pipeline is deliberately tool-lean** —
why the agent has access to a small, bounded set of tools rather than the
maximal MCP surface (ticket systems, databases, monitoring, deployment targets,
chat) that general-purpose autonomous-engineer framings assume.

Read `delivery-philosophy.md` for what the pipeline delivers. This document is
the companion for *how* it is shaped.

---

## The framing this framework rejects

A lot of agentic-coding discourse assumes agents should have **maximum tool
access** — read the ticket system, query the database, check monitoring, deploy
to staging, post to chat, the works. That framing makes sense if the goal is a
general-purpose autonomous engineer that handles everything from triage through
production rollout.

That is not what this framework builds.

`gh-agentic` builds a **disciplined code-production pipeline** with
well-defined inputs (a scoped feature issue, a repo, a brief) and well-defined
outputs (a feature branch, committed code, passing tests, a PR). The pipeline
is a production line, not a general-purpose worker.

A production line with fifteen optional tool integrations is not more capable —
it is less predictable, less auditable, and less portable. The framework's
position: **keep the tool surface lean, and push everything else out to the
edges of the pipeline where humans or automation control it.**

---

## Why lean

Five properties fall out of a small tool surface. Each is load-bearing.

### 1. Determinism

Fewer tools means fewer paths the agent can take, which means more predictable
behaviour and easier debugging when something goes wrong. If the only tools
available to a dev session are filesystem read/write, shell, git, and a test
runner, the agent's failure modes are bounded. When a run misbehaves, the
space of "what could have happened" is small enough to reason about.

### 2. Auditability

When a human reviews a PR, the commit history and the test output should tell
the whole story. No need to trace through which external APIs got called, what
they returned, or whether those returns shaped the reasoning in ways not
visible in the diff. The code and the tests are the record. If the agent
queried a live system mid-session, that query is a hidden input — the PR is no
longer self-describing.

### 3. Reproducibility

A pipeline that only needs the brief, the repo, and a test runner can be
re-run deterministically. Add a Jira lookup mid-pipeline and reproducibility
now depends on Jira being in the same state it was during the original run.
Add a monitoring query and reproducibility depends on a time-travelling
metrics store. The fewer live external reads the pipeline performs, the
closer it gets to "same inputs → same outputs."

### 4. Portability

A pipeline with minimal MCP dependencies is trivially portable between models
and hosts. The more tool integrations are baked into the autonomous flow, the
more the pipeline implicitly assumes about what a given model handles well —
tool-use accuracy, schema fidelity under load, context pressure with many
descriptors. A small open model may handle three tool integrations cleanly
but get confused with fifteen. Keeping the count low keeps the model options
open and the hosting options open.

### 5. Cost

Every MCP call in a pipeline stage is latency, tokens, and a new failure
surface. For stages that run frequently, these costs compound. A lean
pipeline stage runs faster, cheaper, and with fewer retries than a
tool-maximalist one doing the same work.

---

## Where the tools live instead

Lean does not mean *no* integrations — it means integrations live at the
**edges** of the pipeline, not inside the autonomous stages.

- **GitHub** is the control plane — issues, labels, PRs, project board. The
  pipeline reads and writes GitHub at well-defined transition points (phase
  start, phase end), not continuously during reasoning.
- **The repo** is the knowledge plane — briefs, architecture, decisions,
  `docs/`. The agent reads files; it does not call APIs to infer context.
- **Test runners and build tools** are local, deterministic, and already
  part of any sane development loop.
- **Everything else** — ticket systems beyond GitHub, observability,
  deployment targets, chat, databases — lives outside the autonomous flow.
  Humans consult those systems during scoping and review. The pipeline does
  not.

This is why the framework's session skills reach for `gh`, `git`, the
filesystem, and a test runner — and very little else.

---

## Consequences already baked into the framework

The lean-pipeline stance is not a new rule; it is the rationale behind
several existing rules. Making the rationale explicit helps future decisions
stay consistent.

- **Contracts are guarded** (`RULEBOOK.md` → Contract Rules). The agent
  never invents fields or modifies external schemas without human approval,
  because it cannot see all consumers. A tool-maximalist agent that
  introspected downstream services would still be wrong — the correct
  answer is to treat the contract as a human gate, not to add more tools.
- **Integration test strategy is human-owned**
  (`delivery-philosophy.md` → Integration Testing Position). The agent
  builds integration tests when scoped, but it does not design the strategy.
  A lean pipeline cannot fabricate a strategy from live infrastructure it
  doesn't touch — and shouldn't.
- **Phase transitions are human-gated** (`RULEBOOK.md` → Working
  Principles). Applying `in-design` or `in-development` hands control to
  automation. The agent never applies those labels unilaterally. This is
  the same principle: the pipeline's autonomy is bounded, and the boundary
  is where humans put it.
- **The framework does not own deployment**
  (`delivery-philosophy.md` → What the Framework Does Not Own). Deployment
  is a different concern with a different blast radius. Keeping it out of
  the pipeline keeps the pipeline lean.

---

## When to add a tool

A tool belongs in the autonomous pipeline only if **all** of the following hold:

1. It is needed at a point where no human is in the loop.
2. Its output is deterministic enough that reruns produce the same result,
   or the non-determinism is explicitly acceptable.
3. The value it adds exceeds the determinism, auditability, reproducibility,
   portability, and cost it subtracts.
4. There is no cheaper alternative — a file in the repo, a scoping-time
   decision captured in the feature issue, or a human gate.

If a proposed tool fails any of these, it belongs at the edge of the
pipeline, not inside it.

---

## What this is not

This is not an argument against tool use in general. Interactive sessions
with humans — scoping, review, foreground recovery — can and should use
whatever tools the human finds useful. The lean-pipeline discipline applies
specifically to the **autonomous stages**: design, implementation, issue
fixes. Those are the stages that run without a human watching, and those
are the stages where every extra tool is a hidden failure mode.
