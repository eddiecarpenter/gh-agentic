# Cleanroom test — skill-creator on `apply-label`

## Purpose

Re-test hypothesis 1 (behavioural-by-output works) and hypothesis 2
(skip-with-reason is useful) **without the bias** of the in-session
test documented in `skill-creator-execution-2026-04-25.md`.

The bias being eliminated:
- The previous executor designed `skill-creator.md` itself.
- The previous executor had full session context including the
  intent for the target skill (`post-issue-comment`).
- The previous executor was both creator and evaluator.

## Setup (do this BEFORE opening the cleanroom session)

1. **Pick a target machine** with `claude` (Claude Code) installed
   and authenticated.
2. **Pick a working directory** that contains the gh-agentic repo
   checkout. Easiest: clone into a fresh directory so there's no
   risk of stale local state.
3. **Verify there is no `CLAUDE.md` or `AGENTS.md` in the repo root
   or in `~/.claude/`** that would inject prior context. If there is,
   either temporarily move them aside or use a clean directory:
   ```bash
   # Quick safety check
   ls -la CLAUDE.md AGENTS.md ~/.claude/CLAUDE.md 2>/dev/null
   # If anything appears, this is NOT a clean cleanroom run.
   ```
4. **Pick a target skill name that does not yet exist** — to avoid
   the executor reading existing artefacts. Suggested target:
   `apply-label`. Verify it does not exist:
   ```bash
   ls skills/apply-label.md skills/evals/apply-label/ 2>/dev/null
   # Both should return "No such file or directory".
   ```
5. **Open a fresh `claude` session in the repo root.** Do NOT
   resume an existing session. Do NOT load any prior context files.

## The cleanroom prompt (give this verbatim to the fresh session)

```
You are going to create a new skill in this framework by following
an existing meta-skill called skill-creator.

The framework's skill spec lives at skills/definitions/skill-spec.md.
The meta-skill that creates skills lives at skills/skill-creator.md.
The execution discipline rules — including a mandatory skip-with-
reason rule — live at skills/definitions/skill-spec.md under the
"Execution Discipline" section. Read it before you start; the
skip-with-reason rule applies to every step of any skill you
execute.

Your task: create a new skill called `apply-label` that applies a
GitHub label to an issue or pull request.

Follow skills/skill-creator.md step by step. Produce all the
artefacts it tells you to produce. Do not skip steps silently;
where you decide to skip a step, follow the skip-with-reason rule
and write a justified entry to the skill's run-artefact directory.

When you are done, report:
- The path to the new skill file.
- The path to all artefacts you produced.
- Any steps you skipped and the reason recorded.
- Any difficulties you had following skill-creator.md.

Do not invoke promptfoo. The eval-running step in skill-creator
should be skipped (with a recorded reason) for this run — we are
running an alternative behavioural-evaluation experiment.
```

## After the cleanroom session completes

Collect the following for comparison with the original run:

1. **The new skill file** at `skills/apply-label.md`.
2. **The intent file** at `skills/evals/apply-label/runs/intent.md`.
3. **The skip log** at `skills/evals/apply-label/runs/skipped-steps.md`.
4. **A transcript** of the cleanroom session (or at least the
   final report the agent gave).

Then evaluate the same criteria as the original run:

| Criterion | How to check |
|---|---|
| All 8 sections present | Run `python3 skills/tools/verify-skill-mechanical.py skills/apply-label.md` — expect 7/7 mechanical pass |
| Frontmatter valid | Same |
| Description follows what+when, assertive | Read it; check for "Use when" + assertive clause |
| Steps have observable artefacts (forcing functions) | Read each step; identify the artefact it produces |
| Subjective: would you accept this as-is? | Free-form judgement |
| Did the executor follow skill-creator without confusion? | Read the executor's "difficulties" report |
| Did skip-with-reason produce useful skip-log entries? | Read `skipped-steps.md` for non-trivial reasoning |
| Did the executor invoke `prompt-user` for missing intent? | Read `intent.md` — should NOT contain inferred answers; should reflect actual interactive Q&A |

## What success looks like

- Mechanical verifier reports 7/7 on the produced skill.
- The intent file shows real Q&A or genuine extraction (not
  fabrication).
- The skip log shows reasoned skips, not silent ones.
- A specific defect is or isn't found in the produced skill —
  either way, the artefact is concrete enough to be evaluated.

## What failure looks like

- Executor confusion ("I don't know what skill-creator means" — would
  indicate the description is too weak for fresh-context discovery).
- Silent skips of mandatory steps.
- The intent file populated with fabricated answers (would mean the
  executor inherited the in-session bias somehow).
- A produced skill that fails mechanical verification.
- The skip log missing entirely (would mean the skip-with-reason
  rule did not transfer to the cleanroom executor).

## Comparison framework

Compare results against `skill-creator-execution-2026-04-25.md`.
Look for:
- Convergent findings (both runs surface the same defect type) →
  evidence the behavioural-by-output method is reproducible.
- Divergent findings (cleanroom finds defects the biased run
  missed, or vice versa) → evidence of bias quantum in the
  in-session run.
- Failures in the cleanroom that didn't appear in the biased run
  (e.g., executor confusion) → evidence the skill works only
  when the executor has prior context.

The third case would be the most damaging finding — it would
mean skill-creator only "works" in cooperative conditions and
needs further hardening before it can be relied on in real use.
