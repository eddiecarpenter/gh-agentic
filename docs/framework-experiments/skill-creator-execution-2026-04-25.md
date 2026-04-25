# Experiment: skill-creator end-to-end execution (2026-04-25)

## Hypothesis

Two hypotheses tested in a single run:

1. **Behavioural-by-output:** the quality of a reasoning-heavy skill
   can be judged empirically by running it on a real task and
   evaluating the artefacts it produces — not by judging its prose
   under a rubric.
2. **Skip-with-reason:** forcing the agent to articulate a reason
   for skipping a step creates cognitive friction that exposes silent
   skips as either (a) genuine and documented, or (b) wrong, prompting
   the agent to perform the step instead.

## Setup

- **Skill under test:** `skills/skill-creator.md` (round 8 state — 9
  steps, with verification gates, after the simplification pass that
  dropped the redundant self-reformat step).
- **Target skill to be created:** `skills/post-issue-comment.md` —
  a primitive that posts a comment to a GitHub issue or PR.
- **Executor:** Claude (this session) — biased by full session
  context including the design of skill-creator itself. Bias
  documented in line.
- **Promptfoo:** paused for this run. Behavioural execution + output
  judgement is the substitute evaluation method.
- **Skip-with-reason rule:** added to `skills/definitions/skill-spec.md`
  under a new "Execution Discipline" section before this run started.

## Per-step execution log

### Step 1 — Capture intent (PERFORMED)

- Created `skills/evals/post-issue-comment/runs/intent.md` with all
  four sections populated.
- **Bias note recorded inline in the file:** the four answers were
  inferred from session context rather than via `prompt-user`. A
  fresh-session run should treat this as a shortcut and invoke
  `prompt-user` per missing answer.

### Step 2 — Decide whether to proceed (PERFORMED)

- Wrote placeholder `## Consumers` and `## Decision: ___` first.
- Filled `## Consumers` with 5 real Session-skill consumers:
  feature-design, dev-session, pr-review-session, issue-session,
  requirements-session.
- Replaced `___` with `proceed (≥2 consumers)`.
- Verification gate (`grep -E "^## Decision: (inline|proceed)"`)
  returned exactly one match. Pass.

### Step 3 — Draft the skill (PERFORMED)

- Created `skills/post-issue-comment.md` (5,510 bytes) with all 8
  spec-mandated sections in order.
- Used the new "Audit checks (informative)" subsection in
  Verification — first skill to use the split we agreed to during
  this session.

### Step 4 — Identify needed definitions (SKIPPED with reason)

- Reason recorded at `skills/evals/post-issue-comment/runs/skipped-steps.md`.
- Genuine empty work (only `loads:` entry was the existing
  `error-handling.md`).
- **Articulation-test outcome:** writing the reason DID surface a
  small refinement — the realisation that an empty-work skip should
  produce a transient artefact (the skip log entry itself), to
  distinguish "applied-and-found-nothing" from "didn't bother". This
  is one genuine win for skip-with-reason.

### Steps 5–8 — Eval folder, skill-eval, review, iterate (SKIPPED with reason)

- Reasons recorded at `skipped-steps.md`.
- Skipped by experimental design (Promptfoo paused). Not skipped on
  judgement grounds.
- **Articulation-test outcome:** no real friction; the skips were
  directed and the articulation just confirmed the design.

### Step 9 — Hand off (PERFORMED)

- Surfaced via this section — paths, mechanical result, consumer
  wiring deferred (the 5 named consumers need back-references; not
  performed by skill-creator per its design).

## Artefacts produced

```
skills/post-issue-comment.md                                     (5.5 KB)
skills/evals/post-issue-comment/runs/intent.md
skills/evals/post-issue-comment/runs/skipped-steps.md
```

## Mechanical verification of the produced skill

```
$ python3 skills/tools/verify-skill-mechanical.py skills/post-issue-comment.md
Summary: 7 passed, 0 failed (out of 7)
```

## Subjective evaluation of `skills/post-issue-comment.md`

| Criterion | Verdict |
|---|---|
| All 8 sections present | ✅ |
| Frontmatter valid | ✅ |
| Description follows what+when template, assertive | ✅ |
| Description length appropriate (primitive 150-300 target) | ⚠️ 543 chars — wider than target; defensible but borderline |
| Output Artefacts declares both side-effect and return value | ✅ |
| Steps are imperative, one action per step, with detection blocks | ✅ |
| Steps have observable artefacts (forcing functions) | ✅ — every step produces an observable output |
| Verification uses Mechanical / Gateable / Audit split | ✅ — first skill to do so |
| Error Handling explicit and per-error | ✅ — three named error types |
| Would I accept this as-is? | **Yes**, with one caveat |

**The caveat:** step 4 of `post-issue-comment.md` extracts the
comment ID by stripping after the final dash in the URL. This is
fragile — if GitHub changes URL format the parser breaks. A real
ship would use `gh api` to get structured JSON. **This is exactly
the type of finding behavioural eval surfaces and rubric eval
does not** — the prose is fine, but the *approach* is fragile,
and you can only see that by reading the steps as code.

## Hypothesis 1 result: Behavioural-by-output

**Confirmed for this case.** Judging the produced artefact against
concrete criteria yielded:
- A clear pass on all structural and writing-style criteria.
- A specific, actionable defect finding (URL parsing fragility) that
  would not have been surfaced by a prose-rubric check.
- A subjective accept/reject verdict that was easy to articulate
  because it was anchored to a concrete file rather than to abstract
  prose.

The variance issue we hit with the rubric (different findings each
round) does not apply here — the artefact is fixed, so reviewers
either find a defect or they don't, deterministically.

## Hypothesis 2 result: Skip-with-reason

**Partially confirmed.** Mechanism worked once for real (step 4's
articulation surfaced the empty-work-still-needs-an-artefact insight)
and worked five times trivially (steps 5–8, where skips were
directed by experiment design and articulation just confirmed).

The mechanism's surface area on this skill was small. A fairer test
would be a complex Session skill where the agent has multiple genuine
"should I skip this?" decisions. Recommend re-testing on a
judgement-heavy target.

**Even on small surface area, the skipped-steps.md log itself is
high-value:** it gives the human reviewer a concise audit trail of
what the agent chose not to do, with the agent's reasoning. That
audit trail is independently useful regardless of whether the
articulation-test caught any wrong skips.

## Bias and limitations

1. **Executor designed the skill under test.** I (Claude in this
   session) wrote skill-creator. Executing it on myself is biased
   toward favourable interpretation. Verdict on hypothesis 1 should
   be re-tested against a fresh executor.
2. **Full session context.** Intent for `post-issue-comment` was
   fully inferable from our discussion, so step 1's prompt-user
   invocations were shortcut. A fresh session would not have this
   context. Documented in line in `intent.md`.
3. **Single skill, single run.** n=1. The defect found
   (URL-parsing fragility) is anecdote, not statistic. The behavioural
   approach needs more runs across more skill types before we can
   characterise its accuracy.
4. **Output-judgement was performed by the same executor.** The
   evaluator and the creator are the same agent in the same session.
   A clean evaluation would be a fresh agent given the produced
   artefact and the criteria.

## Recommended follow-up: cleanroom re-test

A genuine test of hypothesis 1 needs a fresh Claude session with
NO context from this work — no `AGENTS.md`, no `CLAUDE.md`, no
session memory, just the skills/ folder and an instruction to
follow `skill-creator` by name.

The instruction set is in `cleanroom-prompt.md` alongside this file.

## Recommendation

- **Adopt behavioural-by-output as the primary quality test for
  reasoning-heavy skills.** Drop the prose-rubric layer (the
  variance floor we measured makes it a poor gate).
- **Keep skip-with-reason as a framework-wide rule.** Even with
  small surface area in this run, the audit trail it produces is
  high-value.
- **Run the cleanroom re-test before committing infrastructure
  to behavioural-by-output.** This run shows the approach works
  in principle; cleanroom shows whether it works without bias.
- **The fragility-in-step-4 finding** is a real defect to fix in
  `skills/post-issue-comment.md` — switch to `gh api` for
  structured JSON before declaring the primitive ship-ready.
